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

type UserInfoProvider interface {
	GetUserInfo(ctx context.Context, token string) (map[string]interface{}, error)
}

type IdpAndJwtVerifier struct {
	verifier *IdpJwtVerifier
	config   *Idp
}

func NewIdpVerifiers(ctx *Context, config *Config) ([]IdpAndJwtVerifier, error) {
	idpVerifiers := make([]IdpAndJwtVerifier, 0, len(config.Idp))
	for i := range config.Idp {
		idp := &config.Idp[i] // Use a pointer to the IDP config
		idpVerifier, err := NewJwtVerifier(ctx, idp.ClientID, idp.IssuerURL)
		if err != nil {
			if idp.IgnoreSetupError {
				log.Warn().Err(err).Str("issuer_url", idp.IssuerURL).Str("client_id", idp.ClientID).Msg("Failed to setup IDP verifier, ignoring due to config")
				continue // Skip this IDP and continue with the next one
			}

			log.Error().Err(err).Str("issuer_url", idp.IssuerURL).Str("client_id", idp.ClientID).Msg("Failed to setup IDP verifier, halting startup")
			return nil, fmt.Errorf("failed to setup verifier for IDP %s (%s): %w", idp.Description, idp.IssuerURL, err)
		}
		idpVerifiers = append(idpVerifiers, IdpAndJwtVerifier{idpVerifier, idp}) // Pass the pointer to the config
	}
	return idpVerifiers, nil
}

func runVerification(jwtToken string, items []IdpAndJwtVerifier) (*IdpJwtClaims, *IdpAndJwtVerifier, error) {
	for _, item := range items {
		if item.verifier.ctx.Options.LogSensitive {
			log.Debug().Msgf("verifying jwt against spec. jwt=[%s], spec=[%v]", jwtToken, item.config.ValidationSpec)
		}
		reqClaims, idToken, err := item.verifier.verifyJWT(jwtToken, item.config.CustomMapping)
		if err != nil {
			var expiredErr *oidc.TokenExpiredError
			if errors.As(err, &expiredErr) {
				log.Debug().Msgf("error verifying idp-jwt, %s. Token expired at %v", item.config.Description, expiredErr.Expiry)
				continue
			}
			log.Trace().Msgf("error verifying idp-jwt, %s. Trying next idp...", item.config.Description)
			continue
		}

		err = item.verifier.validateAgainstSpec(reqClaims, item.config.ValidationSpec)
		if err != nil {
			log.Trace().Err(err).Msg("failed checks in idp validation")
			continue
		}

		// Store the idToken for use in fetching user info
		if idToken != nil {
			item.verifier.IDToken = idToken
		}

		return reqClaims, &item, nil
	}

	return nil, nil, errors.New("no idp verifier found for jwtToken")
}

type IdpJwtVerifier struct {
	ctx *Context
	*oidc.IDTokenVerifier
	provider         *oidc.Provider
	issuerURL        string
	MaxTokenLifetime time.Duration
	ClockSkew        time.Duration
	IDToken          *oidc.IDToken
}

func NewJwtVerifier(ctx *Context, clientID string, issuerURL string) (*IdpJwtVerifier, error) {
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
		ctx:             ctx,
		IDTokenVerifier: provider.Verifier(&oidc.Config{ClientID: clientID}),
		provider:        provider,
		issuerURL:       issuerURL,
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
		_ = 1
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

func (v *IdpJwtVerifier) GetUserInfo(ctx context.Context, accessToken string) (map[string]interface{}, error) {
	// Parse and verify the ID token
	if accessToken == "" {
		return nil, fmt.Errorf("no access token found in claim sources")
	}

	// Verify the access token matches the hash in the ID token
	if err := v.IDToken.VerifyAccessToken(accessToken); err != nil {
		return nil, fmt.Errorf("access token verification failed: %w", err)
	}

	// Use the verified access token to fetch user info
	userInfo, err := v.provider.UserInfo(ctx, oauth2.StaticTokenSource(&oauth2.Token{
		AccessToken: accessToken,
	}))
	if err != nil {
		return nil, fmt.Errorf("failed to fetch user info: %w", err)
	}

	userInfoClaims := make(map[string]interface{})
	if err := userInfo.Claims(&userInfoClaims); err != nil {
		return nil, fmt.Errorf("failed to parse user info claims: %w", err)
	}

	return userInfoClaims, nil
}
