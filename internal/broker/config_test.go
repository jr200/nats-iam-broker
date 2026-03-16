package broker

import (
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/goccy/go-yaml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigManager(t *testing.T) {
	// Setup test files
	baseConfig := `
params:
  left_delim: "{{"
  right_delim: "}}"
nats:
  url: "nats://localhost:4222"
service:
  name: "test-service"
  description: "Test Service"
  version: "1.0.0"
  creds_file: "/path/to/creds"
  account:
    name: "test"
    signing_nkey: "SUAGJBPRRXFQL2DXLG4CXW5D6XTLJ4DDMMKHNCIAPNK2Y4IZFHTJM6HN"
    encryption:
      enabled: true
idp:
  - description: "Test IDP"
    issuer_url: "https://test.idp"
    client_id: "test-client"
rbac:
  user_accounts:
    - name: "test-account"
      public_key: "test-key"
      signing_nkey: "SUAGJBPRRXFQL2DXLG4CXW5D6XTLJ4DDMMKHNCIAPNK2Y4IZFHTJM6HN"
  role_binding:
    - user_account: "test-account"
      roles: ["test-role"]
      match:
        - claim: "email"
          value: "{{.email}}"
    - user_account: "templated-account"
      roles: ["test-role"]
      match:
        - claim: "group"
          value: "{{.group}}"
  roles:
    - name: "test-role"
      permissions:
        pub:
          allow: ["test.>"]
`

	// Write test config to temp file
	baseFile, err := os.CreateTemp("", "base-*.yaml")
	require.NoError(t, err)
	defer os.Remove(baseFile.Name())
	_, err = baseFile.WriteString(baseConfig)
	require.NoError(t, err)

	t.Run("Initialize ConfigManager", func(t *testing.T) {
		cm, err := NewConfigManager([]string{baseFile.Name()})
		require.NoError(t, err)
		assert.NotEmpty(t, cm.mergedYAML)
		assert.Equal(t, "test-service", cm.baseConfig.Service.Name)
	})

	t.Run("Get Config with different mappings", func(t *testing.T) {
		cm, err := NewConfigManager([]string{baseFile.Name()})
		require.NoError(t, err)

		// Test with empty mappings
		cfg1, err := cm.GetConfig(map[string]interface{}{})
		require.NoError(t, err)
		assert.Equal(t, "test-service", cfg1.Service.Name)

		// Test with mappings
		mappings := map[string]interface{}{
			"email": "test@example.com",
			"group": "admin",
		}
		cfg2, err := cm.GetConfig(mappings)
		require.NoError(t, err)

		// Find and verify the rendered values
		var emailMatch, groupMatch string
		for _, rb := range cfg2.Rbac.RoleBinding {
			for _, match := range rb.Match {
				switch match.Claim {
				case "email":
					emailMatch = match.Value
				case "group":
					groupMatch = match.Value
				}
			}
		}

		assert.Equal(t, "test@example.com", emailMatch, "Email template should be rendered correctly")
		assert.Equal(t, "admin", groupMatch, "Group template should be rendered correctly")
	})

	t.Run("Verify default values", func(t *testing.T) {
		cm, err := NewConfigManager([]string{baseFile.Name()})
		require.NoError(t, err)

		cfg, err := cm.GetConfig(map[string]interface{}{})
		require.NoError(t, err)

		assert.Equal(t, DefaultTokenExpiryBoundsLower, cfg.NATS.TokenExpiryBounds.Min.Duration)
		assert.Equal(t, DefaultTokenExpiryBoundsUpper, cfg.NATS.TokenExpiryBounds.Max.Duration)
	})

	t.Run("ConfigManager caches validator", func(t *testing.T) {
		cm, err := NewConfigManager([]string{baseFile.Name()})
		require.NoError(t, err)

		assert.NotNil(t, cm.validate, "validator should be pre-compiled")

		// Verify it can validate configs successfully
		cfg, err := cm.GetConfig(map[string]interface{}{})
		require.NoError(t, err)
		assert.NotNil(t, cfg)
	})

	t.Run("ConfigManager caches template regex and templates", func(t *testing.T) {
		cm, err := NewConfigManager([]string{baseFile.Name()})
		require.NoError(t, err)

		assert.NotNil(t, cm.templateCache, "template cache should be initialized")
		assert.NotNil(t, cm.templateCache.regex, "template regex should be pre-compiled")
		assert.NotEmpty(t, cm.templateCache.templates, "templates should be pre-compiled")
	})

	t.Run("ConfigManager initializes exprCache", func(t *testing.T) {
		cm, err := NewConfigManager([]string{baseFile.Name()})
		require.NoError(t, err)

		assert.NotNil(t, cm.exprCache, "expr cache should be initialized")
	})

	t.Run("GetConfig propagates exprCache to Config", func(t *testing.T) {
		cm, err := NewConfigManager([]string{baseFile.Name()})
		require.NoError(t, err)

		cfg, err := cm.GetConfig(map[string]interface{}{})
		require.NoError(t, err)

		assert.NotNil(t, cfg.exprCache, "Config should have exprCache from ConfigManager")
		assert.Same(t, cm.exprCache, cfg.exprCache, "Config should share the same exprCache as ConfigManager")
	})

	t.Run("Multiple GetConfig calls share the same exprCache", func(t *testing.T) {
		cm, err := NewConfigManager([]string{baseFile.Name()})
		require.NoError(t, err)

		cfg1, err := cm.GetConfig(map[string]interface{}{"email": "a@test.com"})
		require.NoError(t, err)
		cfg2, err := cm.GetConfig(map[string]interface{}{"email": "b@test.com"})
		require.NoError(t, err)

		assert.Same(t, cfg1.exprCache, cfg2.exprCache, "all configs from same manager should share exprCache")
	})
}

func TestConfigParsePhase_Atomic(t *testing.T) {
	t.Run("initial phase is render", func(t *testing.T) {
		assert.Equal(t, configPhaseRender, getConfigParsePhase())
	})

	t.Run("set and get are consistent", func(t *testing.T) {
		// Save original and restore after test
		original := getConfigParsePhase()
		defer setConfigParsePhase(original)

		setConfigParsePhase(configPhaseInitial)
		assert.Equal(t, configPhaseInitial, getConfigParsePhase())

		setConfigParsePhase(configPhaseRender)
		assert.Equal(t, configPhaseRender, getConfigParsePhase())
	})

	t.Run("concurrent reads do not panic", func(t *testing.T) {
		var wg sync.WaitGroup
		const goroutines = 50

		for i := 0; i < goroutines; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				phase := getConfigParsePhase()
				assert.Contains(t, []string{configPhaseInitial, configPhaseRender}, phase)
			}()
		}

		wg.Wait()
	})
}

func TestDeepMerge(t *testing.T) {
	tests := []struct {
		name     string
		base     map[string]interface{}
		overlay  map[string]interface{}
		expected map[string]interface{}
	}{
		{
			name:     "disjoint keys",
			base:     map[string]interface{}{"a": 1},
			overlay:  map[string]interface{}{"b": 2},
			expected: map[string]interface{}{"a": 1, "b": 2},
		},
		{
			name:     "overlay primitive wins",
			base:     map[string]interface{}{"a": "old"},
			overlay:  map[string]interface{}{"a": "new"},
			expected: map[string]interface{}{"a": "new"},
		},
		{
			name:     "overlay bool wins",
			base:     map[string]interface{}{"flag": true},
			overlay:  map[string]interface{}{"flag": false},
			expected: map[string]interface{}{"flag": false},
		},
		{
			name: "nested maps merged recursively",
			base: map[string]interface{}{
				"nested": map[string]interface{}{"a": 1, "b": 2},
			},
			overlay: map[string]interface{}{
				"nested": map[string]interface{}{"b": 99, "c": 3},
			},
			expected: map[string]interface{}{
				"nested": map[string]interface{}{"a": 1, "b": 99, "c": 3},
			},
		},
		{
			name: "arrays concatenated",
			base: map[string]interface{}{
				"items": []interface{}{"a", "b"},
			},
			overlay: map[string]interface{}{
				"items": []interface{}{"c"},
			},
			expected: map[string]interface{}{
				"items": []interface{}{"a", "b", "c"},
			},
		},
		{
			name: "empty arrays concatenated",
			base: map[string]interface{}{
				"items": []interface{}{},
			},
			overlay: map[string]interface{}{
				"items": []interface{}{},
			},
			expected: map[string]interface{}{
				"items": []interface{}{},
			},
		},
		{
			name: "array in base non-array in overlay",
			base: map[string]interface{}{
				"items": []interface{}{"a"},
			},
			overlay: map[string]interface{}{
				"items": "override",
			},
			expected: map[string]interface{}{
				"items": "override",
			},
		},
		{
			name: "map in base non-map in overlay",
			base: map[string]interface{}{
				"nested": map[string]interface{}{"a": 1},
			},
			overlay: map[string]interface{}{
				"nested": "flat",
			},
			expected: map[string]interface{}{
				"nested": "flat",
			},
		},
		{
			name: "deeply nested merge (3 levels)",
			base: map[string]interface{}{
				"l1": map[string]interface{}{
					"l2": map[string]interface{}{
						"l3":   "base",
						"keep": "yes",
					},
				},
			},
			overlay: map[string]interface{}{
				"l1": map[string]interface{}{
					"l2": map[string]interface{}{
						"l3": "overlay",
					},
				},
			},
			expected: map[string]interface{}{
				"l1": map[string]interface{}{
					"l2": map[string]interface{}{
						"l3":   "overlay",
						"keep": "yes",
					},
				},
			},
		},
		{
			name:     "empty overlay preserves base",
			base:     map[string]interface{}{"a": 1, "b": 2},
			overlay:  map[string]interface{}{},
			expected: map[string]interface{}{"a": 1, "b": 2},
		},
		{
			name:     "empty base returns overlay",
			base:     map[string]interface{}{},
			overlay:  map[string]interface{}{"x": "y"},
			expected: map[string]interface{}{"x": "y"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := deepMerge(tt.base, tt.overlay)
			assert.Equal(t, tt.expected, result)
		})
	}

	t.Run("base not mutated", func(t *testing.T) {
		base := map[string]interface{}{"a": 1}
		overlay := map[string]interface{}{"a": 2, "b": 3}
		deepMerge(base, overlay)
		assert.Equal(t, map[string]interface{}{"a": 1}, base)
	})
}

func TestDurationUnmarshalYAML(t *testing.T) {
	type wrapper struct {
		D Duration `yaml:"d"`
	}

	tests := []struct {
		name     string
		input    string
		expected time.Duration
		wantErr  bool
	}{
		{
			name:     "string duration 1h",
			input:    "d: \"1h\"",
			expected: 1 * time.Hour,
		},
		{
			name:     "string duration 30m",
			input:    "d: \"30m\"",
			expected: 30 * time.Minute,
		},
		{
			name:     "string duration 1h30m",
			input:    "d: \"1h30m\"",
			expected: 90 * time.Minute,
		},
		{
			name:     "integer seconds via str2duration",
			input:    "d: \"1000000000ns\"",
			expected: 1 * time.Second,
		},
		{
			name:     "map with value key",
			input:    "d:\n  value: \"2h\"",
			expected: 2 * time.Hour,
		},
		{
			name:     "map with max key",
			input:    "d:\n  max: \"45m\"",
			expected: 45 * time.Minute,
		},
		{
			name:     "map with min key",
			input:    "d:\n  min: \"5m\"",
			expected: 5 * time.Minute,
		},
		{
			name:    "map with unknown keys",
			input:   "d:\n  foo: \"1h\"",
			wantErr: true,
		},
		{
			name:     "zero duration from empty string",
			input:    "d: \"0s\"",
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var w wrapper
			err := yaml.Unmarshal([]byte(tt.input), &w)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, w.D.Duration)
			}
		})
	}
}

func TestImproveYAMLErrorMessage(t *testing.T) {
	tests := []struct {
		name           string
		inputErr       error
		wantChanged    bool
		wantSubstrings []string
	}{
		{
			name:        "non-unmarshal error passes through",
			inputErr:    fmt.Errorf("syntax error at line 5"),
			wantChanged: false,
		},
		{
			name:        "map into array with line number",
			inputErr:    fmt.Errorf("line 10: cannot unmarshal !!map into []broker.Idp"),
			wantChanged: true,
			wantSubstrings: []string{
				"single object where an array was expected",
				"10",
				"broker.Idp",
			},
		},
		{
			name:        "map into array without line number",
			inputErr:    fmt.Errorf("cannot unmarshal !!map into []broker.Idp"),
			wantChanged: true,
			wantSubstrings: []string{
				"single object where an array was expected",
				"unknown",
				"broker.Idp",
			},
		},
		{
			name:        "seq into non-array",
			inputErr:    fmt.Errorf("line 5: cannot unmarshal !!seq into broker.NATS"),
			wantChanged: true,
			wantSubstrings: []string{
				"array where a single object was expected",
				"5",
				"broker.NATS",
			},
		},
		{
			name:        "generic type mismatch",
			inputErr:    fmt.Errorf("line 3: cannot unmarshal !!str into int"),
			wantChanged: true,
			wantSubstrings: []string{
				"Type mismatch",
				"str",
				"int",
				"3",
			},
		},
		{
			name:        "unmarshal but no regex match",
			inputErr:    fmt.Errorf("unmarshal error: something weird"),
			wantChanged: true,
			wantSubstrings: []string{
				"check your YAML syntax",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := improveYAMLErrorMessage(tt.inputErr)
			if !tt.wantChanged {
				assert.Equal(t, tt.inputErr, result)
			} else {
				assert.NotEqual(t, tt.inputErr.Error(), result.Error())
				for _, sub := range tt.wantSubstrings {
					assert.Contains(t, result.Error(), sub)
				}
			}
		})
	}
}

func TestGetConfig_ConcurrentAccess(t *testing.T) {
	baseConfig := `
nats:
  url: "nats://localhost:4222"
service:
  name: "concurrent-test"
  description: "Concurrent Test"
  version: "1.0.0"
  creds_file: "/path/to/creds"
  account:
    name: "test"
    signing_nkey: "SUAGJBPRRXFQL2DXLG4CXW5D6XTLJ4DDMMKHNCIAPNK2Y4IZFHTJM6HN"
    encryption:
      enabled: true
idp:
  - description: "Test IDP"
    issuer_url: "https://test.idp"
    client_id: "test-client"
rbac:
  user_accounts:
    - name: "test-account"
      public_key: "test-key"
      signing_nkey: "SUAGJBPRRXFQL2DXLG4CXW5D6XTLJ4DDMMKHNCIAPNK2Y4IZFHTJM6HN"
  role_binding:
    - user_account: "test-account"
      roles: ["test-role"]
      match:
        - claim: "email"
          value: "{{.email}}"
  roles:
    - name: "test-role"
      permissions:
        pub:
          allow: ["test.>"]
`

	baseFile, err := os.CreateTemp("", "concurrent-*.yaml")
	require.NoError(t, err)
	defer os.Remove(baseFile.Name())
	_, err = baseFile.WriteString(baseConfig)
	require.NoError(t, err)

	cm, err := NewConfigManager([]string{baseFile.Name()})
	require.NoError(t, err)

	var wg sync.WaitGroup
	const goroutines = 20

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			mappings := map[string]interface{}{
				"email": "user@test.com",
			}
			cfg, err := cm.GetConfig(mappings)
			assert.NoError(t, err)
			assert.NotNil(t, cfg)
			assert.Equal(t, "concurrent-test", cfg.Service.Name)
		}()
	}

	wg.Wait()
}
