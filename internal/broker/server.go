package server

import (
	internal "github.com/jr200/nats-iam-broker/internal"
	"github.com/nats-io/jwt/v2"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/micro"
	"github.com/nats-io/nkeys"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func Start(configFiles []string, serverOpts *ServerOptions) error {
	ctx := NewServerContext(serverOpts)

	config, err := readConfigFiles(configFiles, make(map[string]interface{}))
	if err != nil {
		log.Err(err).Msg("bad configuration")
		return err
	}

	log.Logger.UpdateContext(func(c zerolog.Context) zerolog.Context {
		return c.Str("name", config.Service.Name)
	})

	// Connect to NATS
	natsOpts := config.natsOptions()
	log.Info().Msgf("connecting to %s", config.NATS.URL)
	nc, err := nats.Connect(config.NATS.URL, natsOpts...)
	if err != nil {
		return err
	}
	defer nc.Drain()

	idpVerifiers, err := NewIdpVerifiers(ctx, config)
	if err != nil {
		return err
	}

	auth := NewAuthService(ctx, config.Service.Account.SigningNKey.KeyPair, config.serviceEncryptionXkey(), func(request *jwt.AuthorizationRequestClaims) (*jwt.UserClaims, nkeys.KeyPair, error) {
		log.Trace().Msgf("NewAuthService (request): %s", request)

		idpJwt := request.ConnectOptions.Password

		reqClaims, err := runVerification(idpJwt, idpVerifiers)
		if err != nil {
			return nil, nil, err
		}

		if ctx.Options.LogSensitive {
			log.Debug().Msgf("reqClaims: %v", reqClaims.toMap())
		}

		cfgForRequest, err := readConfigFiles(configFiles, reqClaims.toMap())
		if err != nil {
			log.Error().Err(err).Msg("error rendering config against idp-jwt")
			return nil, nil, err
		}

		userAccountName, permissions, limits := cfgForRequest.lookupUserAccount(reqClaims.toMap())
		userAccountInfo, err := config.lookupAccountInfo(userAccountName)
		if err != nil {
			return nil, nil, err
		}

		if ctx.Options.LogSensitive {
			log.Debug().Msgf("userAccountInfo: %v", userAccountInfo)
		}

		// setup claims for user's nats-jwt
		claims := jwt.NewUserClaims(request.UserNkey)
		claims.Name = request.ConnectOptions.Username
		claims.IssuerAccount = userAccountInfo.PublicKey
		claims.Expires = reqClaims.Expiry
		claims.Permissions = *permissions
		claims.Limits = *limits

		return claims, userAccountInfo.SigningNKey.KeyPair, nil
	})

	log.Info().Msgf("Starting service %s v%s", config.Service.Name, config.Service.Version)

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

	log.Info().Msgf("Listening to $SYS.REQ.USER.AUTH on %s", nc.ConnectedAddr())

	// Block and wait for interrupt signal
	internal.WaitForInterrupt()

	log.Info().Msg("Exiting...")
	return nil
}
