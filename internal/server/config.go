package server

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nkeys"
	"github.com/rs/zerolog/log"
	"go.uber.org/config"
)

// Struct definitions
type ConfigParams struct {
	LeftDelim  string
	RightDelim string
}

type Config struct {
	NATS    NATS    `yaml:"nats"`
	Service Service `yaml:"service"`
	Idp     Idp     `yaml:"idp"`
	NatsJwt NatsJwt `yaml:"nats_jwt"`
	Rbac    Rbac    `yaml:"rbac"`
}

type NATS struct {
	URL string `yaml:"url"`
}

type Service struct {
	Name        string         `yaml:"name"`
	Description string         `yaml:"description"`
	Version     string         `yaml:"version"`
	CredsFile   string         `yaml:"creds_file"`
	Account     ServiceAccount `yaml:"account"`
}

type ServiceAccount struct {
	Name        string     `yaml:"name"`
	SigningNKey NKey       `yaml:"signing_nkey"`
	Encryption  Encryption `yaml:"encryption"`
}

type Encryption struct {
	Enabled bool `yaml:"enabled"`
	Seed    NKey `yaml:"xkey_secret"`
}

type Idp struct {
	IssuerURL      []string             `yaml:"issuer_url"`
	ClientID       string               `yaml:"client_id"`
	ValidationSpec IdpJwtValidationSpec `yaml:"validation"`
}

type IdpJwtValidationSpec struct {
	Claims   []string       `yaml:"claims"`
	Audience []string       `yaml:"aud"`
	Expiry   DurationBounds `yaml:"exp"`
}

type DurationBounds struct {
	Min Duration `yaml:"min"`
	Max Duration `yaml:"max"`
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
		return err
	}
	v.Duration = d
	return nil
}

func (v *NKey) UnmarshalText(text []byte) error {
	nkey, err := nkeys.FromSeed([]byte(text))
	if err != nil {
		return err
	}
	v.KeyPair = nkey
	return nil
}

func readConfigFiles(files []string, mappings map[string]interface{}, params ConfigParams) (*Config, error) {

	cfg := Config{
		Idp: Idp{
			ValidationSpec: IdpJwtValidationSpec{
				Expiry: DurationBounds{
					Min: Duration{time.Duration(0)},
					Max: Duration{time.Duration(24 * time.Hour)},
				},
			},
		},
	}

	var providerOptions []config.YAMLOption
	for _, filePath := range files {
		log.Debug().Msgf("loading config %s", filePath)
		raw, err := os.ReadFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("error reading file content: %v", err)
		}

		rendered := renderAllTemplates(string(raw), mappings, params)
		providerOptions = append(providerOptions, config.Source(strings.NewReader(rendered)))
	}

	provider, err := config.NewYAML(providerOptions...)
	if err != nil {
		return nil, err
	}

	err = provider.Get(config.Root).Populate(&cfg)
	if err != nil {
		return nil, err
	}

	if cfg.Service.Name == "" {
		return nil, fmt.Errorf("missing configuration value service.name")
	}

	// log.Trace().Msgf("cfg: %v", string(IgnoreError(yaml.Marshal(cfg))))

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
