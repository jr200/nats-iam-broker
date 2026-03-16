package broker

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	internal "github.com/jr200/nats-iam-broker/internal"
	"github.com/nats-io/jwt/v2"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/micro"
	"github.com/nats-io/nkeys"
	"go.uber.org/zap"
)

func Start(configFiles []string, serverOpts *Options) error {
	ctx := NewServerContext(serverOpts)
	// This is reads the config from disk on server start. Downside with caching is that if the config
	// is updated, the service will not pick it up until the service is restarted.
	configManager, err := NewConfigManager(configFiles)
	if err != nil {
		return fmt.Errorf("failed to initialize config manager: %v", err)
	}

	config, err := configManager.GetConfig(make(map[string]interface{}))
	if err != nil {
		zap.L().Error("bad configuration", zap.Error(err))
		return err
	}

	zap.ReplaceGlobals(zap.L().Named(config.Service.Name))

	// Connect to NATS
	natsOpts := config.natsOptions()
	natsOpts = append(natsOpts, nats.Name(config.Service.Name))

	natsDrainConnection := func(nc *nats.Conn) {
		if nc != nil {
			err := nc.Drain()
			if err != nil {
				zap.L().Error("error draining NATS connection", zap.Error(err))
			}
		}
	}

	zap.L().Info("connecting to NATS", zap.String("url", config.NATS.URL))
	nc, err := nats.Connect(config.NATS.URL, natsOpts...)
	if err != nil {
		return err
	}

	defer natsDrainConnection(nc)

	idpVerifiers, err := NewIdpVerifiers(ctx, config)
	if err != nil {
		return err
	}

	auditEventSubject := config.Service.Name + ".evt.audit.account.%s.user.%s.created"
	//nolint:mnd // 2 is the number of %s placeholders in auditEventSubject
	zap.L().Info("audit events configured", zap.String("subject_pattern", strings.Replace(auditEventSubject, "%s", "*", 2)))

	auth := NewAuthService(ctx, config.Service.Account.SigningNKey.KeyPair, config.serviceEncryptionXkey(), func(request *jwt.AuthorizationRequestClaims) (*jwt.UserClaims, nkeys.KeyPair, *UserAccountInfo, error) {
		var tokenReq TokenRequest
		var idpRawJwt string

		if ctx.Options.LogSensitive {
			zap.L().Debug("NewAuthService request", zap.Any("request", request))
		}

		if request.ConnectOptions.Token != "" {
			// Try to parse as JSON token response first
			if err := json.Unmarshal([]byte(request.ConnectOptions.Token), &tokenReq); err == nil {
				idpRawJwt = tokenReq.IDToken
			} else {
				// If not JSON, treat as raw JWT
				idpRawJwt = request.ConnectOptions.Token
			}
		} else {
			// Try password field if token is empty
			if err := json.Unmarshal([]byte(request.ConnectOptions.Password), &tokenReq); err == nil {
				idpRawJwt = tokenReq.IDToken
			} else {
				idpRawJwt = request.ConnectOptions.Password
			}
		}

		if idpRawJwt == "" {
			return nil, nil, nil, fmt.Errorf("no valid JWT token found in request")
		}

		reqClaims, matchedVerifier, verifiedIDToken, err := runVerification(idpRawJwt, idpVerifiers)
		if err != nil {
			return nil, nil, nil, err
		}

		// Only fetch and append user info if enabled for this IDP
		if matchedVerifier.config.UserInfo.Enabled {
			if tokenReq.AccessToken != "" {
				userInfoCtx, userInfoCancel := context.WithTimeout(context.Background(), oidcTimeout)
				defer userInfoCancel()
				userInfo, err := matchedVerifier.verifier.GetUserInfo(userInfoCtx, tokenReq.AccessToken, verifiedIDToken)
				if err != nil {
					zap.L().Warn("failed to fetch user info", zap.Error(err))
				} else {
					// Merge user info into claims
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

		// Merge in client information from the request
		reqJwtClaims := reqClaims.toMap()
		reqJwtClaims["client_id"] = request.ClientInformation.User        // Sentinel ID
		reqJwtClaims["also_known_as"] = request.ClientInformation.NameTag // Sentinel name
		reqClaims.fromMap(reqJwtClaims, matchedVerifier.config.CustomMapping)

		if ctx.Options.LogSensitive {
			zap.L().Debug("reqClaims", zap.Any("claims", reqClaims.toMap()))
		}

		// Lets render the config with a different mapping:
		cfgForRequest, err := configManager.GetConfig(reqClaims.toMap())
		if err != nil {
			zap.L().Error("error rendering config against idp-jwt", zap.Error(err))
			return nil, nil, nil, err
		}
		userAccountName, permissions, limits, roleBindingTokenMaxExpiry, err := cfgForRequest.lookupUserAccount(reqClaims.toMap())
		if err != nil {
			zap.L().Error("error looking up user account", zap.Error(err))
			return nil, nil, nil, err
		}
		userAccountInfo, err := config.lookupAccountInfo(userAccountName)
		if err != nil {
			zap.L().Error("error looking up account-info", zap.Error(err))
			return nil, nil, nil, err
		}

		if ctx.Options.LogSensitive {
			zap.L().Debug("userAccountInfo", zap.Any("info", userAccountInfo))
		}

		// setup claims for user's nats-jwt
		claims := jwt.NewUserClaims(request.UserNkey)
		claims.Audience = userAccountName
		claims.Name = request.ConnectOptions.Username
		claims.IssuerAccount = userAccountInfo.PublicKey

		claims.Expires = calculateExpiration(
			cfgForRequest,    // NatsJwt ExpiryBounds and RBAC TokenMaxExpiry
			reqClaims.Expiry, // IDP Max Expiry
			&matchedVerifier.config.ValidationSpec.TokenExpiryBounds, // IDP ValidationSpec
			&roleBindingTokenMaxExpiry,                               // RoleBinding TokenMaxExpiry
		)
		claims.Permissions = *permissions
		claims.Limits = *limits
		claims.Tags.Add(fmt.Sprintf("email: %s, name: %s, idp: %s, expires: %s",
			reqClaims.Email,
			reqClaims.Name,
			matchedVerifier.config.Description,
			time.Unix(claims.Expires, 0).Format(time.RFC3339)))

		// Determine the type of signing key used
		signingKeyInfo, err := determineSigningKeyType(claims, userAccountInfo.SigningNKey.KeyPair, userAccountInfo)
		if err != nil {
			zap.L().Warn("failed to determine signing key type for audit event", zap.Error(err))
		}

		// Publish user creation event with detailed information
		userEvent := map[string]interface{}{
			"account":          userAccountName,
			"account_pub_nkey": userAccountInfo.PublicKey,
			"user_pub_nkey":    request.UserNkey,
			"username":         request.ConnectOptions.Username,
			"email":            reqClaims.Email,
			"name":             reqClaims.Name,
			"idp":              matchedVerifier.config.Description,
			"created_at":       time.Now().Format(time.RFC3339),
			"expires_at":       time.Unix(claims.Expires, 0).Format(time.RFC3339),
			"permissions":      permissions,
			"limits":           limits,
			"signing_account":  config.Service.Account.Name,
		}

		// Add signing key information if available
		if signingKeyInfo != nil {
			userEvent["signing_key_type"] = signingKeyInfo.Type
			userEvent["signing_key_pub_nkey"] = signingKeyInfo.PublicKey
		}

		eventJSON, err := json.Marshal(userEvent)
		if err != nil {
			zap.L().Warn("failed to marshal user creation event", zap.Error(err))
		} else {
			err = nc.Publish(fmt.Sprintf(auditEventSubject, userAccountName, request.UserNkey), eventJSON)
			if err != nil {
				zap.L().Warn("failed to publish user creation event", zap.Error(err))
			}
		}

		return claims, userAccountInfo.SigningNKey.KeyPair, userAccountInfo, nil
	})

	zap.L().Info("starting service", zap.String("version", config.Service.Version))

	_, err = micro.AddService(nc, micro.Config{
		Name:        config.Service.Name,
		Version:     config.Service.Version,
		Description: config.Service.Description,
		Endpoint: &micro.EndpointConfig{
			Subject: "$SYS.REQ.USER.AUTH",
			Handler: auth,
		},
	})
	if err != nil {
		return err
	}

	zap.L().Info("listening for auth requests", zap.String("subject", "$SYS.REQ.USER.AUTH"), zap.String("addr", nc.ConnectedAddr()))

	// Block and wait for interrupt signal
	internal.WaitForInterrupt()

	zap.L().Info("exiting")
	return nil
}

func calculateExpiration(cfg *Config, idpProvidedExpiry int64, idpValidationExpiry *DurationBounds, roleBindingTokenMaxExpiry *Duration) int64 {
	// Token expiration is calculated from the following sources, in order of precedence:
	// 1. IDP ValidationSpec. This is the expiration time set by the IDP.
	// 2. (Optional) IDP ValidationSpec.TokenExpiryBounds. This is the outer bounds that can be set per IDP.
	// 3. (Optional) RoleBinding TokenMaxExpiry. This is the expiration time set by the RoleBinding.
	//    Overrides RBAC TokenMaxExpiry. Both up and down to the bounds set by NatsJwt.TokenExpiryBounds.
	// 4. (Optional) RBAC TokenMaxExpiry. Default expiration time set by the RBAC as the Max expiration time for a token.
	// 5. NatsJwt.TokenExpiryBounds is the outer bounds that can be set in the config.

	now := time.Now()

	// 1. Start with IDP provided expiry
	expiry := idpProvidedExpiry

	// TODO: Is it allowed to have a token that is higher than the IDP provided max expiry?

	// 2. Apply idpValidation bounds
	if idpValidationExpiry != nil {
		if idpValidationExpiry.Min.Duration > 0 {
			if expiry < now.Add(idpValidationExpiry.Min.Duration).Unix() {
				expiry = now.Add(idpValidationExpiry.Min.Duration).Unix()
			}
		}
		if idpValidationExpiry.Max.Duration > 0 {
			if expiry > now.Add(idpValidationExpiry.Max.Duration).Unix() {
				expiry = now.Add(idpValidationExpiry.Max.Duration).Unix()
			}
		}
	}

	// 3. Apply role binding expiry if set
	if roleBindingTokenMaxExpiry != nil && roleBindingTokenMaxExpiry.Duration > 0 {
		expiry = now.Add(roleBindingTokenMaxExpiry.Duration).Unix()
	} else if cfg.Rbac.TokenMaxExpiry.Duration > 0 {
		// 4. Apply RBAC bounds
		if expiry > now.Add(cfg.Rbac.TokenMaxExpiry.Duration).Unix() {
			expiry = now.Add(cfg.Rbac.TokenMaxExpiry.Duration).Unix()
		}
	}

	// Make sure that the expiry is within the bounds
	if expiry < now.Add(cfg.NATS.TokenExpiryBounds.Min.Duration).Unix() {
		expiry = now.Add(cfg.NATS.TokenExpiryBounds.Min.Duration).Unix()
	}
	if expiry > now.Add(cfg.NATS.TokenExpiryBounds.Max.Duration).Unix() {
		expiry = now.Add(cfg.NATS.TokenExpiryBounds.Max.Duration).Unix()
	}

	return expiry
}
