package server

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestIdpJwtClaims_Permissions(t *testing.T) {
	tests := []struct {
		name               string
		claims             map[string]interface{}
		requiredPermission string
		expectedResult     bool
	}{
		{
			name: "permissions in array of strings",
			claims: map[string]interface{}{
				"custom:permissions": []string{"read", "write", "admin"},
			},
			requiredPermission: "write",
			expectedResult:     true,
		},
		{
			name: "permissions in array of interfaces",
			claims: map[string]interface{}{
				"scopes": []interface{}{"read", "write", "admin"},
			},
			requiredPermission: "admin",
			expectedResult:     true,
		},
		{
			name: "single permission as string",
			claims: map[string]interface{}{
				"permission": "admin",
			},
			requiredPermission: "admin",
			expectedResult:     true,
		},
		{
			name: "permissions in custom format",
			claims: map[string]interface{}{
				"https://example.com/claims/permissions": []string{"read", "write", "admin"},
			},
			requiredPermission: "write",
			expectedResult:     true,
		},
		{
			name: "permission not found",
			claims: map[string]interface{}{
				"custom:permissions": []string{"read", "write"},
			},
			requiredPermission: "admin",
			expectedResult:     false,
		},
		{
			name: "empty permissions",
			claims: map[string]interface{}{
				"custom:permissions": []string{},
			},
			requiredPermission: "read",
			expectedResult:     false,
		},
		{
			name: "permissions in different formats",
			claims: map[string]interface{}{
				"permission1": "read",
				"permission2": []string{"write", "admin"},
				"permission3": []interface{}{"delete", "update"},
			},
			requiredPermission: "admin",
			expectedResult:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			claims := &IdpJwtClaims{}
			claims.fromMap(tt.claims, nil)
			result := claims.hasPermission(tt.requiredPermission)
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}

func TestIdpJwtClaims_ValidateAudience(t *testing.T) {
	tests := []struct {
		name          string
		claims        map[string]interface{}
		expectedAud   []string
		shouldSucceed bool
	}{
		{
			name: "single audience match",
			claims: map[string]interface{}{
				"aud": "test-audience",
			},
			expectedAud:   []string{"test-audience"},
			shouldSucceed: true,
		},
		{
			name: "multiple audiences match",
			claims: map[string]interface{}{
				"aud": []interface{}{"aud1", "aud2", "aud3"},
			},
			expectedAud:   []string{"aud2"},
			shouldSucceed: true,
		},
		{
			name: "no audience match",
			claims: map[string]interface{}{
				"aud": []interface{}{"aud1", "aud2"},
			},
			expectedAud:   []string{"aud3"},
			shouldSucceed: false,
		},
		{
			name: "empty audience",
			claims: map[string]interface{}{
				"aud": "",
			},
			expectedAud:   []string{"test-audience"},
			shouldSucceed: false,
		},
		{
			name: "audience as interface array",
			claims: map[string]interface{}{
				"aud": []interface{}{"aud1", "aud2"},
			},
			expectedAud:   []string{"aud1", "aud2"},
			shouldSucceed: true,
		},
		{
			name: "audience as string array",
			claims: map[string]interface{}{
				"aud": []string{"aud1", "aud2"},
			},
			expectedAud:   []string{"aud1", "aud2"},
			shouldSucceed: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			claims := &IdpJwtClaims{}
			claims.fromMap(tt.claims, nil)
			err := claims.validateAudience(tt.expectedAud)
			if tt.shouldSucceed {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

func TestIdpJwtClaims_ValidateExpiryBounds(t *testing.T) {
	now := time.Now().Unix()
	tests := []struct {
		name          string
		claims        map[string]interface{}
		bounds        DurationBounds
		shouldSucceed bool
	}{
		{
			name: "valid expiry within bounds",
			claims: map[string]interface{}{
				"exp": now + 3600, // 1 hour from now
			},
			bounds: DurationBounds{
				Min: Duration{Duration: 30 * time.Minute},
				Max: Duration{Duration: 2 * time.Hour},
			},
			shouldSucceed: true,
		},
		{
			name: "expiry too short",
			claims: map[string]interface{}{
				"exp": now + 900, // 15 minutes from now
			},
			bounds: DurationBounds{
				Min: Duration{Duration: 30 * time.Minute},
				Max: Duration{Duration: 2 * time.Hour},
			},
			shouldSucceed: false,
		},
		{
			name: "expiry too long",
			claims: map[string]interface{}{
				"exp": now + 7200, // 2 hours from now
			},
			bounds: DurationBounds{
				Min: Duration{Duration: 30 * time.Minute},
				Max: Duration{Duration: 1 * time.Hour},
			},
			shouldSucceed: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			claims := &IdpJwtClaims{}
			claims.fromMap(tt.claims, nil)
			err := claims.validateExpiryBounds(tt.bounds)
			if tt.shouldSucceed {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

func TestIdpJwtClaims_Exists(t *testing.T) {
	tests := []struct {
		name           string
		claims         map[string]interface{}
		requiredClaims []string
		shouldSucceed  bool
	}{
		{
			name: "all required claims exist",
			claims: map[string]interface{}{
				"name":  "John Doe",
				"email": "john@example.com",
				"sub":   "123",
			},
			requiredClaims: []string{"name", "email", "sub"},
			shouldSucceed:  true,
		},
		{
			name: "missing required claim",
			claims: map[string]interface{}{
				"name":  "John Doe",
				"email": "john@example.com",
			},
			requiredClaims: []string{"name", "email", "sub"},
			shouldSucceed:  false,
		},
		{
			name: "empty required claims",
			claims: map[string]interface{}{
				"name": "John Doe",
			},
			requiredClaims: []string{},
			shouldSucceed:  true,
		},
		{
			name: "custom claims exist",
			claims: map[string]interface{}{
				"custom:claim1": "value1",
				"custom:claim2": "value2",
			},
			requiredClaims: []string{"custom:claim1", "custom:claim2"},
			shouldSucceed:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			claims := &IdpJwtClaims{}
			claims.fromMap(tt.claims, nil)
			err := claims.exists(tt.requiredClaims)
			if tt.shouldSucceed {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

func TestIdpJwtClaims_CustomClaims(t *testing.T) {
	tests := []struct {
		name               string
		claims             map[string]interface{}
		requiredClaims     map[string]string
		requiredPermission string
		shouldSucceed      bool
		checkClaims        bool
		customMapping      map[string]string
		expectedUnmapped   []string // Names of claims that should retain their original names
	}{
		{
			name: "valid custom claims with permission",
			claims: map[string]interface{}{
				"email":     "first.last@example.com",
				"client_id": "UBYLQLVSLUMX5OPEDWOT7WFYHXV55TTUTKPDLNB7JWVHVN4YXGLVGYTN",
				"https://example.com/claims/permissions": []string{
					"nats:core:account:test2:non-prod",
					"other:permission",
				},
			},
			requiredClaims: map[string]string{
				"email":     "first.last@example.com",
				"client_id": "UBYLQLVSLUMX5OPEDWOT7WFYHXV55TTUTKPDLNB7JWVHVN4YXGLVGYTN",
			},
			requiredPermission: "nats:core:account:test2:non-prod",
			shouldSucceed:      true,
			checkClaims:        true,
		},
		{
			name: "valid claims but missing permission",
			claims: map[string]interface{}{
				"email":     "first.last@example.com",
				"client_id": "UBYLQLVSLUMX5OPEDWOT7WFYHXV55TTUTKPDLNB7JWVHVN4YXGLVGYTN",
				"https://example.com/claims/permissions": []string{
					"other:permission",
				},
			},
			requiredClaims: map[string]string{
				"email":     "first.last@example.com",
				"client_id": "UBYLQLVSLUMX5OPEDWOT7WFYHXV55TTUTKPDLNB7JWVHVN4YXGLVGYTN",
			},
			requiredPermission: "nats:core:account:test2:non-prod",
			shouldSucceed:      false,
			checkClaims:        true,
		},
		{
			name: "valid permission but wrong claims",
			claims: map[string]interface{}{
				"email":     "wrong@email.com",
				"client_id": "wrong-client-id",
				"https://example.com/claims/permissions": []string{
					"nats:core:account:test2:non-prod",
				},
			},
			requiredClaims: map[string]string{
				"email":     "first.last@example.com",
				"client_id": "UBYLQLVSLUMX5OPEDWOT7WFYHXV55TTUTKPDLNB7JWVHVN4YXGLVGYTN",
			},
			requiredPermission: "nats:core:account:test2:non-prod",
			shouldSucceed:      true,
			checkClaims:        false,
		},
		{
			name: "permissions in different format",
			claims: map[string]interface{}{
				"email":                                  "first.last@example.com",
				"client_id":                              "UBYLQLVSLUMX5OPEDWOT7WFYHXV55TTUTKPDLNB7JWVHVN4YXGLVGYTN",
				"https://example.com/claims/permissions": "nats:core:account:test2:non-prod",
			},
			requiredClaims: map[string]string{
				"email":     "first.last@example.com",
				"client_id": "UBYLQLVSLUMX5OPEDWOT7WFYHXV55TTUTKPDLNB7JWVHVN4YXGLVGYTN",
			},
			requiredPermission: "nats:core:account:test2:non-prod",
			shouldSucceed:      true,
			checkClaims:        true,
		},
		{
			name: "permissions in custom claim name",
			claims: map[string]interface{}{
				"email":     "first.last@example.com",
				"client_id": "UBYLQLVSLUMX5OPEDWOT7WFYHXV55TTUTKPDLNB7JWVHVN4YXGLVGYTN",
				"custom:permissions": []string{
					"nats:core:account:test2:non-prod",
				},
			},
			requiredClaims: map[string]string{
				"email":     "first.last@example.com",
				"client_id": "UBYLQLVSLUMX5OPEDWOT7WFYHXV55TTUTKPDLNB7JWVHVN4YXGLVGYTN",
			},
			requiredPermission: "nats:core:account:test2:non-prod",
			shouldSucceed:      true,
			checkClaims:        true,
		},
		{
			name: "claims with roles and groups",
			claims: map[string]any{
				"email":     "first.last@example.com",
				"client_id": "UBYLQLVSLUMX5OPEDWOT7WFYHXV55TTUTKPDLNB7JWVHVN4YXGLVGYTN",
				"https://example.com/claims/roles": []string{
					"user",
					"nats:core:account:test2:non-prod",
				},
				"https://example.com/claims/groups": []string{
					"team-X",
					"app-stage-admin",
				},
				"https://example.com/claims/permissions": []string{
					"nats:core:account:test2",
				},
			},
			requiredClaims: map[string]string{
				"email":     "first.last@example.com",
				"client_id": "UBYLQLVSLUMX5OPEDWOT7WFYHXV55TTUTKPDLNB7JWVHVN4YXGLVGYTN",
			},
			requiredPermission: "nats:core:account:test2",
			shouldSucceed:      true,
			checkClaims:        true,
		},
		{
			name: "custom style claims with roles and groups as interfaces",
			claims: map[string]any{
				"email":                                  "test@example.com",
				"client_id":                              "test-client",
				"https://example.com/claims/roles":       []any{"role1", "role2"},
				"https://example.com/claims/groups":      []any{"group1", "group2"},
				"https://example.com/claims/permissions": []any{"permission1", "permission2"},
			},
			requiredClaims: map[string]string{
				"email":     "test@example.com",
				"client_id": "test-client",
			},
			requiredPermission: "permission1",
			shouldSucceed:      true,
			checkClaims:        true,
		},
		{
			name: "unmapped claims retain original names",
			claims: map[string]interface{}{
				"email":                              "test@example.com",
				"client_id":                          "test-client",
				"https://example.com/claims/custom1": "value1",
				"https://example.com/claims/custom2": "value2",
			},
			requiredClaims: map[string]string{
				"email":     "test@example.com",
				"client_id": "test-client",
			},
			requiredPermission: "some:permission",
			shouldSucceed:      false,
			checkClaims:        true,
			customMapping: map[string]string{
				"https://example.com/claims/roles": "roles",
			},
			expectedUnmapped: []string{
				"https://example.com/claims/custom1",
				"https://example.com/claims/custom2",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			claims := &IdpJwtClaims{}
			claims.fromMap(tt.claims, tt.customMapping)

			// Check required claims if needed
			if tt.checkClaims {
				for claim, value := range tt.requiredClaims {
					claimsMap := claims.toMap()
					actualValue, exists := claimsMap[claim]
					assert.True(t, exists, "Claim %s should exist", claim)
					assert.Equal(t, value, actualValue, "Claim %s should have value %s", claim, value)
				}
			}

			// Check permission
			hasPermission := claims.hasPermission(tt.requiredPermission)
			assert.Equal(t, tt.shouldSucceed, hasPermission, "Permission check should match expected result")

			if tt.checkClaims {
				// Verify that custom claims are properly stored
				claimsMap := claims.toMap()
				for k, v := range tt.claims {
					if k != "email" && k != "client_id" {
						assert.Contains(t, claimsMap, k, "Custom claim %s should exist", k)
						assert.Equal(t, v, claimsMap[k], "Custom claim %s should have correct value", k)
					}
				}

				// Verify that unmapped claims retain their original names
				if len(tt.expectedUnmapped) > 0 {
					claimsMap := claims.toMap()
					for _, claimName := range tt.expectedUnmapped {
						assert.Contains(t, claimsMap, claimName, "Unmapped claim %s should retain original name", claimName)
						assert.Equal(t, tt.claims[claimName], claimsMap[claimName], "Unmapped claim %s should retain original value", claimName)
					}
				}
			}
		})
	}
}

func (j *IdpJwtClaims) hasPermission(requiredPermission string) bool {
	// Try to find permissions in custom claims
	for _, value := range j.CustomClaims {
		switch v := value.(type) {
		case []interface{}:
			for _, p := range v {
				if permission, ok := p.(string); ok && permission == requiredPermission {
					return true
				}
			}
		case []string:
			for _, permission := range v {
				if permission == requiredPermission {
					return true
				}
			}
		case string:
			if v == requiredPermission {
				return true
			}
		}
	}
	return false
}
