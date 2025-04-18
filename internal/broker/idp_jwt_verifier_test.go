package server

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIdpJwtVerifier_ValidateAgainstSpec(t *testing.T) {
	tests := []struct {
		name          string
		claims        *IdpJwtClaims
		spec          IdpJwtValidationSpec
		shouldSucceed bool
	}{
		{
			name: "valid claims with audience check",
			claims: &IdpJwtClaims{
				Audience: []string{"test-audience"},
			},
			spec: IdpJwtValidationSpec{
				Audience: []string{"test-audience"},
			},
			shouldSucceed: true,
		},
		{
			name: "invalid audience",
			claims: &IdpJwtClaims{
				Audience: []string{"wrong-audience"},
			},
			spec: IdpJwtValidationSpec{
				Audience: []string{"test-audience"},
			},
			shouldSucceed: false,
		},
		{
			name: "no audience check when not configured",
			claims: &IdpJwtClaims{
				Audience: []string{"any-audience"},
			},
			spec: IdpJwtValidationSpec{
				Audience: nil,
			},
			shouldSucceed: true,
		},
		{
			name: "no audience check when empty array",
			claims: &IdpJwtClaims{
				Audience: []string{"any-audience"},
			},
			spec: IdpJwtValidationSpec{
				Audience: []string{},
			},
			shouldSucceed: true,
		},
		{
			name: "multiple audiences with one match",
			claims: &IdpJwtClaims{
				Audience: []string{"aud1", "aud2", "aud3"},
			},
			spec: IdpJwtValidationSpec{
				Audience: []string{"aud2", "aud4"},
			},
			shouldSucceed: true,
		},
		{
			name: "multiple audiences with no match",
			claims: &IdpJwtClaims{
				Audience: []string{"aud1", "aud2", "aud3"},
			},
			spec: IdpJwtValidationSpec{
				Audience: []string{"aud4", "aud5"},
			},
			shouldSucceed: false,
		},
		{
			name: "skip audience validation when flag is set",
			claims: &IdpJwtClaims{
				Audience: []string{"wrong-audience"},
			},
			spec: IdpJwtValidationSpec{
				Audience:               []string{"test-audience"},
				SkipAudienceValidation: true,
			},
			shouldSucceed: true,
		},
		{
			name: "skip audience validation with empty audience",
			claims: &IdpJwtClaims{
				Audience: []string{"any-audience"},
			},
			spec: IdpJwtValidationSpec{
				Audience:               []string{},
				SkipAudienceValidation: true,
			},
			shouldSucceed: true,
		},
		{
			name: "skip audience validation with nil audience",
			claims: &IdpJwtClaims{
				Audience: []string{"any-audience"},
			},
			spec: IdpJwtValidationSpec{
				Audience:               nil,
				SkipAudienceValidation: true,
			},
			shouldSucceed: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			verifier := &IdpJwtVerifier{}
			err := verifier.validateAgainstSpec(tt.claims, tt.spec)
			if tt.shouldSucceed {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}
