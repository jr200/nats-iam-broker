package server

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
)

// https://www.iana.org/assignments/jwt/jwt.xhtml#claims
type IdpJwtClaims struct {
	Name              string           `json:"name,omitempty"`
	GivenName         string           `json:"given_name,omitempty"`
	FamilyName        string           `json:"family_name,omitempty"`
	PreferredUsername string           `json:"preferred_username,omitempty"`
	Nickname          string           `json:"nickname,omitempty"`
	Gender            string           `json:"gender,omitempty"`
	ZoneInfo          string           `json:"zoneinfo,omitempty"`
	Locale            string           `json:"locale,omitempty"`
	ClientID          string           `json:"client_id,omitempty"`
	Groups            string           `json:"groups,omitempty"`
	Roles             string           `json:"roles,omitempty"`
	Email             string           `json:"email,omitempty"`
	EmailVerified     bool             `json:"email_verified,omitempty"`
	Picture           string           `json:"picture,omitempty"`
	Subject           string           `json:"sub,omitempty"`
	Audience          JwtClaimAudience `json:"aud,omitempty"`
	Expiry            int64            `json:"exp,omitempty"`
	IssuedAt          int64            `json:"iat,omitempty"`
	NotBeforeTime     int64            `json:"nbf,omitempty"`
	JwtId             string           `json:"jti,omitempty"`
	AccessTokenHash   string           `json:"at_hash,omitempty"`
}

type JwtClaimAudience []string

func (a *JwtClaimAudience) UnmarshalJSON(data []byte) error {
	var single string
	if err := json.Unmarshal(data, &single); err == nil {
		*a = JwtClaimAudience{single}
		return nil
	}

	var multi []string
	if err := json.Unmarshal(data, &multi); err == nil {
		*a = JwtClaimAudience(multi)
		return nil
	}

	return fmt.Errorf("aud field is not a valid string or array of strings")
}

func (j *IdpJwtClaims) toMap() map[string]interface{} {
	result := make(map[string]interface{})

	// Marshal the struct to JSON
	jsonBytes, err := json.Marshal(j)
	if err != nil {
		log.Err(err)
		return result
	}

	if err := json.Unmarshal(jsonBytes, &result); err != nil {
		log.Err(err)
		return result
	}

	return result
}

func (j *IdpJwtClaims) exists(expected []string) error {
	claimsMap := j.toMap()
	log.Trace().Msgf("idp claims: %v", claimsMap)
	for _, claimName := range expected {
		_, found := claimsMap[claimName]

		if !found {
			return fmt.Errorf("missing or empty claim '%s' in idp token", claimName)
		}
	}

	return nil
}

func (j *IdpJwtClaims) validateAudience(expected []string) error {
	for _, claimAud := range j.Audience {
		for _, expectedClaim := range expected {
			if claimAud == expectedClaim {
				return nil
			}
		}
	}

	return fmt.Errorf("idp 'aud' did not match expected. 0 == intersect(%v, %v)", j.Audience, expected)
}

func (j *IdpJwtClaims) validateExpiryBounds(bounds DurationBounds) error {
	now := time.Now().Unix()
	ttl := time.Duration((j.Expiry - now) * int64(time.Second))

	if ttl < bounds.Min.Duration {
		return fmt.Errorf("idp 'exp' (%v) too short. must have at least %v remaining", ttl, bounds.Min.Duration)
	}

	if ttl > bounds.Max.Duration {
		return fmt.Errorf("idp 'exp' (%v) too long. must expire within %v", ttl, bounds.Max.Duration)
	}

	return nil
}
