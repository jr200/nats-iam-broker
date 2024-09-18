package server

import (
	"context"
	"errors"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/rs/zerolog/log"
)

type IdpAndJwtVerifier struct {
	verifier *IdpJwtVerifier
	config   *Idp
}

func NewIdpVerifiers(config *Config) ([]IdpAndJwtVerifier, error) {
	idpVerifiers := make([]IdpAndJwtVerifier, 0, len(config.Idp))
	for _, idp := range config.Idp {
		idpVerifier, err := NewJwtVerifier(context.Background(), idp.ClientID, idp.IssuerURL)
		if err != nil {
			return nil, err
		}
		idpVerifiers = append(idpVerifiers, IdpAndJwtVerifier{idpVerifier, &idp})
	}
	return idpVerifiers, nil
}

func runVerification(jwtToken string, items []IdpAndJwtVerifier) (*IdpJwtClaims, error) {
	for _, item := range items {
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
	*oidc.IDTokenVerifier
	MaxTokenLifetime time.Duration
	ClockSkew        time.Duration
}

func NewJwtVerifier(ctx context.Context, clientID string, issuerUrl string) (*IdpJwtVerifier, error) {
	provider, err := oidc.NewProvider(ctx, issuerUrl)
	if err != nil {
		log.Err(err)
		return nil, err
	}

	log.Trace().Msgf("NewJwtVerifier (config-params) clientId=%s, issuerUrl=%s", clientID, issuerUrl)

	return &IdpJwtVerifier{provider.Verifier(&oidc.Config{ClientID: clientID}), time.Hour * 24, time.Minute * 5}, nil
}

// Verifies that the ID token was signed by google and is valid.
// Returns the claims embedded with the token
func (v *IdpJwtVerifier) verifyJWT(token string) (*IdpJwtClaims, error) {
	// log.Trace().Msgf("VerifyJWT %s", token)
	claims := &IdpJwtClaims{}

	idToken, err := v.Verify(context.Background(), token)
	if err != nil {
		return nil, err
	}

	if err := idToken.Claims(claims); err != nil {
		return nil, err
	}

	err = v.ValidateTimes(idToken.IssuedAt, idToken.Expiry)
	if err != nil {
		return nil, err
	}

	return claims, nil
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

	err := claims.exists(spec.Claims)
	if err != nil {
		return err
	}

	err = claims.validateAudience(spec.Audience)
	if err != nil {
		return err
	}

	err = claims.validateExpiryBounds(spec.Expiry)
	if err != nil {
		return err
	}

	return nil
}
