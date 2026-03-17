//go:build integration

package integration

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/nats-io/jwt/v2"
	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nkeys"
	"github.com/stretchr/testify/require"
)

// TestAccount holds keypairs and JWT for a NATS account created programmatically.
type TestAccount struct {
	Name       string
	AccountKP  nkeys.KeyPair
	SigningKP  nkeys.KeyPair
	AccountPub string
	SigningPub string
	Users      map[string]*TestUser
	JWT        string
}

// TestUser holds keypairs and credentials for a NATS user.
type TestUser struct {
	Name      string
	UserKP    nkeys.KeyPair
	UserPub   string
	CredsFile string
}

// TestCluster holds an embedded NATS server and all generated keys/creds.
type TestCluster struct {
	Server      *server.Server
	OperatorKP  nkeys.KeyPair
	OperatorPub string
	SysAccount  *TestAccount
	MintAccount *TestAccount
	AppAccount  *TestAccount
	URL         string
	Dir         string
	XKeyPair    nkeys.KeyPair
	XKeyPub     string
}

// SetupTestCluster creates an embedded NATS server with operator, SYS, MINT (auth callout),
// and APP accounts — all programmatically, no NSC binary required.
func SetupTestCluster(t *testing.T) *TestCluster {
	t.Helper()

	dir := t.TempDir()

	// Create operator
	operatorKP, err := nkeys.CreateOperator()
	require.NoError(t, err)
	operatorPub, err := operatorKP.PublicKey()
	require.NoError(t, err)

	operatorClaims := jwt.NewOperatorClaims(operatorPub)
	operatorClaims.Name = "test-operator"

	// Create SYS account
	sysAccount := createTestAccount(t, "SYS")
	operatorClaims.SystemAccount = sysAccount.AccountPub

	// Create MINT account (auth callout)
	mintAccount := createTestAccount(t, "MINT")

	// Create xkey for encryption
	xkp, err := nkeys.CreateCurveKeys()
	require.NoError(t, err)
	xkeyPub, err := xkp.PublicKey()
	require.NoError(t, err)

	// Create users on MINT account
	nobodyUser := createTestUser(t, mintAccount, "nobody")
	minterUser := createTestUser(t, mintAccount, "minter")

	// Configure auth callout on MINT account claims
	mintAccountClaims := jwt.NewAccountClaims(mintAccount.AccountPub)
	mintAccountClaims.Name = "MINT"
	mintAccountClaims.SigningKeys.Add(mintAccount.SigningPub)
	mintAccountClaims.Authorization.AuthUsers = append(mintAccountClaims.Authorization.AuthUsers, minterUser.UserPub)
	mintAccountClaims.Authorization.AllowedAccounts = append(mintAccountClaims.Authorization.AllowedAccounts, jwt.AnyAccount)
	mintAccountClaims.Authorization.XKey = xkeyPub
	mintAccount.JWT, err = mintAccountClaims.Encode(operatorKP)
	require.NoError(t, err)

	// Create APP1 account with JetStream
	appAccount := createTestAccount(t, "APP1")
	appAccountClaims := jwt.NewAccountClaims(appAccount.AccountPub)
	appAccountClaims.Name = "APP1"
	appAccountClaims.SigningKeys.Add(appAccount.SigningPub)
	appAccountClaims.Limits.JetStreamLimits.MemoryStorage = 1024 * 1024
	appAccountClaims.Limits.JetStreamLimits.DiskStorage = 1024 * 1024
	appAccountClaims.Limits.JetStreamLimits.Streams = 10
	appAccountClaims.Limits.JetStreamLimits.Consumer = 100
	appAccount.JWT, err = appAccountClaims.Encode(operatorKP)
	require.NoError(t, err)

	// Create SYS account JWT
	sysAccountClaims := jwt.NewAccountClaims(sysAccount.AccountPub)
	sysAccountClaims.Name = "SYS"
	sysAccountClaims.SigningKeys.Add(sysAccount.SigningPub)
	sysAccount.JWT, err = sysAccountClaims.Encode(operatorKP)
	require.NoError(t, err)

	// Create SYS user for admin operations
	sysUser := createTestUser(t, sysAccount, "sys")

	// Write credential files
	writeCredsFile(t, dir, sysAccount, sysUser)
	writeCredsFile(t, dir, mintAccount, nobodyUser)
	writeCredsFile(t, dir, mintAccount, minterUser)

	// Start embedded NATS server
	jsDir := filepath.Join(dir, "jetstream")
	require.NoError(t, os.MkdirAll(jsDir, 0o755))

	opts := &server.Options{
		Port:          -1, // random port
		JetStream:     true,
		StoreDir:      jsDir,
		TrustedKeys:   []string{operatorPub},
		SystemAccount: sysAccount.AccountPub,
	}

	resolver := server.MemAccResolver{}
	require.NoError(t, resolver.Store(sysAccount.AccountPub, sysAccount.JWT))
	require.NoError(t, resolver.Store(mintAccount.AccountPub, mintAccount.JWT))
	require.NoError(t, resolver.Store(appAccount.AccountPub, appAccount.JWT))
	opts.AccountResolver = &resolver

	ns, err := server.NewServer(opts)
	require.NoError(t, err)

	ns.ConfigureLogger()
	go ns.Start()
	if !ns.ReadyForConnections(10 * time.Second) {
		t.Fatal("NATS server failed to start")
	}

	tc := &TestCluster{
		Server:      ns,
		OperatorKP:  operatorKP,
		OperatorPub: operatorPub,
		SysAccount:  sysAccount,
		MintAccount: mintAccount,
		AppAccount:  appAccount,
		URL:         ns.ClientURL(),
		Dir:         dir,
		XKeyPair:    xkp,
		XKeyPub:     xkeyPub,
	}

	t.Cleanup(func() {
		ns.Shutdown()
		ns.WaitForShutdown()
	})

	return tc
}

func createTestAccount(t *testing.T, name string) *TestAccount {
	t.Helper()

	accountKP, err := nkeys.CreateAccount()
	require.NoError(t, err)
	accountPub, err := accountKP.PublicKey()
	require.NoError(t, err)

	signingKP, err := nkeys.CreateAccount()
	require.NoError(t, err)
	signingPub, err := signingKP.PublicKey()
	require.NoError(t, err)

	return &TestAccount{
		Name:       name,
		AccountKP:  accountKP,
		SigningKP:  signingKP,
		AccountPub: accountPub,
		SigningPub: signingPub,
		Users:      make(map[string]*TestUser),
	}
}

func createTestUser(t *testing.T, account *TestAccount, name string) *TestUser {
	t.Helper()

	userKP, err := nkeys.CreateUser()
	require.NoError(t, err)
	userPub, err := userKP.PublicKey()
	require.NoError(t, err)

	// Create user JWT signed by the account's signing key
	userClaims := jwt.NewUserClaims(userPub)
	userClaims.Name = name
	userClaims.IssuerAccount = account.AccountPub

	if name == "nobody" {
		// Deny all pub/sub for the sentinel user
		userClaims.Permissions.Pub.Deny.Add(">")
		userClaims.Permissions.Sub.Deny.Add(">")
	}

	userJWT, err := userClaims.Encode(account.SigningKP)
	require.NoError(t, err)

	user := &TestUser{
		Name:    name,
		UserKP:  userKP,
		UserPub: userPub,
	}
	// Store the JWT temporarily on the user for creds file generation
	user.CredsFile = userJWT // will be replaced with actual file path
	account.Users[name] = user

	return user
}

func writeCredsFile(t *testing.T, dir string, account *TestAccount, user *TestUser) {
	t.Helper()

	accountDir := filepath.Join(dir, account.Name)
	require.NoError(t, os.MkdirAll(accountDir, 0o755))

	// Regenerate the user JWT for the creds file
	userClaims := jwt.NewUserClaims(user.UserPub)
	userClaims.Name = user.Name
	userClaims.IssuerAccount = account.AccountPub

	if user.Name == "nobody" {
		userClaims.Permissions.Pub.Deny.Add(">")
		userClaims.Permissions.Sub.Deny.Add(">")
	}

	userJWT, err := userClaims.Encode(account.SigningKP)
	require.NoError(t, err)

	seed, err := user.UserKP.Seed()
	require.NoError(t, err)

	credsContent := fmt.Sprintf("-----BEGIN NATS USER JWT-----\n%s\n------END NATS USER JWT------\n\n-----BEGIN USER NKEY SEED-----\n%s\n------END USER NKEY SEED------\n",
		userJWT, string(seed))

	credsPath := filepath.Join(accountDir, fmt.Sprintf("user-%s.creds", user.Name))
	require.NoError(t, os.WriteFile(credsPath, []byte(credsContent), 0o600))
	user.CredsFile = credsPath
}

// WriteAccountKeys writes account public key and signing key to files in the test directory,
// matching the format expected by the broker's RBAC config.
func (tc *TestCluster) WriteAccountKeys(t *testing.T, account *TestAccount) (pubKeyFile, signingKeyFile string) {
	t.Helper()

	accountDir := filepath.Join(tc.Dir, account.Name)
	require.NoError(t, os.MkdirAll(accountDir, 0o755))

	pubKeyFile = filepath.Join(accountDir, "acct-pubkey.pub")
	require.NoError(t, os.WriteFile(pubKeyFile, []byte(account.AccountPub), 0o600))

	signingKeySeed, err := account.SigningKP.Seed()
	require.NoError(t, err)
	signingKeyFile = filepath.Join(accountDir, "acct-signing-key.nk")
	require.NoError(t, os.WriteFile(signingKeyFile, signingKeySeed, 0o600))

	return pubKeyFile, signingKeyFile
}

// WriteXKey writes the xkey seed to a file for the broker config.
func (tc *TestCluster) WriteXKey(t *testing.T) string {
	t.Helper()

	xkeyDir := filepath.Join(tc.Dir, tc.MintAccount.Name)
	require.NoError(t, os.MkdirAll(xkeyDir, 0o755))

	seed, err := tc.XKeyPair.Seed()
	require.NoError(t, err)

	xkeyFile := filepath.Join(xkeyDir, "acct-encryption-key.xk")
	content := fmt.Sprintf("%s\n%s\n", tc.XKeyPub, string(seed))
	require.NoError(t, os.WriteFile(xkeyFile, []byte(content), 0o600))

	return xkeyFile
}

// ConnectAsUser creates a NATS connection using the specified user's credentials.
func (tc *TestCluster) ConnectAsUser(t *testing.T, account *TestAccount, userName string, opts ...nats.Option) *nats.Conn {
	t.Helper()

	user, ok := account.Users[userName]
	require.True(t, ok, "user %s not found in account %s", userName, account.Name)

	allOpts := append([]nats.Option{
		nats.UserCredentials(user.CredsFile),
		nats.MaxReconnects(0),
	}, opts...)

	nc, err := nats.Connect(tc.URL, allOpts...)
	require.NoError(t, err)

	t.Cleanup(func() {
		nc.Close()
	})

	return nc
}

// ConnectWithToken creates a NATS connection using the nobody user's credentials
// and an OIDC JWT token for auth callout.
func (tc *TestCluster) ConnectWithToken(t *testing.T, idToken string) (*nats.Conn, error) {
	t.Helper()

	nobody := tc.MintAccount.Users["nobody"]
	require.NotNil(t, nobody, "nobody user not found in MINT account")

	nc, err := nats.Connect(tc.URL,
		nats.UserCredentials(nobody.CredsFile),
		nats.Token(idToken),
		nats.MaxReconnects(0),
	)
	return nc, err
}
