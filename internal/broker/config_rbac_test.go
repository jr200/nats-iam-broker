package server

import (
	"testing"

	"github.com/nats-io/jwt/v2"
	"github.com/stretchr/testify/assert"
)

func TestLookupUserAccount_Strategies(t *testing.T) {
	// Define some basic roles used in tests
	roles := []Role{
		{Name: "role-a", Permissions: Permissions{Pub: jwt.Permission{Allow: []string{"a.>"}}}},
		{Name: "role-b", Permissions: Permissions{Pub: jwt.Permission{Allow: []string{"b.>"}}}},
		{Name: "role-c", Permissions: Permissions{Pub: jwt.Permission{Allow: []string{"c.>"}}}},
	}

	tests := []struct {
		name            string
		strategy        RoleBindingStrategy
		bindings        []RoleBinding
		context         map[string]interface{}
		expectedAccount string
		expectedRoles   []string // For verifying which binding was chosen
	}{
		// --- Strict Strategy Tests ---
		{
			name:     "Strict: Exact Match Found",
			strategy: StrategyStrict,
			bindings: []RoleBinding{
				{Account: "Acc1", Roles: []string{"role-a"}, Match: []Match{{Claim: "sub", Value: "user1"}}},                                // Does not match
				{Account: "Acc2", Roles: []string{"role-b"}, Match: []Match{{Claim: "sub", Value: "user2"}, {Claim: "aud", Value: "app1"}}}, // Matches exactly
				{Account: "Acc3", Roles: []string{"role-c"}, Match: []Match{{Claim: "sub", Value: "user2"}}},                                // Partial match, ignored in strict
			},
			context:         map[string]interface{}{"sub": "user2", "aud": "app1"},
			expectedAccount: "Acc2",
			expectedRoles:   []string{"role-b"},
		},
		{
			name:     "Strict: First Exact Match Wins",
			strategy: StrategyStrict,
			bindings: []RoleBinding{
				{Account: "Acc1", Roles: []string{"role-a"}, Match: []Match{{Claim: "sub", Value: "user1"}, {Claim: "aud", Value: "app1"}}}, // Matches
				{Account: "Acc2", Roles: []string{"role-b"}, Match: []Match{{Claim: "sub", Value: "user1"}, {Claim: "aud", Value: "app1"}}}, // Also matches, but later
			},
			context:         map[string]interface{}{"sub": "user1", "aud": "app1"},
			expectedAccount: "Acc1",
			expectedRoles:   []string{"role-a"},
		},
		{
			name:     "Strict: No Exact Match -> Corrected to Match Acc2",
			strategy: StrategyStrict,
			bindings: []RoleBinding{
				{Account: "Acc1", Roles: []string{"role-a"}, Match: []Match{{Claim: "sub", Value: "user1"}, {Claim: "aud", Value: "app1"}}}, // Requires app1, sub doesn't match
				{Account: "Acc2", Roles: []string{"role-b"}, Match: []Match{{Claim: "sub", Value: "user2"}}},                                // Only requires sub=user2, which matches
			},
			context:         map[string]interface{}{"sub": "user2", "aud": "app2"}, // aud=app2 is irrelevant for Acc2 matching
			expectedAccount: "Acc2",                                                // Expect Acc2 to match
			expectedRoles:   []string{"role-b"},                                    // Expect role-b
		},
		{
			name:     "Strict: Permission Match",
			strategy: StrategyStrict,
			bindings: []RoleBinding{
				{Account: "AccPerm", Roles: []string{"role-c"}, Match: []Match{{Permission: "perm:read"}, {Claim: "sub", Value: "tester"}}},
			},
			context:         map[string]interface{}{"sub": "tester", "permissions": []interface{}{"perm:read", "perm:write"}},
			expectedAccount: "AccPerm",
			expectedRoles:   []string{"role-c"},
		},
		{
			name:     "Strict: Permission Mismatch",
			strategy: StrategyStrict,
			bindings: []RoleBinding{
				{Account: "AccPerm", Roles: []string{"role-c"}, Match: []Match{{Permission: "perm:admin"}, {Claim: "sub", Value: "tester"}}},
			},
			context:         map[string]interface{}{"sub": "tester", "permissions": []interface{}{"perm:read", "perm:write"}},
			expectedAccount: "",
			expectedRoles:   nil,
		},

		// --- Best Match Strategy Tests ---
		{
			name:     "BestMatch: Most Matches Wins",
			strategy: StrategyBestMatch,
			bindings: []RoleBinding{
				{Account: "Acc1", Roles: []string{"role-a"}, Match: []Match{{Claim: "sub", Value: "user1"}}},                                // 1 match
				{Account: "Acc2", Roles: []string{"role-b"}, Match: []Match{{Claim: "sub", Value: "user1"}, {Claim: "aud", Value: "app1"}}}, // 2 matches - wins
				{Account: "Acc3", Roles: []string{"role-c"}, Match: []Match{{Claim: "aud", Value: "app1"}}},                                 // 1 match
			},
			context:         map[string]interface{}{"sub": "user1", "aud": "app1"},
			expectedAccount: "Acc2",
			expectedRoles:   []string{"role-b"},
		},
		{
			name:     "BestMatch: Tie in Matches, Specificity Wins",
			strategy: StrategyBestMatch,
			bindings: []RoleBinding{
				{Account: "Acc1", Roles: []string{"role-a"}, Match: []Match{{Claim: "sub", Value: "user1"}, {Claim: "aud", Value: "app2"}}},                                   // 1 match (sub), 2 criteria
				{Account: "Acc2", Roles: []string{"role-b"}, Match: []Match{{Claim: "sub", Value: "user1"}, {Claim: "aud", Value: "app1"}, {Claim: "group", Value: "admin"}}}, // 2 matches (sub, aud), 3 criteria - wins
				{Account: "Acc3", Roles: []string{"role-c"}, Match: []Match{{Claim: "sub", Value: "user1"}, {Claim: "aud", Value: "app1"}}},                                   // 2 matches (sub, aud), 2 criteria
			},
			context:         map[string]interface{}{"sub": "user1", "aud": "app1", "group": "admin"},
			expectedAccount: "Acc2",
			expectedRoles:   []string{"role-b"},
		},
		{
			name:     "BestMatch: Tie in Matches and Specificity, First Wins",
			strategy: StrategyBestMatch,
			bindings: []RoleBinding{
				{Account: "Acc1", Roles: []string{"role-a"}, Match: []Match{{Claim: "sub", Value: "user1"}, {Claim: "aud", Value: "app1"}}}, // 2 matches, 2 criteria - wins (first)
				{Account: "Acc2", Roles: []string{"role-b"}, Match: []Match{{Claim: "sub", Value: "user1"}, {Claim: "aud", Value: "app1"}}}, // 2 matches, 2 criteria
				{Account: "Acc3", Roles: []string{"role-c"}, Match: []Match{{Claim: "sub", Value: "user1"}}},                                // 1 match
			},
			context:         map[string]interface{}{"sub": "user1", "aud": "app1"},
			expectedAccount: "Acc1",
			expectedRoles:   []string{"role-a"},
		},
		{
			name:     "BestMatch: No Matches",
			strategy: StrategyBestMatch,
			bindings: []RoleBinding{
				{Account: "Acc1", Roles: []string{"role-a"}, Match: []Match{{Claim: "sub", Value: "user1"}}},
				{Account: "Acc2", Roles: []string{"role-b"}, Match: []Match{{Claim: "aud", Value: "app1"}}},
			},
			context:         map[string]interface{}{"sub": "user3", "aud": "app2"},
			expectedAccount: "",
			expectedRoles:   nil,
		},

		// --- Default Strategy Tests (should be best_match) ---
		{
			name:     "Default (BestMatch): Most Matches Wins",
			strategy: "", // Test the default behavior (will be set to BestMatch by UnmarshalYAML or default struct value)
			bindings: []RoleBinding{
				{Account: "Acc1", Roles: []string{"role-a"}, Match: []Match{{Claim: "sub", Value: "user1"}}},                                // 1 match
				{Account: "Acc2", Roles: []string{"role-b"}, Match: []Match{{Claim: "sub", Value: "user1"}, {Claim: "aud", Value: "app1"}}}, // 2 matches - wins
			},
			context:         map[string]interface{}{"sub": "user1", "aud": "app1"},
			expectedAccount: "Acc2",
			expectedRoles:   []string{"role-b"},
		},
		{
			name:     "Invalid Strategy (Defaults to BestMatch): Most Matches Wins",
			strategy: "unknown_strategy", // Test invalid value defaulting (handled by UnmarshalYAML or default struct value)
			bindings: []RoleBinding{
				{Account: "Acc1", Roles: []string{"role-a"}, Match: []Match{{Claim: "sub", Value: "user1"}}},                                // 1 match
				{Account: "Acc2", Roles: []string{"role-b"}, Match: []Match{{Claim: "sub", Value: "user1"}, {Claim: "aud", Value: "app1"}}}, // 2 matches - wins
			},
			context:         map[string]interface{}{"sub": "user1", "aud": "app1"},
			expectedAccount: "Acc2",
			expectedRoles:   []string{"role-b"},
		},
		{
			name:     "BestMatch: Skip binding with no match criteria",
			strategy: StrategyBestMatch,
			bindings: []RoleBinding{
				{Account: "AccNoMatch", Roles: []string{"role-a"}, Match: []Match{}},                                 // No criteria, should be skipped
				{Account: "AccWithMatch", Roles: []string{"role-b"}, Match: []Match{{Claim: "sub", Value: "user1"}}}, // Wins
			},
			context:         map[string]interface{}{"sub": "user1"},
			expectedAccount: "AccWithMatch",
			expectedRoles:   []string{"role-b"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Directly set the strategy in the config, simulating the result of UnmarshalYAML
			// For empty/invalid strings, it should default to StrategyBestMatch
			configStrategy := tt.strategy
			if configStrategy == "" || (configStrategy != StrategyStrict && configStrategy != StrategyBestMatch) {
				configStrategy = StrategyBestMatch
			}

			cfg := &Config{
				Rbac: Rbac{
					RoleBindingMatchingStrategy: configStrategy,
					RoleBinding:                 tt.bindings,
					Roles:                       roles, // Use predefined roles
				},
			}

			account, perms, _, _ := cfg.lookupUserAccount(tt.context)

			assert.Equal(t, tt.expectedAccount, account)

			// Verify the roles assigned to check if the correct binding was chosen
			if tt.expectedAccount != "" {
				assert.NotNil(t, perms, "Permissions should not be nil for a successful match")

				// Basic check: Ensure the expected permissions are present
				// (A more thorough check might involve comparing the full permission set)
				expectedPermCount := 0
				for _, expectedRoleName := range tt.expectedRoles {
					for _, role := range roles {
						if role.Name == expectedRoleName {
							assert.True(t, perms.Pub.Allow.Contains(role.Permissions.Pub.Allow[0]), "Expected pub permission from role %s not found", expectedRoleName)
							expectedPermCount++
						}
					}
				}
				assert.Equal(t, expectedPermCount, len(tt.expectedRoles), "Number of matched role permissions differs from expected")
			} else {
				assert.Nil(t, perms, "Permissions should be nil when no account is matched")
			}
		})
	}
}
