package broker

import (
	"os"
	"sync"
	"testing"

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
