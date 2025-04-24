package server

import (
	"os"
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

		assert.Equal(t, DefaultTokenExpiryBoundsMin, cfg.NATS.TokenExpiryBounds.Min.Duration)
		assert.Equal(t, DefaultTokenExpiryBoundsMax, cfg.NATS.TokenExpiryBounds.Max.Duration)
	})
}
