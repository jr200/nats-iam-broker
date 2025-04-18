package server

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/rs/zerolog/log"
	"golang.org/x/oauth2"
)

type IdpAndJwtVerifier struct {
	verifier *IdpJwtVerifier
	config   *Idp
}

func NewIdpVerifiers(ctx *ServerContext, config *Config) ([]IdpAndJwtVerifier, error) {
	idpVerifiers := make([]IdpAndJwtVerifier, 0, len(config.Idp))
	for _, idp := range config.Idp {
		idpVerifier, err := NewJwtVerifier(ctx, idp.ClientID, idp.IssuerURL)
		if err != nil {
			return nil, err
		}
		idpVerifiers = append(idpVerifiers, IdpAndJwtVerifier{idpVerifier, &idp})
	}
	return idpVerifiers, nil
}

func runVerification(jwtToken string, items []IdpAndJwtVerifier) (*IdpJwtClaims, error) {
	for _, item := range items {
		if item.verifier.ctx.Options.LogSensitive {
			log.Debug().Msgf("verifying jwt against spec. jwt=[%s], spec=[%v]", jwtToken, item.config.ValidationSpec)
		}

		reqClaims, err := item.verifier.verifyJWT(jwtToken)
		if err != nil {
			log.Trace().Err(err).Msg("error verifying idp-jwt")
			continue
		}

		err = item.verifier.validateAgainstSpec(reqClaims, item.config.ValidationSpec)
		if err != nil {
			log.Trace().Err(err).Msg("failed checks in idp validation")
			continue
		}

		return reqClaims, nil
	}

	return nil, errors.New("no idp verifier found for jwtToken")
}

type IdpJwtVerifier struct {
	ctx *ServerContext
	*oidc.IDTokenVerifier
	MaxTokenLifetime time.Duration
	ClockSkew        time.Duration
}

	const maxTokenLifetime = time.Hour * 24
	const clockSkew = time.Minute * 5

	provider, err := oidc.NewProvider(context.Background(), issuerURL)
	if err != nil {
		log.Err(err)
		return nil, err
	}

	if ctx.Options.LogSensitive {
		log.Trace().Msgf("NewJwtVerifier (config-params) clientId=%s, issuerUrl=%s", clientID, issuerURL)
	}

	return &IdpJwtVerifier{
		// TODO: take MaxTokenLifetime from config
		MaxTokenLifetime: maxTokenLifetime,
		ClockSkew:        clockSkew,
	}, nil
}

// Verifies that the ID token was signed by idp and is valid.
// Returns the claims embedded with the token
func (v *IdpJwtVerifier) verifyJWT(token string, customMapping map[string]string) (*IdpJwtClaims, *oidc.IDToken, error) {
	claims := &IdpJwtClaims{}

	if v.ctx.Options.LogSensitive {
		log.Trace().Msgf("VerifyJWT %s", token)
	}

	idToken, err := v.Verify(context.Background(), token)
	if err != nil {
		return nil, nil, err
	}

	// First get all claims as a map
	var rawClaims map[string]interface{}
	if err := idToken.Claims(&rawClaims); err != nil {
		return nil, nil, err
	}

	// Then convert to our claims struct
	if err := idToken.Claims(claims); err != nil {
		return nil, nil, err
	}

	// Store all claims in CustomClaims using the custom mapping
	claims.fromMap(rawClaims, customMapping)

	err = v.ValidateTimes(idToken.IssuedAt, idToken.Expiry)
	if err != nil {
		return nil, nil, err
	}

	return claims, idToken, nil
}

func (v *IdpJwtVerifier) ValidateTimes(issuedAt time.Time, expiry time.Time) error {
	if issuedAt.Unix() < 1 {
		return errors.New("missing 'issued at' time in token")
	}

	if expiry.Unix() < 1 {
		return errors.New("missing 'expiry' time in token")
	}

	now := time.Now()
	if expiry.Unix() > now.Unix()+int64(v.MaxTokenLifetime.Seconds()) {
		return errors.New("expiry too far in future")
	}

	skewedEarliest := issuedAt.Unix() - int64(v.ClockSkew.Seconds())
	skewedLatest := expiry.Unix() + int64(v.ClockSkew.Seconds())

	if now.Unix() < skewedEarliest {
		return errors.New("token used too early. check clock skew?")
	}

	if now.Unix() > skewedLatest {
		return errors.New("token used too late. check clock skew?")
	}

	return nil
}

func (v *IdpJwtVerifier) validateAgainstSpec(claims *IdpJwtClaims, spec IdpJwtValidationSpec) error {
	if spec.Claims != nil {
		err := claims.exists(spec.Claims)
		if err != nil {
			return err
		}
	}

	if spec.SkipAudienceValidation || spec.Audience == nil || len(spec.Audience) == 0 {
		// Skip audience validation if explicitly disabled or no audience configured
	} else {
		err := claims.validateAudience(spec.Audience)
		if err != nil {
			log.Error().Err(err).Msgf("failed audience check: %v", spec.Audience)
			return err
		}
	}

	if spec.TokenExpiryBounds.Min.Duration > 0 || spec.TokenExpiryBounds.Max.Duration > 0 {
		err := claims.validateExpiryBounds(spec.TokenExpiryBounds)
		if err != nil {
			return err
		}
	}

	return nil
}
