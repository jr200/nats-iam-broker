package broker

import (
	"fmt"
	"strings"
	"time"

	internal "github.com/jr200/nats-iam-broker/internal"
	"github.com/jr200/nats-iam-broker/internal/metrics"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/micro"
	"go.uber.org/zap"
)

func Start(configFiles []string, serverOpts *Options) error {
	ctx := NewServerContext(serverOpts)

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

	// Start metrics server if enabled
	var m *metrics.Metrics
	if serverOpts.MetricsEnabled {
		m = metrics.New()
		metricsServer := metrics.NewServer(serverOpts.MetricsPort)
		metricsServer.Start()
		defer metricsServer.Stop()
	}

	// Connect to NATS
	natsOpts := config.natsOptions()
	natsOpts = append(natsOpts,
		nats.Name(config.Service.Name),
		nats.MaxReconnects(-1),
		nats.DisconnectErrHandler(func(_ *nats.Conn, err error) {
			if err != nil {
				zap.L().Error("NATS disconnected", zap.Error(err))
			} else {
				zap.L().Warn("NATS disconnected (graceful)")
			}
		}),
		nats.ReconnectHandler(func(nc *nats.Conn) {
			zap.L().Info("NATS reconnected", zap.String("addr", nc.ConnectedAddr()))
		}),
		nats.ClosedHandler(func(_ *nats.Conn) {
			zap.L().Info("NATS connection closed")
		}),
		nats.ErrorHandler(func(_ *nats.Conn, sub *nats.Subscription, err error) {
			if sub != nil {
				zap.L().Error("NATS async error", zap.String("subject", sub.Subject), zap.Error(err))
			} else {
				zap.L().Error("NATS async error", zap.Error(err))
			}
		}),
	)

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

	// Build initial live state and config watcher
	initial := &liveState{
		config:        config,
		configManager: configManager,
		idpVerifiers:  idpVerifiers,
		auditSubject:  auditEventSubject,
	}
	watcher := NewConfigWatcher(ctx, configFiles, initial)

	if serverOpts.WatchConfig {
		if err := watcher.Start(); err != nil {
			zap.L().Warn("failed to start config watcher, continuing without hot-reload", zap.Error(err))
		} else {
			defer watcher.Stop()
		}
	}

	authCallback := newAuthCallbackWithWatcher(ctx, m, nc, watcher)
	auth := NewAuthService(ctx, config.Service.Account.SigningNKey.KeyPair, config.serviceEncryptionXkey(), authCallback, m)

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
