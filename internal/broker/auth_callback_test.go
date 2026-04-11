package broker

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jr200-labs/nats-iam-broker/internal/tracing"
	"github.com/nats-io/jwt/v2"
	natsserver "github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nkeys"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

func ensurePropagator(t *testing.T) {
	t.Helper()
	otel.SetTextMapPropagator(propagation.TraceContext{})
}

// testFixture builds a minimal but complete set of keys, config, and claims
// needed to exercise the auth callback logic end-to-end.
type testFixture struct {
	accountKP  nkeys.KeyPair
	signingKP  nkeys.KeyPair
	userKP     nkeys.KeyPair
	accountPub string
	userPub    string
	config     *Config
	configMgr  *ConfigManager
	ctx        *Context
	tmpFile    string
}

func newTestFixture(t *testing.T) *testFixture {
	t.Helper()

	accountKP, err := nkeys.CreateAccount()
	require.NoError(t, err)
	accountPub, err := accountKP.PublicKey()
	require.NoError(t, err)

	signingKP, err := nkeys.CreateAccount()
	require.NoError(t, err)

	userKP, err := nkeys.CreateUser()
	require.NoError(t, err)
	userPub, err := userKP.PublicKey()
	require.NoError(t, err)

	configYAML := fmt.Sprintf(`
nats:
  url: "nats://localhost:4222"
  jwt_expiry_bounds:
    min: 1m
    max: 1h
service:
  name: "test-auth-svc"
  description: "Test Auth Service"
  version: "1.0.0"
  creds_file: "/dev/null"
  account:
    name: "test"
    signing_nkey: "SUAGJBPRRXFQL2DXLG4CXW5D6XTLJ4DDMMKHNCIAPNK2Y4IZFHTJM6HN"
idp:
  - description: "Test IDP"
    issuer_url: "https://test.idp"
    client_id: "test-client"
rbac:
  user_accounts:
    - name: "test-account"
      public_key: "%s"
      signing_nkey: "SUAGJBPRRXFQL2DXLG4CXW5D6XTLJ4DDMMKHNCIAPNK2Y4IZFHTJM6HN"
  role_binding:
    - user_account: "test-account"
      roles: ["basic-role"]
  roles:
    - name: "basic-role"
      permissions:
        pub:
          allow: ["test.>"]
        sub:
          allow: ["test.>"]
`, accountPub)

	tmpFile, err := os.CreateTemp("", "auth-test-*.yaml")
	require.NoError(t, err)
	_, err = tmpFile.WriteString(configYAML)
	require.NoError(t, err)
	tmpFile.Close()

	cm, err := NewConfigManager([]string{tmpFile.Name()})
	require.NoError(t, err)

	cfg, err := cm.GetConfig(make(map[string]interface{}))
	require.NoError(t, err)

	// Override the account info with real key pairs for signing
	cfg.Rbac.Accounts[0].PublicKey = accountPub
	cfg.Rbac.Accounts[0].SigningNKey = NKey{KeyPair: signingKP}

	ctx := NewServerContext(&Options{LogSensitive: false})

	t.Cleanup(func() {
		os.Remove(tmpFile.Name())
	})

	return &testFixture{
		accountKP:  accountKP,
		signingKP:  signingKP,
		userKP:     userKP,
		accountPub: accountPub,
		userPub:    userPub,
		config:     cfg,
		configMgr:  cm,
		ctx:        ctx,
		tmpFile:    tmpFile.Name(),
	}
}

// fakeIdpVerifier creates an IdpAndJwtVerifier stub for testing without a real OIDC provider.
func fakeIdpVerifier() *IdpAndJwtVerifier {
	return &IdpAndJwtVerifier{
		verifier: &IdpJwtVerifier{
			ctx:              &Context{Options: &Options{}},
			MaxTokenLifetime: DefaultMaxTokenLifetime,
			ClockSkew:        DefaultClockSkew,
		},
		config: &Idp{
			Description: "Test IDP",
			ClientID:    "test-client",
			IssuerURL:   "https://test.idp",
		},
	}
}

func TestBuildUserClaims_EndToEnd(t *testing.T) {
	f := newTestFixture(t)

	now := time.Now()
	idpExpiry := now.Add(30 * time.Minute).Unix()

	claims := &IdpJwtClaims{
		Subject:  "test-user-sub",
		Email:    "test@example.com",
		Name:     "Test User",
		Expiry:   idpExpiry,
		IssuedAt: now.Add(-1 * time.Minute).Unix(),
	}

	t.Run("produces valid user claims with correct fields", func(t *testing.T) {
		request := &jwt.AuthorizationRequestClaims{}
		request.UserNkey = f.userPub
		request.ConnectOptions.Username = "testuser"

		resultClaims, kp, accountInfo, status, err := buildUserClaims(
			f.ctx, f.config, f.configMgr, claims, fakeIdpVerifier(), request,
		)
		require.NoError(t, err)
		assert.Empty(t, status)
		assert.NotNil(t, resultClaims)
		assert.NotNil(t, kp)
		assert.NotNil(t, accountInfo)

		// Verify claims structure
		assert.Equal(t, "test-account", resultClaims.Audience)
		assert.Equal(t, "testuser", resultClaims.Name)
		assert.Equal(t, f.accountPub, resultClaims.IssuerAccount)
		assert.Equal(t, f.userPub, resultClaims.Subject)

		// Verify permissions from role
		assert.Contains(t, resultClaims.Permissions.Pub.Allow, "test.>")
		assert.Contains(t, resultClaims.Permissions.Sub.Allow, "test.>")

		// Verify expiry is clamped to IDP ceiling
		assert.LessOrEqual(t, resultClaims.Expires, idpExpiry)
		assert.Greater(t, resultClaims.Expires, now.Unix())
	})

	t.Run("signed JWT can be decoded and verified", func(t *testing.T) {
		request := &jwt.AuthorizationRequestClaims{}
		request.UserNkey = f.userPub
		request.ConnectOptions.Username = "signed-user"

		resultClaims, kp, accountInfo, _, err := buildUserClaims(
			f.ctx, f.config, f.configMgr, claims, fakeIdpVerifier(), request,
		)
		require.NoError(t, err)

		signedToken, err := ValidateAndSign(resultClaims, kp, accountInfo)
		require.NoError(t, err)
		assert.NotEmpty(t, signedToken)

		decoded, err := jwt.DecodeUserClaims(signedToken)
		require.NoError(t, err)
		assert.Equal(t, "test-account", decoded.Audience)
		assert.Equal(t, "signed-user", decoded.Name)
		assert.Equal(t, f.userPub, decoded.Subject)
		assert.Contains(t, decoded.Permissions.Pub.Allow, "test.>")
	})

	t.Run("unknown account returns error", func(t *testing.T) {
		badCfg := *f.config
		badCfg.Rbac.Accounts = nil

		request := &jwt.AuthorizationRequestClaims{}
		request.UserNkey = f.userPub

		_, _, _, status, err := buildUserClaims(
			f.ctx, &badCfg, f.configMgr, claims, fakeIdpVerifier(), request,
		)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unknown user-account info")
		assert.Equal(t, "error", status)
	})

	t.Run("expiry clamped to IDP ceiling when role binding extends beyond", func(t *testing.T) {
		// Create config with role binding that sets a long expiry
		longExpiryCfg := fmt.Sprintf(`
nats:
  url: "nats://localhost:4222"
  jwt_expiry_bounds:
    min: 1m
    max: 4h
service:
  name: "test-auth-svc"
  description: "Test Auth Service"
  version: "1.0.0"
  creds_file: "/dev/null"
  account:
    name: "test"
    signing_nkey: "SUAGJBPRRXFQL2DXLG4CXW5D6XTLJ4DDMMKHNCIAPNK2Y4IZFHTJM6HN"
idp:
  - description: "Test IDP"
    issuer_url: "https://test.idp"
    client_id: "test-client"
rbac:
  user_accounts:
    - name: "test-account"
      public_key: "%s"
      signing_nkey: "SUAGJBPRRXFQL2DXLG4CXW5D6XTLJ4DDMMKHNCIAPNK2Y4IZFHTJM6HN"
  role_binding:
    - user_account: "test-account"
      roles: ["basic-role"]
      token_max_expiration: 2h
  roles:
    - name: "basic-role"
      permissions:
        pub:
          allow: ["test.>"]
`, f.accountPub)

		tmpFile, err := os.CreateTemp("", "expiry-test-*.yaml")
		require.NoError(t, err)
		defer os.Remove(tmpFile.Name())
		_, err = tmpFile.WriteString(longExpiryCfg)
		require.NoError(t, err)
		tmpFile.Close()

		cm, err := NewConfigManager([]string{tmpFile.Name()})
		require.NoError(t, err)
		cfg, err := cm.GetConfig(make(map[string]interface{}))
		require.NoError(t, err)
		cfg.Rbac.Accounts[0].PublicKey = f.accountPub
		cfg.Rbac.Accounts[0].SigningNKey = NKey{KeyPair: f.signingKP}

		// IDP token expires in 30 minutes
		shortClaims := &IdpJwtClaims{
			Subject:  "short-expiry-user",
			Email:    "short@test.com",
			Name:     "Short Expiry",
			Expiry:   now.Add(30 * time.Minute).Unix(),
			IssuedAt: now.Add(-1 * time.Minute).Unix(),
		}

		request := &jwt.AuthorizationRequestClaims{}
		request.UserNkey = f.userPub

		resultClaims, _, _, _, err := buildUserClaims(
			f.ctx, cfg, cm, shortClaims, fakeIdpVerifier(), request,
		)
		require.NoError(t, err)

		// Role binding wants 2h, but IDP ceiling is 30m — must be clamped
		assert.LessOrEqual(t, resultClaims.Expires, shortClaims.Expiry,
			"expiry must not exceed IDP-provided ceiling")
		assert.InDelta(t, shortClaims.Expiry, resultClaims.Expires, 2,
			"expiry should be at IDP ceiling (within timing tolerance)")
	})
}

func TestPublishAuditEvent_Format(t *testing.T) {
	f := newTestFixture(t)

	now := time.Now()
	claims := &jwt.UserClaims{}
	claims.Subject = f.userPub
	claims.Audience = "test-account"
	claims.Expires = now.Add(30 * time.Minute).Unix()

	request := &jwt.AuthorizationRequestClaims{}
	request.UserNkey = f.userPub
	request.ConnectOptions.Username = "audituser"

	idpClaims := &IdpJwtClaims{
		Email: "audit@test.com",
		Name:  "Audit User",
	}

	accountInfo := &UserAccountInfo{
		Name:        "test-account",
		PublicKey:   f.accountPub,
		SigningNKey: NKey{KeyPair: f.signingKP},
	}

	// Build the event manually to verify the structure
	signingKeyInfo, _ := determineSigningKeyType(claims, f.signingKP, accountInfo)

	userEvent := map[string]interface{}{
		"account":          claims.Audience,
		"account_pub_nkey": accountInfo.PublicKey,
		"user_pub_nkey":    request.UserNkey,
		"username":         request.ConnectOptions.Username,
		"email":            idpClaims.Email,
		"name":             idpClaims.Name,
		"idp":              "Test IDP",
		"created_at":       now.Format(time.RFC3339),
		"expires_at":       time.Unix(claims.Expires, 0).Format(time.RFC3339),
		"permissions":      &claims.Permissions,
		"limits":           &claims.Limits,
		"signing_account":  "test",
	}

	if signingKeyInfo != nil {
		userEvent["signing_key_type"] = signingKeyInfo.Type
		userEvent["signing_key_pub_nkey"] = signingKeyInfo.PublicKey
	}

	eventJSON, err := json.Marshal(userEvent)
	require.NoError(t, err)

	var parsed map[string]interface{}
	require.NoError(t, json.Unmarshal(eventJSON, &parsed))

	assert.Equal(t, "test-account", parsed["account"])
	assert.Equal(t, f.accountPub, parsed["account_pub_nkey"])
	assert.Equal(t, f.userPub, parsed["user_pub_nkey"])
	assert.Equal(t, "audituser", parsed["username"])
	assert.Equal(t, "audit@test.com", parsed["email"])
	assert.Equal(t, "Audit User", parsed["name"])
	assert.Equal(t, "Test IDP", parsed["idp"])
	assert.NotEmpty(t, parsed["created_at"])
	assert.NotEmpty(t, parsed["expires_at"])
}

func TestPublishAuditEvent_TraceparentHeader(t *testing.T) {
	ensurePropagator(t)
	f := newTestFixture(t)

	// Start an embedded NATS server
	opts := &natsserver.Options{
		Host: "127.0.0.1",
		Port: -1, // random port
	}
	ns, err := natsserver.NewServer(opts)
	require.NoError(t, err)
	go ns.Start()
	if !ns.ReadyForConnections(5 * time.Second) {
		t.Fatal("NATS server failed to start")
	}
	defer ns.Shutdown()

	nc, err := nats.Connect(ns.ClientURL())
	require.NoError(t, err)
	defer nc.Close()

	// Subscribe to the audit subject
	auditSubject := "test-svc.evt.audit.account.test-account.user." + f.userPub + ".created"
	sub, err := nc.SubscribeSync(auditSubject)
	require.NoError(t, err)
	require.NoError(t, nc.Flush())

	// Create a context with a known trace (via traceparent extraction, same as production path)
	tp := "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"
	ctx := tracing.ExtractFromTraceparent(tp)

	// Build test data
	claims := &jwt.UserClaims{}
	claims.Subject = f.userPub
	claims.Audience = "test-account"
	claims.Expires = time.Now().Add(30 * time.Minute).Unix()

	request := &jwt.AuthorizationRequestClaims{}
	request.UserNkey = f.userPub
	request.ConnectOptions.Username = "traceuser"

	idpClaims := &IdpJwtClaims{Email: "trace@test.com", Name: "Trace User"}
	accountInfo := &UserAccountInfo{
		Name:        "test-account",
		PublicKey:   f.accountPub,
		SigningNKey: NKey{KeyPair: f.signingKP},
	}

	// Publish with trace context
	publishAuditEvent(ctx, nc, "test-svc.evt.audit.account.%s.user.%s.created",
		f.config, claims, request, idpClaims, fakeIdpVerifier(), accountInfo)
	require.NoError(t, nc.Flush())

	// Receive and verify traceparent header
	msg, err := sub.NextMsg(2 * time.Second)
	require.NoError(t, err)
	assert.NotNil(t, msg.Header)

	// nats.Header is case-sensitive; W3C propagator uses lowercase "traceparent"
	traceparent := msg.Header.Get("traceparent")
	assert.NotEmpty(t, traceparent, "expected traceparent header on audit message")
	assert.Contains(t, traceparent, "4bf92f3577b34da6a3ce929d0e0e4736",
		"traceparent should contain the original trace ID")

	// Verify the trace context can be extracted by a downstream consumer
	extractedCtx := tracing.ExtractTraceContext(msg.Header)
	extractedSC := trace.SpanContextFromContext(extractedCtx)
	assert.True(t, extractedSC.IsValid())
	expectedTraceID, _ := trace.TraceIDFromHex("4bf92f3577b34da6a3ce929d0e0e4736")
	assert.Equal(t, expectedTraceID, extractedSC.TraceID())
}

func TestExtractJWT(t *testing.T) {
	ctx := NewServerContext(&Options{})

	t.Run("extracts JWT from token field", func(t *testing.T) {
		request := &jwt.AuthorizationRequestClaims{}
		request.ConnectOptions.Token = "raw-jwt-token"

		rawJwt, tokenReq := extractJWT(ctx, request)
		assert.Equal(t, "raw-jwt-token", rawJwt)
		assert.Empty(t, tokenReq.IDToken)
	})

	t.Run("extracts JWT from JSON token field", func(t *testing.T) {
		tokenJSON := `{"id_token":"my-id-token","access_token":"my-access-token"}`
		request := &jwt.AuthorizationRequestClaims{}
		request.ConnectOptions.Token = tokenJSON

		rawJwt, tokenReq := extractJWT(ctx, request)
		assert.Equal(t, "my-id-token", rawJwt)
		assert.Equal(t, "my-access-token", tokenReq.AccessToken)
	})

	t.Run("extracts JWT from password field when token is empty", func(t *testing.T) {
		request := &jwt.AuthorizationRequestClaims{}
		request.ConnectOptions.Password = "password-jwt-token"

		rawJwt, tokenReq := extractJWT(ctx, request)
		assert.Equal(t, "password-jwt-token", rawJwt)
		assert.Empty(t, tokenReq.IDToken)
	})

	t.Run("extracts JWT from JSON password field", func(t *testing.T) {
		tokenJSON := `{"id_token":"pw-id-token","access_token":"pw-access"}`
		request := &jwt.AuthorizationRequestClaims{}
		request.ConnectOptions.Password = tokenJSON

		rawJwt, tokenReq := extractJWT(ctx, request)
		assert.Equal(t, "pw-id-token", rawJwt)
		assert.Equal(t, "pw-access", tokenReq.AccessToken)
	})

	t.Run("returns empty when no token or password", func(t *testing.T) {
		request := &jwt.AuthorizationRequestClaims{}

		rawJwt, _ := extractJWT(ctx, request)
		assert.Empty(t, rawJwt)
	})
}

func TestConcurrentAuthDuringConfigReload(t *testing.T) {
	f := newTestFixture(t)

	initial := &LiveState{
		config:        f.config,
		configManager: f.configMgr,
		idpVerifiers:  nil,
		auditSubject:  "test-auth-svc.evt.audit.account.%s.user.%s.created",
	}

	watcher := NewConfigWatcher(f.ctx, []string{f.tmpFile}, initial)

	const numReaders = 20
	const numReloads = 5
	const readsPerGoroutine = 50

	var wg sync.WaitGroup
	errCh := make(chan error, numReaders*readsPerGoroutine+numReloads)

	// Start concurrent readers that snapshot state and use it
	for i := 0; i < numReaders; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < readsPerGoroutine; j++ {
				state := watcher.State()
				if state == nil {
					errCh <- fmt.Errorf("state is nil")
					return
				}
				if state.config == nil {
					errCh <- fmt.Errorf("config is nil")
					return
				}
				if state.configManager == nil {
					errCh <- fmt.Errorf("configManager is nil")
					return
				}
				// Simulate rendering config with claims (as handleAuthRequest does)
				cfg, err := state.configManager.GetConfig(map[string]interface{}{
					"email": "concurrent@test.com",
				})
				if err != nil {
					errCh <- fmt.Errorf("GetConfig failed: %w", err)
					return
				}
				if cfg.Service.Name != "test-auth-svc" {
					errCh <- fmt.Errorf("unexpected service name: %s", cfg.Service.Name)
					return
				}
			}
		}()
	}

	// Concurrent config swaps (simulating reloads via direct state swap)
	for i := 0; i < numReloads; i++ {
		wg.Add(1)
		go func(iteration int) {
			defer wg.Done()
			newCM, err := NewConfigManager([]string{f.tmpFile})
			if err != nil {
				errCh <- fmt.Errorf("reload %d: NewConfigManager failed: %w", iteration, err)
				return
			}
			newCfg, err := newCM.GetConfig(make(map[string]interface{}))
			if err != nil {
				errCh <- fmt.Errorf("reload %d: GetConfig failed: %w", iteration, err)
				return
			}
			newCfg.Rbac.Accounts[0].PublicKey = f.accountPub
			newCfg.Rbac.Accounts[0].SigningNKey = NKey{KeyPair: f.signingKP}

			newState := &LiveState{
				config:        newCfg,
				configManager: newCM,
				idpVerifiers:  nil,
				auditSubject:  "test-auth-svc.evt.audit.account.%s.user.%s.created",
			}
			watcher.state.Store(newState)
		}(i)
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Errorf("concurrent error: %v", err)
	}
}

func TestConfigWatcherSnapshotConsistency(t *testing.T) {
	f := newTestFixture(t)

	initial := &LiveState{
		config:        f.config,
		configManager: f.configMgr,
		idpVerifiers:  nil,
		auditSubject:  "test-auth-svc.evt.audit.account.%s.user.%s.created",
	}

	watcher := NewConfigWatcher(f.ctx, []string{f.tmpFile}, initial)

	// Take a snapshot before reload
	snapshot := watcher.State()

	// Swap state to something new
	newCM, err := NewConfigManager([]string{f.tmpFile})
	require.NoError(t, err)
	newCfg, err := newCM.GetConfig(make(map[string]interface{}))
	require.NoError(t, err)
	newCfg.Service.Name = "swapped-service"

	watcher.state.Store(&LiveState{
		config:        newCfg,
		configManager: newCM,
		idpVerifiers:  nil,
		auditSubject:  "swapped.audit.%s.%s",
	})

	// Original snapshot must still have old values (request-local consistency)
	assert.Equal(t, "test-auth-svc", snapshot.config.Service.Name)
	assert.Equal(t, "test-auth-svc.evt.audit.account.%s.user.%s.created", snapshot.auditSubject)

	// New reads should see swapped values
	current := watcher.State()
	assert.Equal(t, "swapped-service", current.config.Service.Name)
	assert.Equal(t, "swapped.audit.%s.%s", current.auditSubject)
}
