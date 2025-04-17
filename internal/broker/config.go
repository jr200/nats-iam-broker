package server

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/go-playground/validator"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nkeys"
	"github.com/rs/zerolog/log"
	"go.uber.org/config"
	"gopkg.in/yaml.v2"
)

// Struct definitions
type Config struct {
	AppParams ConfigParams `yaml:"params"`
	NATS      NATS         `yaml:"nats" validate:"required"`
	Service   Service      `yaml:"service" validate:"required"`
	Idp       []Idp        `yaml:"idp" validate:"required"`
	NatsJwt   NatsJwt      `yaml:"nats_jwt" validate:"required"`
	Rbac      Rbac         `yaml:"rbac" validate:"required"`
}

type ConfigParams struct {
	LeftDelim  string `yaml:"left_delim" validate:"required"`
	RightDelim string `yaml:"right_delim" validate:"required"`
}

type NATS struct {
	URL string `yaml:"url" validate:"required"`
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
	Description    string               `yaml:"description"`
	IssuerURL      string               `yaml:"issuer_url" validate:"required"`
	ClientID       string               `yaml:"client_id" validate:"required"`
	ValidationSpec IdpJwtValidationSpec `yaml:"validation"`
}

type IdpJwtValidationSpec struct {
	Claims   []string       `yaml:"claims"`
	Audience []string       `yaml:"aud"`
	Expiry   DurationBounds `yaml:"exp"`
}

type DurationBounds struct {
	Min Duration `yaml:"min" validate:"required"`
	Max Duration `yaml:"max" validate:"required"`
}

type Duration struct {
	Duration time.Duration
}

type NatsJwt struct {
	Expiry Duration `yaml:"exp_max"`
}

type NKey struct {
	KeyPair nkeys.KeyPair
}

func (v *Duration) UnmarshalText(text []byte) error {
	d, err := time.ParseDuration(string(text))
	if err != nil {
		// possibly templated
		log.Debug().Msgf("failed to parse duration from '%s' (%v)", string(text), err)
		return nil
	}
	v.Duration = d
	return nil
}

func (v *NKey) UnmarshalText(text []byte) error {
	nkey, err := nkeys.FromSeed([]byte(text))
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
		if len(lineMatch) >= 2 {
			lineNum = lineMatch[1]
		}

		// Extract the type if possible
		typeMatch := regexp.MustCompile(`into \[\]([^\s]+)`).FindStringSubmatch(errMsg)
		typeStr := "array"
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
	if len(lineMatch) >= 4 {
		lineNum := lineMatch[1]
		actualType := lineMatch[2]
		expectedType := lineMatch[3]

		typeExplanation := ""

		// Common type mismatch scenarios
		if actualType == "map" && strings.HasPrefix(expectedType, "[]") {
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
		} else if actualType == "seq" && !strings.HasPrefix(expectedType, "[]") {
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
		} else {
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

func mergeConfigurationFiles(files []string) (string, error) {

	var providerOptions []config.YAMLOption

	for _, filePath := range files {
		log.Debug().Msgf("merging config %s", filePath)
		raw, err := os.ReadFile(filePath)
		if err != nil {
			return "", fmt.Errorf("error reading file content: %v", err)
		}

		providerOptions = append(providerOptions, config.Source(strings.NewReader(string(raw))))
	}

	provider, err := config.NewYAML(providerOptions...)
	if err != nil {
		return "", fmt.Errorf("error creating YAML provider: %v", err)
	}

	var mergedMap map[string]interface{}
	if err := provider.Get(config.Root).Populate(&mergedMap); err != nil {
		return "", fmt.Errorf("error populating merged config map: %v", err)
	}

	mergedYAML, err := yaml.Marshal(mergedMap)
	if err != nil {
		return "", fmt.Errorf("error marshalling merged config to YAML: %v", err)
	}

	return string(mergedYAML), nil
}

func readConfigFiles(files []string, mappings map[string]interface{}) (*Config, error) {

	mergedYAML, err := mergeConfigurationFiles(files)
	if err != nil {
		return nil, err
	}

	// log.Trace().Msgf("Merged Yaml: \n%s", mergedYAML)

	// Initialize a default configuration with default AppParams
	cfg := Config{
		AppParams: ConfigParams{
			LeftDelim:  "{{",
			RightDelim: "}}",
		},
	}

	if err := yaml.Unmarshal([]byte(mergedYAML), &cfg); err != nil {
		return nil, fmt.Errorf("error unmarshalling merged YAML into Config: %v", err)
	}

	renderedYAML := renderAllTemplates(string(mergedYAML), mappings, cfg.AppParams)

	// log.Trace().Msgf("Rendered Yaml: \n%s", renderedYAML)

	renderedProvider, err := config.NewYAML(config.Source(strings.NewReader(renderedYAML)))
	if err != nil {
		return nil, fmt.Errorf("error creating YAML provider from rendered content: %v", err)
	}

	err = renderedProvider.Get(config.Root).Populate(&cfg)
	if err != nil {
		return nil, fmt.Errorf("error populating config: %v", err)
	}

	if cfg.Service.Name == "" {
		return nil, fmt.Errorf("missing configuration value service.name")
	}

	validate := validator.New()
	validate.RegisterValidation("semver", validateSemVer)

	err = validate.Struct(cfg)
	if err != nil {
		if validationErrors, ok := err.(validator.ValidationErrors); ok {
			var errorMessages []string
			for _, fieldErr := range validationErrors {
				errorMessages = append(errorMessages, fmt.Sprintf("Field '%s' is required", fieldErr.Field()))
			}
			combinedError := fmt.Errorf(strings.Join(errorMessages, ", "))
			return nil, combinedError
		}
	}

	return &cfg, nil
}

func (c *Config) natsOptions() []nats.Option {

	natsCreds := c.Service.CredsFile

	var opts []nats.Option
	if natsCreds != "" {
		opts = append(opts, nats.UserCredentials(natsCreds))
	}
	return opts
}

func (c *Config) serviceEncryptionXkey() nkeys.KeyPair {
	if c.Service.Account.Encryption.Enabled {
		return c.Service.Account.Encryption.Seed.KeyPair
	}

	return nil
}

func validateSemVer(fl validator.FieldLevel) bool {
	version := fl.Field().String()
	semVerRegex := `^v?(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)$`
	r := regexp.MustCompile(semVerRegex)
	return r.MatchString(version)
}
