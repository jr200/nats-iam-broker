package broker

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jr200/nats-iam-broker/internal/metrics"
	"github.com/nats-io/jwt/v2"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nkeys"
	"go.uber.org/zap"
)

func newAuthCallbackWithWatcher(
	ctx *Context,
	m *metrics.Metrics,
	nc *nats.Conn,
	watcher *ConfigWatcher,
) AuthHandler {
	return func(request *jwt.AuthorizationRequestClaims) (*jwt.UserClaims, nkeys.KeyPair, *UserAccountInfo, error) {
		// Snapshot current state at request start. All operations in this
		// request use this snapshot, ensuring consistency even if a reload
		// happens mid-request.
		state := watcher.State()
		return handleAuthRequest(ctx, m, nc, state.config, state.configManager, state.idpVerifiers, state.auditSubject, request)
	}
}

func handleAuthRequest(
	ctx *Context,
	m *metrics.Metrics,
	nc *nats.Conn,
	config *Config,
	configManager *ConfigManager,
	idpVerifiers []IdpAndJwtVerifier,
	auditEventSubject string,
	request *jwt.AuthorizationRequestClaims,
) (*jwt.UserClaims, nkeys.KeyPair, *UserAccountInfo, error) {
	requestStart := time.Now()
	if m != nil {
		m.AuthRequestsInFlight.Inc()
		defer m.AuthRequestsInFlight.Dec()
	}

	recordResult := func(status string) {
		if m != nil {
			duration := time.Since(requestStart).Seconds()
			m.AuthRequestsTotal.WithLabelValues(status).Inc()
			m.AuthRequestDuration.WithLabelValues(status).Observe(duration)
		}
	}

	idpRawJwt, tokenReq := extractJWT(ctx, request)
	if idpRawJwt == "" {
		recordResult(metrics.StatusError)
		return nil, nil, nil, fmt.Errorf("no valid JWT token found in request")
	}

	reqClaims, matchedVerifier, _, err := verifyAndEnrich(m, idpRawJwt, tokenReq, idpVerifiers)
	if err != nil {
		recordResult(metrics.StatusDenied)
		return nil, nil, nil, err
	}

	// Merge in client information from the request
	reqJwtClaims := reqClaims.toMap()
	reqJwtClaims["client_id"] = request.ClientInformation.User        // Sentinel ID
	reqJwtClaims["also_known_as"] = request.ClientInformation.NameTag // Sentinel name
	reqClaims.fromMap(reqJwtClaims, matchedVerifier.config.CustomMapping)

	if ctx.Options.LogSensitive {
		zap.L().Debug("reqClaims", zap.Any("claims", reqClaims.toMap()))
	}

	claims, signingKeyPair, userAccountInfo, resultStatus, err := buildUserClaims(ctx, config, configManager, reqClaims, matchedVerifier, request)
	if err != nil {
		recordResult(resultStatus)
		return nil, nil, nil, err
	}

	publishAuditEvent(nc, auditEventSubject, config, claims, request, reqClaims, matchedVerifier, userAccountInfo)

	recordResult(metrics.StatusSuccess)
	if m != nil {
		m.TokensMinted.WithLabelValues(claims.Audience, matchedVerifier.config.Description).Inc()
	}

	return claims, signingKeyPair, userAccountInfo, nil
}

func extractJWT(ctx *Context, request *jwt.AuthorizationRequestClaims) (string, TokenRequest) {
	var tokenReq TokenRequest
	var idpRawJwt string

	if ctx.Options.LogSensitive {
		zap.L().Debug("NewAuthService request", zap.Any("request", request))
	}

	if request.ConnectOptions.Token != "" {
		if err := json.Unmarshal([]byte(request.ConnectOptions.Token), &tokenReq); err == nil {
			idpRawJwt = tokenReq.IDToken
		} else {
			idpRawJwt = request.ConnectOptions.Token
		}
	} else {
		if err := json.Unmarshal([]byte(request.ConnectOptions.Password), &tokenReq); err == nil {
			idpRawJwt = tokenReq.IDToken
		} else {
			idpRawJwt = request.ConnectOptions.Password
		}
	}

	return idpRawJwt, tokenReq
}

func verifyAndEnrich(
	m *metrics.Metrics,
	idpRawJwt string,
	tokenReq TokenRequest,
	idpVerifiers []IdpAndJwtVerifier,
) (*IdpJwtClaims, *IdpAndJwtVerifier, TokenRequest, error) {
	verifyStart := time.Now()
	reqClaims, matchedVerifier, verifiedIDToken, err := runVerification(idpRawJwt, idpVerifiers)
	if err != nil {
		if m != nil {
			m.IDPVerifyTotal.WithLabelValues("unknown", metrics.StatusError).Inc()
		}
		return nil, nil, tokenReq, err
	}
	if m != nil {
		idpDesc := matchedVerifier.config.Description
		m.IDPVerifyTotal.WithLabelValues(idpDesc, metrics.StatusSuccess).Inc()
		m.IDPVerifyDuration.WithLabelValues(idpDesc).Observe(time.Since(verifyStart).Seconds())
	}

	if matchedVerifier.config.UserInfo.Enabled {
		if tokenReq.AccessToken != "" {
			userInfoCtx, userInfoCancel := context.WithTimeout(context.Background(), oidcTimeout)
			defer userInfoCancel()
			userInfo, err := matchedVerifier.verifier.GetUserInfo(userInfoCtx, tokenReq.AccessToken, verifiedIDToken)
			if err != nil {
				zap.L().Warn("failed to fetch user info", zap.Error(err))
			} else {
				claims := reqClaims.toMap()
				for k, v := range userInfo {
					claims[k] = v
				}
				reqClaims.fromMap(claims, matchedVerifier.config.CustomMapping)
			}
		} else {
			zap.L().Debug("skipping user info fetch - no access token available")
		}
	}

	return reqClaims, matchedVerifier, tokenReq, nil
}

func buildUserClaims(
	ctx *Context,
	config *Config,
	configManager *ConfigManager,
	reqClaims *IdpJwtClaims,
	matchedVerifier *IdpAndJwtVerifier,
	request *jwt.AuthorizationRequestClaims,
) (*jwt.UserClaims, nkeys.KeyPair, *UserAccountInfo, string, error) {
	cfgForRequest, err := configManager.GetConfig(reqClaims.toMap())
	if err != nil {
		zap.L().Error("error rendering config against idp-jwt", zap.Error(err))
		return nil, nil, nil, metrics.StatusError, err
	}

	userAccountName, permissions, limits, roleBindingTokenMaxExpiry, err := cfgForRequest.lookupUserAccount(reqClaims.toMap())
	if err != nil {
		zap.L().Error("error looking up user account", zap.Error(err))
		return nil, nil, nil, metrics.StatusDenied, err
	}

	userAccountInfo, err := config.lookupAccountInfo(userAccountName)
	if err != nil {
		zap.L().Error("error looking up account-info", zap.Error(err))
		return nil, nil, nil, metrics.StatusError, err
	}

	if ctx.Options.LogSensitive {
		zap.L().Debug("userAccountInfo", zap.Any("info", userAccountInfo))
	}

	claims := jwt.NewUserClaims(request.UserNkey)
	claims.Audience = userAccountName
	claims.Name = request.ConnectOptions.Username
	claims.IssuerAccount = userAccountInfo.PublicKey
	claims.Expires = calculateExpiration(
		cfgForRequest,
		reqClaims.Expiry,
		&matchedVerifier.config.ValidationSpec.TokenExpiryBounds,
		&roleBindingTokenMaxExpiry,
	)
	claims.Permissions = *permissions
	claims.Limits = *limits
	claims.Tags.Add(fmt.Sprintf("email: %s, name: %s, idp: %s, expires: %s",
		reqClaims.Email,
		reqClaims.Name,
		matchedVerifier.config.Description,
		time.Unix(claims.Expires, 0).Format(time.RFC3339)))

	return claims, userAccountInfo.SigningNKey.KeyPair, userAccountInfo, "", nil
}

func publishAuditEvent(
	nc *nats.Conn,
	auditEventSubject string,
	config *Config,
	claims *jwt.UserClaims,
	request *jwt.AuthorizationRequestClaims,
	reqClaims *IdpJwtClaims,
	matchedVerifier *IdpAndJwtVerifier,
	userAccountInfo *UserAccountInfo,
) {
	signingKeyInfo, err := determineSigningKeyType(claims, userAccountInfo.SigningNKey.KeyPair, userAccountInfo)
	if err != nil {
		zap.L().Warn("failed to determine signing key type for audit event", zap.Error(err))
	}

	userEvent := map[string]interface{}{
		"account":          claims.Audience,
		"account_pub_nkey": userAccountInfo.PublicKey,
		"user_pub_nkey":    request.UserNkey,
		"username":         request.ConnectOptions.Username,
		"email":            reqClaims.Email,
		"name":             reqClaims.Name,
		"idp":              matchedVerifier.config.Description,
		"created_at":       time.Now().Format(time.RFC3339),
		"expires_at":       time.Unix(claims.Expires, 0).Format(time.RFC3339),
		"permissions":      &claims.Permissions,
		"limits":           &claims.Limits,
		"signing_account":  config.Service.Account.Name,
	}

	if signingKeyInfo != nil {
		userEvent["signing_key_type"] = signingKeyInfo.Type
		userEvent["signing_key_pub_nkey"] = signingKeyInfo.PublicKey
	}

	eventJSON, err := json.Marshal(userEvent)
	if err != nil {
		zap.L().Warn("failed to marshal user creation event", zap.Error(err))
		return
	}

	err = nc.Publish(fmt.Sprintf(auditEventSubject, claims.Audience, request.UserNkey), eventJSON)
	if err != nil {
		zap.L().Warn("failed to publish user creation event", zap.Error(err))
	}
}
