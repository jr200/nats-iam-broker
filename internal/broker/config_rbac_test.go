package broker

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/nats-io/jwt/v2"
	"github.com/nats-io/nkeys"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDiscoverAccounts(t *testing.T) {
	// Helper to create a valid account keypair and write its files
	writeAccountFiles := func(t *testing.T, dir, name string) string {
		t.Helper()
		kp, err := nkeys.CreateAccount()
		require.NoError(t, err)
		pub, err := kp.PublicKey()
		require.NoError(t, err)
		sd, err := kp.Seed()
		require.NoError(t, err)

		require.NoError(t, os.WriteFile(filepath.Join(dir, name+"-id-1.pub"), []byte(pub), 0644))
		require.NoError(t, os.WriteFile(filepath.Join(dir, name+"-sk-1.nk"), sd, 0644))
		return pub
	}

	t.Run("empty dir config returns nil", func(t *testing.T) {
		r := &Rbac{AutoAccountsDir: ""}
		accounts, err := r.discoverAccounts()
		assert.NoError(t, err)
		assert.Nil(t, accounts)
	})

	t.Run("no matching files returns empty", func(t *testing.T) {
		dir := t.TempDir()
		r := &Rbac{AutoAccountsDir: dir}
		accounts, err := r.discoverAccounts()
		assert.NoError(t, err)
		assert.Empty(t, accounts)
	})

	t.Run("discovers single account", func(t *testing.T) {
		dir := t.TempDir()
		pub := writeAccountFiles(t, dir, "ACME")

		r := &Rbac{AutoAccountsDir: dir}
		accounts, err := r.discoverAccounts()
		require.NoError(t, err)
		require.Len(t, accounts, 1)
		assert.Equal(t, "ACME", accounts[0].Name)
		assert.Equal(t, pub, accounts[0].PublicKey)
		assert.NotNil(t, accounts[0].SigningNKey.KeyPair)
	})

	t.Run("discovers multiple accounts", func(t *testing.T) {
		dir := t.TempDir()
		writeAccountFiles(t, dir, "ALPHA")
		writeAccountFiles(t, dir, "BETA")

		r := &Rbac{AutoAccountsDir: dir}
		accounts, err := r.discoverAccounts()
		require.NoError(t, err)
		require.Len(t, accounts, 2)

		names := []string{accounts[0].Name, accounts[1].Name}
		assert.Contains(t, names, "ALPHA")
		assert.Contains(t, names, "BETA")
	})

	t.Run("skips pub without matching sk", func(t *testing.T) {
		dir := t.TempDir()
		kp, err := nkeys.CreateAccount()
		require.NoError(t, err)
		pub, _ := kp.PublicKey()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "ORPHAN-id-1.pub"), []byte(pub), 0644))

		r := &Rbac{AutoAccountsDir: dir}
		accounts, err := r.discoverAccounts()
		assert.NoError(t, err)
		assert.Empty(t, accounts)
	})

	t.Run("ignores unrelated files", func(t *testing.T) {
		dir := t.TempDir()
		writeAccountFiles(t, dir, "REAL")
		require.NoError(t, os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("hello"), 0644))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "config.yaml"), []byte("x: y"), 0644))

		r := &Rbac{AutoAccountsDir: dir}
		accounts, err := r.discoverAccounts()
		require.NoError(t, err)
		require.Len(t, accounts, 1)
		assert.Equal(t, "REAL", accounts[0].Name)
	})

	t.Run("invalid signing key produces nil KeyPair", func(t *testing.T) {
		dir := t.TempDir()
		kp, err := nkeys.CreateAccount()
		require.NoError(t, err)
		pub, _ := kp.PublicKey()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "BAD-id-1.pub"), []byte(pub), 0644))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "BAD-sk-1.nk"), []byte("not-a-valid-seed"), 0644))

		r := &Rbac{AutoAccountsDir: dir}
		accounts, err := r.discoverAccounts()
		// NKey.UnmarshalText logs a warning but doesn't return an error
		assert.NoError(t, err)
		require.Len(t, accounts, 1)
		assert.Nil(t, accounts[0].SigningNKey.KeyPair)
	})
}

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

		// --- Expr-based Matching Tests ---
		{
			name:     "Expr: Simple equality",
			strategy: StrategyStrict,
			bindings: []RoleBinding{
				{Account: "AccExpr", Roles: []string{"role-a"}, Match: []Match{{Expr: `sub == "user1"`}}},
			},
			context:         map[string]interface{}{"sub": "user1"},
			expectedAccount: "AccExpr",
			expectedRoles:   []string{"role-a"},
		},
		{
			name:     "Expr: Array membership with 'in'",
			strategy: StrategyStrict,
			bindings: []RoleBinding{
				{Account: "AccExpr", Roles: []string{"role-b"}, Match: []Match{{Expr: `"superuser" in groups`}}},
			},
			context:         map[string]interface{}{"groups": []interface{}{"superuser", "admin"}},
			expectedAccount: "AccExpr",
			expectedRoles:   []string{"role-b"},
		},
		{
			name:     "Expr: Array membership miss",
			strategy: StrategyStrict,
			bindings: []RoleBinding{
				{Account: "AccExpr", Roles: []string{"role-b"}, Match: []Match{{Expr: `"superuser" in groups`}}},
			},
			context:         map[string]interface{}{"groups": []interface{}{"viewer"}},
			expectedAccount: "",
			expectedRoles:   nil,
		},
		{
			name:     "Expr: Combined with legacy match",
			strategy: StrategyStrict,
			bindings: []RoleBinding{
				{Account: "AccMixed", Roles: []string{"role-c"}, Match: []Match{
					{Expr: `"admin" in groups`},
					{Claim: "sub", Value: "user1"},
				}},
			},
			context:         map[string]interface{}{"sub": "user1", "groups": []interface{}{"admin", "dev"}},
			expectedAccount: "AccMixed",
			expectedRoles:   []string{"role-c"},
		},
		{
			name:     "Expr: Logical operators",
			strategy: StrategyBestMatch,
			bindings: []RoleBinding{
				{Account: "AccLogic", Roles: []string{"role-a"}, Match: []Match{
					{Expr: `sub == "user1" && email == "user1@example.com"`},
				}},
			},
			context:         map[string]interface{}{"sub": "user1", "email": "user1@example.com"},
			expectedAccount: "AccLogic",
			expectedRoles:   []string{"role-a"},
		},
		{
			name:     "Expr: Compile error returns no match",
			strategy: StrategyStrict,
			bindings: []RoleBinding{
				{Account: "AccBad", Roles: []string{"role-a"}, Match: []Match{{Expr: `invalid syntax !!!`}}},
			},
			context:         map[string]interface{}{"sub": "user1"},
			expectedAccount: "",
			expectedRoles:   nil,
		},

		// --- Fallback Binding Tests ---
		{
			name:     "BestMatch: Fallback used when no match",
			strategy: StrategyBestMatch,
			bindings: []RoleBinding{
				{Account: "AccSpecific", Roles: []string{"role-a"}, Match: []Match{{Claim: "sub", Value: "user1"}}},
				{Account: "AccFallback", Roles: []string{"role-b"}, Match: []Match{}}, // Fallback
			},
			context:         map[string]interface{}{"sub": "user99"}, // No match
			expectedAccount: "AccFallback",
			expectedRoles:   []string{"role-b"},
		},
		{
			name:     "Strict: Fallback used when no strict match",
			strategy: StrategyStrict,
			bindings: []RoleBinding{
				{Account: "AccStrict", Roles: []string{"role-a"}, Match: []Match{{Claim: "sub", Value: "user1"}, {Claim: "aud", Value: "app1"}}},
				{Account: "AccFallback", Roles: []string{"role-c"}, Match: []Match{}}, // Fallback
			},
			context:         map[string]interface{}{"sub": "user1", "aud": "app2"}, // Partial match, strict fails
			expectedAccount: "AccFallback",
			expectedRoles:   []string{"role-c"},
		},
		{
			name:     "BestMatch: Fallback not used when match exists",
			strategy: StrategyBestMatch,
			bindings: []RoleBinding{
				{Account: "AccFallback", Roles: []string{"role-c"}, Match: []Match{}},                            // Fallback, ignored
				{Account: "AccMatch", Roles: []string{"role-a"}, Match: []Match{{Claim: "sub", Value: "user1"}}}, // Wins
			},
			context:         map[string]interface{}{"sub": "user1"},
			expectedAccount: "AccMatch",
			expectedRoles:   []string{"role-a"},
		},
		{
			name:     "BestMatch: First fallback wins when multiple fallbacks",
			strategy: StrategyBestMatch,
			bindings: []RoleBinding{
				{Account: "AccFallback1", Roles: []string{"role-a"}, Match: []Match{}}, // First fallback
				{Account: "AccFallback2", Roles: []string{"role-b"}, Match: []Match{}}, // Second fallback, ignored
			},
			context:         map[string]interface{}{"sub": "nobody"},
			expectedAccount: "AccFallback1",
			expectedRoles:   []string{"role-a"},
		},
		{
			name:     "BestMatch: No match and no fallback",
			strategy: StrategyBestMatch,
			bindings: []RoleBinding{
				{Account: "Acc1", Roles: []string{"role-a"}, Match: []Match{{Claim: "sub", Value: "user1"}}},
			},
			context:         map[string]interface{}{"sub": "nobody"},
			expectedAccount: "",
			expectedRoles:   nil,
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

			account, perms, _, _, err := cfg.lookupUserAccount(tt.context)

			if tt.expectedAccount == "" {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
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

func TestLoadOrCompileExpr(t *testing.T) {
	ctx := map[string]interface{}{"sub": "user1", "email": "user1@test.com"}

	t.Run("compiles and returns program without cache", func(t *testing.T) {
		program, err := loadOrCompileExpr(`sub == "user1"`, ctx, nil)
		require.NoError(t, err)
		assert.NotNil(t, program)
	})

	t.Run("returns error for invalid expression without cache", func(t *testing.T) {
		_, err := loadOrCompileExpr(`invalid syntax !!!`, ctx, nil)
		assert.Error(t, err)
	})

	t.Run("populates cache on first call", func(t *testing.T) {
		cache := &sync.Map{}
		expression := `sub == "user1"`

		program, err := loadOrCompileExpr(expression, ctx, cache)
		require.NoError(t, err)
		assert.NotNil(t, program)

		// Verify it was stored in cache
		cached, ok := cache.Load(expression)
		assert.True(t, ok, "program should be stored in cache")
		assert.Equal(t, program, cached)
	})

	t.Run("returns cached program on subsequent calls", func(t *testing.T) {
		cache := &sync.Map{}
		expression := `email == "user1@test.com"`

		// First call populates cache
		program1, err := loadOrCompileExpr(expression, ctx, cache)
		require.NoError(t, err)

		// Second call should return the same program from cache
		program2, err := loadOrCompileExpr(expression, ctx, cache)
		require.NoError(t, err)

		assert.Same(t, program1, program2, "second call should return the same cached program pointer")
	})

	t.Run("does not cache on error", func(t *testing.T) {
		cache := &sync.Map{}
		expression := `invalid syntax !!!`

		_, err := loadOrCompileExpr(expression, ctx, cache)
		assert.Error(t, err)

		_, ok := cache.Load(expression)
		assert.False(t, ok, "failed compilations should not be cached")
	})
}

func TestExprCache_ConcurrentAccess(t *testing.T) {
	cache := &sync.Map{}
	ctx := map[string]interface{}{"sub": "user1", "groups": []interface{}{"admin", "dev"}}
	expressions := []string{
		`sub == "user1"`,
		`"admin" in groups`,
		`"dev" in groups`,
		`sub == "user1" && "admin" in groups`,
	}

	var wg sync.WaitGroup
	const goroutines = 20

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			expression := expressions[idx%len(expressions)]
			program, err := loadOrCompileExpr(expression, ctx, cache)
			assert.NoError(t, err)
			assert.NotNil(t, program)
		}(i)
	}

	wg.Wait()

	// Verify all expressions were cached
	for _, expression := range expressions {
		_, ok := cache.Load(expression)
		assert.True(t, ok, "expression %q should be in cache", expression)
	}
}

func TestLookupUserAccount_WithExprCache(t *testing.T) {
	roles := []Role{
		{Name: "role-a", Permissions: Permissions{Pub: jwt.Permission{Allow: []string{"a.>"}}}},
	}

	t.Run("expr results are cached across calls", func(t *testing.T) {
		cache := &sync.Map{}
		cfg := &Config{
			Rbac: Rbac{
				RoleBindingMatchingStrategy: StrategyStrict,
				RoleBinding: []RoleBinding{
					{Account: "Acc1", Roles: []string{"role-a"}, Match: []Match{{Expr: `sub == "user1"`}}},
				},
				Roles: roles,
			},
			exprCache: cache,
		}

		// First call
		account, _, _, _, err := cfg.lookupUserAccount(map[string]interface{}{"sub": "user1"})
		require.NoError(t, err)
		assert.Equal(t, "Acc1", account)

		// Verify expression was cached
		_, ok := cache.Load(`sub == "user1"`)
		assert.True(t, ok, "expression should be in cache after first call")

		// Second call should use cache
		account2, _, _, _, err := cfg.lookupUserAccount(map[string]interface{}{"sub": "user1"})
		require.NoError(t, err)
		assert.Equal(t, "Acc1", account2)
	})

	t.Run("nil exprCache falls back to uncached behavior", func(t *testing.T) {
		cfg := &Config{
			Rbac: Rbac{
				RoleBindingMatchingStrategy: StrategyStrict,
				RoleBinding: []RoleBinding{
					{Account: "Acc1", Roles: []string{"role-a"}, Match: []Match{{Expr: `sub == "user1"`}}},
				},
				Roles: roles,
			},
			exprCache: nil,
		}

		account, _, _, _, err := cfg.lookupUserAccount(map[string]interface{}{"sub": "user1"})
		require.NoError(t, err)
		assert.Equal(t, "Acc1", account)
	})
}
