package server

import (
	"bytes"
	"fmt"
	"os"
	"reflect"
	"regexp"
	"strings"
	"time"

	"github.com/go-playground/validator"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nkeys"
	"github.com/rs/zerolog/log"
	"github.com/xhit/go-str2duration/v2"
	"gopkg.in/yaml.v2"
)

const (
	DefaultTokenExpiryBoundsMin = 1 * time.Minute
	DefaultTokenExpiryBoundsMax = 1 * time.Hour
)

// Struct definitions
type Config struct {
	AppParams ConfigParams `yaml:"params"`
	NATS      NATS         `yaml:"nats" validate:"required"`
	Service   Service      `yaml:"service" validate:"required"`
	Idp       []Idp        `yaml:"idp" validate:"required"`
	Rbac      Rbac         `yaml:"rbac" validate:"required"`
}

type ConfigParams struct {
	LeftDelim  string `yaml:"left_delim"`
	RightDelim string `yaml:"right_delim"`
}

type NATS struct {
	URL               string         `yaml:"url" validate:"required"`
	TokenExpiryBounds DurationBounds `yaml:"token_bounds" validate:"required"`
}

type Service struct {
	Name        string         `yaml:"name" validate:"required"`
	Description string         `yaml:"description" validate:"required"`
	Version     string         `yaml:"version" validate:"required,semver"`
	CredsFile   string         `yaml:"creds_file" validate:"required"`
	Account     ServiceAccount `yaml:"account" validate:"required"`
}

type ServiceAccount struct {
	Name        string     `yaml:"name"`
	SigningNKey NKey       `yaml:"signing_nkey" validate:"required"`
	Encryption  Encryption `yaml:"encryption" validate:"required"`
}

type Encryption struct {
	Enabled bool `yaml:"enabled" validate:"required"`
	Seed    NKey `yaml:"xkey_secret"`
}

type Idp struct {
	Description       string               `yaml:"description"`
	IssuerURL         string               `yaml:"issuer_url" validate:"required"`
	ClientID          string               `yaml:"client_id" validate:"required"`
	ValidationSpec    IdpJwtValidationSpec `yaml:"validation"`
	UserInfo          UserInfoConfig       `yaml:"user_info"`
	TokenExpiryBounds DurationBounds       `yaml:"token_bounds"`
	CustomMapping     map[string]string    `yaml:"custom_mapping"`
	IgnoreSetupError  bool                 `yaml:"ignore_setup_error"`
}

type UserInfoConfig struct {
	Enabled bool `yaml:"enabled"`
}

type IdpJwtValidationSpec struct {
	Claims                 []string          `yaml:"claims"`
	Audience               []string          `yaml:"aud"`
	SkipAudienceValidation bool              `yaml:"skip_audience_validation"`
	TokenExpiryBounds      DurationBounds    `yaml:"token_bounds"`
	CustomClaimsMapping    map[string]string `yaml:"custom_claims_mapping"`
}

type DurationBounds struct {
	Min Duration `yaml:"min"`
	Max Duration `yaml:"max" validate:"required"`
}

type Duration struct {
	Duration time.Duration
}

type NKey struct {
	KeyPair nkeys.KeyPair
}

type TokenRequest struct {
	AccessToken  string `json:"access_token"`
	ExpiresIn    int    `json:"expires_in"`
	IDToken      string `json:"id_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
}

type ConfigManager struct {
	mergedYAML string
	baseConfig Config // stores the initial config with defaults
}

// NewConfigManager creates a new ConfigManager instance
func NewConfigManager(files []string) (*ConfigManager, error) {
	merged, err := mergeConfigurationFiles(files)
	if err != nil {
		return nil, err
	}

	// Initialize base config with defaults
	baseConfig := Config{}

	// Parse the merged YAML into base config
	if err := yaml.Unmarshal([]byte(merged), &baseConfig); err != nil {
		return nil, improveYAMLErrorMessage(err)
	}

	if baseConfig.Service.Name == "" {
		return nil, fmt.Errorf("missing configuration value service.name")
	}

	if baseConfig.AppParams.LeftDelim == "" {
		baseConfig.AppParams.LeftDelim = "{{"
	}
	if baseConfig.AppParams.RightDelim == "" {
		baseConfig.AppParams.RightDelim = "}}"
	}

	if baseConfig.NATS.TokenExpiryBounds.Min.Duration == 0 {
		baseConfig.NATS.TokenExpiryBounds.Min.Duration = DefaultTokenExpiryBoundsMin
	}
	if baseConfig.NATS.TokenExpiryBounds.Max.Duration == 0 {
		baseConfig.NATS.TokenExpiryBounds.Max.Duration = DefaultTokenExpiryBoundsMax
	}

	return &ConfigManager{
		mergedYAML: merged,
		baseConfig: baseConfig,
	}, nil
}

// GetConfig renders the merged YAML and returns a Config instance
func (cm *ConfigManager) GetConfig(mappings map[string]interface{}) (*Config, error) {
	// Create a new config instance starting with the base config
	cfg := cm.baseConfig

	// Render templates with provided mappings
	renderedYAML := renderAllTemplates(cm.mergedYAML, mappings, cfg.AppParams)

	// Create a temporary config to hold rendered values
	var tempCfg Config
	if err := yaml.Unmarshal([]byte(renderedYAML), &tempCfg); err != nil {
		return nil, improveYAMLErrorMessage(err)
	}

	// Selectively update fields that might have been templated
	// while preserving special types from the base config
	cfg.NATS.URL = tempCfg.NATS.URL
	cfg.NATS.TokenExpiryBounds = tempCfg.NATS.TokenExpiryBounds
	if cfg.NATS.TokenExpiryBounds.Max.Duration == 0 {
		cfg.NATS.TokenExpiryBounds.Max.Duration = DefaultTokenExpiryBoundsMax
	}
	if cfg.NATS.TokenExpiryBounds.Min.Duration == 0 {
		cfg.NATS.TokenExpiryBounds.Min.Duration = DefaultTokenExpiryBoundsMin
	}

	cfg.Service.Name = tempCfg.Service.Name
	cfg.Service.Description = tempCfg.Service.Description
	cfg.Service.Version = tempCfg.Service.Version
	cfg.Service.CredsFile = tempCfg.Service.CredsFile

	// Sanitize service name for use in NATS subjects
	// NATS subjects cannot contain spaces, tabs, CR, LF, or the following characters: . * > /
	// Replace any illegal characters inside the string with underscore
	// Remove illegal characters at the start or end of the string
	illegalChars := " \t\r\n.*>/"
	if strings.ContainsAny(cfg.Service.Name, illegalChars) {
		// First, replace illegal chars inside the string with underscores
		sanitizedName := cfg.Service.Name
		for _, c := range illegalChars {
			sanitizedName = strings.ReplaceAll(sanitizedName, string(c), "_")
		}

		// Then trim any remaining illegal chars from start and end
		sanitizedName = strings.TrimFunc(sanitizedName, func(r rune) bool {
			return strings.ContainsRune(illegalChars, r)
		})

		log.Warn().Str("original", cfg.Service.Name).Str("sanitized", sanitizedName).Msg("Service name contained illegal characters for NATS subjects, sanitizing")
		cfg.Service.Name = sanitizedName
	}

	// Only update NKeys if they were successfully parsed in the temp config
	if tempCfg.Service.Account.SigningNKey.KeyPair != nil {
		cfg.Service.Account.SigningNKey = tempCfg.Service.Account.SigningNKey
	}
	if tempCfg.Service.Account.Encryption.Seed.KeyPair != nil {
		cfg.Service.Account.Encryption.Seed = tempCfg.Service.Account.Encryption.Seed
	}

	cfg.Service.Account.Name = tempCfg.Service.Account.Name
	cfg.Service.Account.Encryption.Enabled = tempCfg.Service.Account.Encryption.Enabled

	// Update IDP list and RBAC
	cfg.Idp = tempCfg.Idp
	cfg.Rbac = tempCfg.Rbac

	// Validate the final config
	validate := validator.New()
	if err := validate.RegisterValidation("semver", validateSemVer); err != nil {
		return nil, fmt.Errorf("error registering semver validation: %v", err)
	}

	if err := validate.Struct(&cfg); err != nil {
		if validationErrors, ok := err.(validator.ValidationErrors); ok {
			var errorMessages []string
			for _, fieldErr := range validationErrors {
				errorMessages = append(errorMessages, "Field '"+fieldErr.Field()+"' is required")
			}
			return nil, fmt.Errorf("%s", strings.Join(errorMessages, ", "))
		}
		return nil, err
	}

	return &cfg, nil
}

// natsOptions returns the NATS options for the service account
func (c *Config) natsOptions() []nats.Option {
	natsCreds := c.Service.CredsFile

	var opts []nats.Option
	if natsCreds != "" {
		opts = append(opts, nats.UserCredentials(natsCreds))
	}
	return opts
}

// serviceEncryptionXkey returns the encryption key pair for the service account
func (c *Config) serviceEncryptionXkey() nkeys.KeyPair {
	if c.Service.Account.Encryption.Enabled {
		return c.Service.Account.Encryption.Seed.KeyPair
	}

	return nil
}

// UnmarshalText unmarshals a Duration from a string
func (v *Duration) UnmarshalText(text []byte) error {
	d, err := str2duration.ParseDuration(string(text))
	if err != nil {
		// possibly templated
		log.Debug().Msgf("failed to parse duration from '%s' (%v)", string(text), err)
		return nil
	}
	v.Duration = d
	return nil
}

// UnmarshalText unmarshals a NKey from a string
func (v *NKey) UnmarshalText(text []byte) error {
	text = bytes.TrimSpace(text)
	nkey, err := nkeys.FromSeed(text)
	if err != nil {
		// possibly templated
		log.Debug().Msgf("skipped parsing nkey: %v (%v)", SecureLogKey(string(text)), err)
		return nil
	}
	v.KeyPair = nkey
	return nil
}

// Helper function to improve YAML unmarshaling error messages
func improveYAMLErrorMessage(err error) error {
	errMsg := err.Error()

	// Check if it's an unmarshaling error
	if !strings.Contains(errMsg, "unmarshal") {
		return err
	}

	// First check for the specific array error pattern that doesn't get captured properly
	if strings.Contains(errMsg, "cannot unmarshal !!map into []") {
		// Extract a more helpful line number if possible
		lineMatch := regexp.MustCompile(`line (\d+):`).FindStringSubmatch(errMsg)
		lineNum := "unknown"
		//nolint:mnd // 2 is the index of the line number capture group in the regex
		if len(lineMatch) >= 2 {
			lineNum = lineMatch[1]
		}

		// Extract the type if possible
		typeMatch := regexp.MustCompile(`into \[\]([^\s]+)`).FindStringSubmatch(errMsg)
		typeStr := "array"
		//nolint:mnd // 2 is the index of the type capture group in the regex
		if len(typeMatch) >= 2 {
			typeStr = typeMatch[1]
		}

		typeExplanation := fmt.Sprintf(
			"In your YAML configuration: Found a single object where an array was expected.\n"+
				"The merged configuration shows this at line %s, but it might be in any of your config files.\n"+
				"Field type: []%s\n"+
				"Check ALL your config files for this pattern:\n"+
				"  field: { key: value }\n"+
				"And replace with:\n"+
				"  field:\n"+
				"  - { key: value }\n"+
				"Note the dash (-) which indicates an array element.",
			lineNum, typeStr)

		return fmt.Errorf("YAML configuration error: %s\n\nOriginal error: %v", typeExplanation, err)
	}

	// Extract line number and type information if available
	lineMatch := regexp.MustCompile(`line (\d+):.*unmarshal !!(\w+) into \[?]?([^\s]+)`).FindStringSubmatch(errMsg)
	//nolint:mnd // 4 is the minimum length of lineMatch array to have all capture groups
	if len(lineMatch) >= 4 {
		lineNum := lineMatch[1]
		actualType := lineMatch[2]
		expectedType := lineMatch[3]

		typeExplanation := ""

		// Common type mismatch scenarios
		switch {
		case actualType == "map" && strings.HasPrefix(expectedType, "[]"):
			// Map into array error
			typeExplanation = fmt.Sprintf(
				"In your YAML configuration: Found a single object where an array was expected.\n"+
					"The merged configuration shows this at line %s, but it might be in any of your config files.\n"+
					"Field type: %s\n"+
					"Check ALL your config files for this pattern:\n"+
					"  field: { key: value }\n"+
					"And replace with:\n"+
					"  field:\n"+
					"  - { key: value }\n"+
					"Note the dash (-) which indicates an array element.",
				lineNum, expectedType)
		case actualType == "seq" && !strings.HasPrefix(expectedType, "[]"):
			// Array into non-array error
			typeExplanation = fmt.Sprintf(
				"In your YAML configuration: Found an array where a single object was expected.\n"+
					"The merged configuration shows this at line %s, but it might be in any of your config files.\n"+
					"Field type: %s\n"+
					"Check ALL your config files for this pattern:\n"+
					"  field:\n"+
					"  - key: value\n"+
					"And replace with:\n"+
					"  field:\n"+
					"    key: value\n"+
					"Remove the dash (-) since this isn't an array.",
				lineNum, expectedType)
		default:
			// Generic type mismatch
			typeExplanation = fmt.Sprintf(
				"In your YAML configuration: Type mismatch - found '%s' but expected '%s'.\n"+
					"The merged configuration shows this at line %s, but it might be in any of your config files.\n"+
					"Check the YAML structure in all your configuration files.",
				actualType, expectedType, lineNum)
		}

		return fmt.Errorf("YAML configuration error: %s\n\nOriginal error: %v", typeExplanation, err)
	}

	// If we couldn't parse the specific error, return a slightly improved general message
	return fmt.Errorf("error in YAML configuration. Please check your YAML syntax and field types.\nOriginal error: %v", err)
}

// mergeConfigurationFiles merges the given YAML files into a single YAML string
func mergeConfigurationFiles(files []string) (string, error) {
	var mergedMap map[interface{}]interface{}

	for _, filePath := range files {
		log.Debug().Msgf("merging config %s", filePath)
		raw, err := os.ReadFile(filePath)
		if err != nil {
			return "", fmt.Errorf("error reading file content: %v", err)
		}

		var nextMapToMerge map[interface{}]interface{}
		if err := yaml.Unmarshal(raw, &nextMapToMerge); err != nil {
			return "", fmt.Errorf("error in file %s: %v", filePath, improveYAMLErrorMessage(err))
		}

		if mergedMap == nil {
			mergedMap = nextMapToMerge
			continue
		}

		// Recursively merge the maps
		mergedMap = deepMerge(mergedMap, nextMapToMerge)
	}

	mergedYAML, err := yaml.Marshal(mergedMap)
	if err != nil {
		return "", fmt.Errorf("error marshalling merged config to YAML: %v", err)
	}

	return string(mergedYAML), nil
}

func deepMerge(base, overlay map[interface{}]interface{}) map[interface{}]interface{} {
	result := make(map[interface{}]interface{})

	// Copy base map
	for k, v := range base {
		result[k] = v
	}

	// Merge overlay map
	for k, v := range overlay {
		if baseVal, exists := result[k]; exists {
			baseType := reflect.TypeOf(baseVal)
			overlayType := reflect.TypeOf(v)

			if baseType != overlayType {
				panic(fmt.Sprintf("Type mismatch for key '%s': cannot merge %v with %v", k, baseType, overlayType))
			}

			// If both values are arrays, concatenate them
			if baseArr, ok := baseVal.([]interface{}); ok {
				if overlayArr, ok := v.([]interface{}); ok {
					result[k] = append(baseArr, overlayArr...)
					continue
				}
			}

			// If both values are maps, merge them recursively
			if baseMap, ok := baseVal.(map[interface{}]interface{}); ok {
				if overlayMap, ok := v.(map[interface{}]interface{}); ok {
					result[k] = deepMerge(baseMap, overlayMap)
					continue
				}
			}

			// For primitive types (strings, numbers, bools), overlay value takes precedence
			result[k] = v
			continue
		}

		// If key doesn't exist in base, add from overlay
		result[k] = v
	}

	return result
}

// validateSemVer validates that a version string is a valid semantic version
func validateSemVer(fl validator.FieldLevel) bool {
	version := fl.Field().String()
	semVerRegex := `^v?(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)$`
	r := regexp.MustCompile(semVerRegex)
	return r.MatchString(version)
}
