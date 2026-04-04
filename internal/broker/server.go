package broker

import (
	"context"
	"fmt"
	"strings"
	"time"

	internal "github.com/jr200/nats-iam-broker/internal"
	"github.com/jr200/nats-iam-broker/internal/logging"
	"github.com/jr200/nats-iam-broker/internal/metrics"
	"github.com/jr200/nats-iam-broker/internal/tracing"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/micro"
	"go.uber.org/zap"
)

// Start runs the broker, blocking until an OS interrupt signal is received.
func Start(configFiles []string, cliOpts *Options, cliFlags map[string]bool) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		internal.WaitForInterrupt()
		cancel()
	}()

	return StartWithContext(ctx, configFiles, cliOpts, cliFlags)
}

// StartWithContext runs the broker, blocking until the given context is cancelled.
func StartWithContext(ctx context.Context, configFiles []string, cliOpts *Options, cliFlags map[string]bool) error {
	configManager, err := NewConfigManager(configFiles)
	if err != nil {
		return fmt.Errorf("failed to initialize config manager: %v", err)
	}

	// Merge: defaults <- YAML <- explicit CLI flags
	serverOpts := MergeOptions(configManager.ServerOptions(), cliOpts, cliFlags)

	// Configure logging from merged options (YAML + CLI overrides)
	logging.Setup(serverOpts.LogLevel, serverOpts.LogFormat == "human")

	srvCtx := NewServerContext(serverOpts)

	config, err := configManager.GetConfig(make(map[string]interface{}))
	if err != nil {
		zap.L().Error("bad configuration", zap.Error(err))
		return err
	}

	zap.ReplaceGlobals(zap.L().Named(config.Service.Name))

	// Log available RBAC account names
	accountNames := make([]string, len(config.Rbac.Accounts))
	for i, acct := range config.Rbac.Accounts {
		accountNames[i] = acct.Name
	}
	zap.L().Info("available RBAC accounts", zap.Strings("accounts", accountNames))

	// Start tracing (OTLP gRPC if OTEL_EXPORTER_OTLP_ENDPOINT is set,
	// console if OTEL_TRACES_EXPORTER=console, otherwise no-op).
	shutdownTracing, err := tracing.Setup(ctx, config.Service.Name, config.Service.Version)
	if err != nil {
		zap.L().Warn("failed to initialise OTel tracing, continuing without tracing", zap.Error(err))
	} else {
		defer func() {
			// Use a fresh context with timeout — the parent ctx is already cancelled
			// by the time defers run, and we need a live context to flush pending spans.
			//nolint:mnd // matches metrics.shutdownTimeout
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := shutdownTracing(shutdownCtx); err != nil {
				zap.L().Warn("error shutting down OTel tracing", zap.Error(err))
			}
		}()
	}

	// Start metrics server if enabled
	var m *metrics.Metrics
	var health *metrics.HealthChecker
	if serverOpts.MetricsEnabled {
		m = metrics.New()
		health = metrics.NewHealthChecker()
		metricsServer := metrics.NewServer(serverOpts.MetricsPort, health)
		metricsServer.Start()
		defer metricsServer.Stop()
	}

	// Connect to NATS
	natsOpts := config.natsOptions()
	natsOpts = append(natsOpts,
		nats.Name(config.Service.Name),
		nats.MaxReconnects(-1),
		nats.DisconnectErrHandler(func(_ *nats.Conn, err error) {
			if health != nil {
				health.SetNATSConnected(false)
			}
			if err != nil {
				zap.L().Error("NATS disconnected", zap.Error(err))
			} else {
				zap.L().Warn("NATS disconnected (graceful)")
			}
		}),
		nats.ReconnectHandler(func(nc *nats.Conn) {
			if health != nil {
				health.SetNATSConnected(true)
			}
			zap.L().Info("NATS reconnected", zap.String("addr", nc.ConnectedAddr()))
		}),
		nats.ClosedHandler(func(_ *nats.Conn) {
			if health != nil {
				health.SetNATSConnected(false)
			}
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

	if health != nil {
		health.SetNATSConn(nc)
	}

	idpVerifiers, err := NewIdpVerifiers(srvCtx, config)
	if err != nil {
		return err
	}

	if health != nil {
		health.SetIDPVerifiersReady(len(idpVerifiers) > 0)
	}

	auditEventSubject := config.Service.Name + ".evt.audit.account.%s.user.%s.created"
	//nolint:mnd // 2 is the number of %s placeholders in auditEventSubject
	zap.L().Info("audit events configured", zap.String("subject_pattern", strings.Replace(auditEventSubject, "%s", "*", 2)))

	// Build initial live state and config watcher
	initial := &LiveState{
		config:        config,
		configManager: configManager,
		idpVerifiers:  idpVerifiers,
		auditSubject:  auditEventSubject,
	}
	watcher := NewConfigWatcher(srvCtx, configFiles, initial)

	if serverOpts.WatchConfig {
		if err := watcher.Start(); err != nil {
			zap.L().Warn("failed to start config watcher, continuing without hot-reload", zap.Error(err))
		} else {
			defer watcher.Stop()
		}
	}

	authCallback := newAuthCallbackWithWatcher(srvCtx, m, nc, watcher)
	auth := NewAuthService(srvCtx, config.Service.Account.SigningNKey.KeyPair, config.serviceEncryptionXkey(), authCallback, m)

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

	if health != nil {
		health.SetServiceRegistered(true)
	}

	zap.L().Info("listening for auth requests", zap.String("subject", "$SYS.REQ.USER.AUTH"), zap.String("addr", nc.ConnectedAddr()))

	// Block until context is cancelled
	<-ctx.Done()

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

	// The IDP-provided expiry is the absolute upper bound. Downstream overrides
	// (role bindings, RBAC) may shorten the token lifetime but never extend it
	// beyond what the IDP originally granted.
	idpCeiling := idpProvidedExpiry

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

	// 5. Make sure that the expiry is within the NATS token bounds
	if expiry < now.Add(cfg.NATS.TokenExpiryBounds.Min.Duration).Unix() {
		expiry = now.Add(cfg.NATS.TokenExpiryBounds.Min.Duration).Unix()
	}
	if expiry > now.Add(cfg.NATS.TokenExpiryBounds.Max.Duration).Unix() {
		expiry = now.Add(cfg.NATS.TokenExpiryBounds.Max.Duration).Unix()
	}

	// 6. Enforce the IDP-provided expiry as the absolute ceiling. No override
	// (role binding, RBAC, or NATS bounds) may extend the token beyond the
	// lifetime the IDP originally granted.
	if expiry > idpCeiling {
		expiry = idpCeiling
	}

	return expiry
}
