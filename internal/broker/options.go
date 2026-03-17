package broker

const DefaultMetricsPort = 8080

// Options holds all configuration options for the server
type Options struct {
	// LogLevel sets the log level: disabled, panic, fatal, error, warn, info, debug, trace
	LogLevel string `yaml:"log_level"`

	// LogFormat sets the log output format: "json" or "human"
	LogFormat string `yaml:"log_format"`

	// LogSensitive enables logging of sensitive information (for debugging)
	LogSensitive bool `yaml:"log_sensitive"`

	// MetricsEnabled enables the Prometheus metrics endpoint
	MetricsEnabled bool `yaml:"metrics"`

	// MetricsPort is the port for the metrics HTTP server
	MetricsPort int `yaml:"metrics_port"`

	// WatchConfig enables hot-reload of configuration files via filesystem watching
	WatchConfig bool `yaml:"watch"`
}

// Context holds both server options and other server state
type Context struct {
	Options *Options
}

// NewServerContext creates a new server.Context with the given options
func NewServerContext(opts *Options) *Context {
	if opts == nil {
		opts = DefaultServerOptions()
	}
	return &Context{
		Options: opts,
	}
}

// DefaultServerOptions returns a server.Options instance with default values
func DefaultServerOptions() *Options {
	return &Options{
		LogLevel:       "info",
		LogFormat:      "json",
		LogSensitive:   false,
		MetricsEnabled: false,
		MetricsPort:    DefaultMetricsPort,
	}
}

// MergeOptions creates a final Options by starting with defaults, applying
// YAML-sourced values, then overlaying any CLI flags that were explicitly set.
// The cliFlags set contains the names of flags that were explicitly provided
// on the command line.
func MergeOptions(yamlOpts Options, cliOpts *Options, cliFlags map[string]bool) *Options {
	merged := DefaultServerOptions()

	// Apply YAML values (non-zero values override defaults)
	if yamlOpts.LogLevel != "" {
		merged.LogLevel = yamlOpts.LogLevel
	}
	if yamlOpts.LogFormat != "" {
		merged.LogFormat = yamlOpts.LogFormat
	}
	if yamlOpts.LogSensitive {
		merged.LogSensitive = true
	}
	if yamlOpts.MetricsEnabled {
		merged.MetricsEnabled = true
	}
	if yamlOpts.MetricsPort != 0 {
		merged.MetricsPort = yamlOpts.MetricsPort
	}
	if yamlOpts.WatchConfig {
		merged.WatchConfig = true
	}

	// CLI flags override everything (only if explicitly set)
	if cliFlags["log-level"] {
		merged.LogLevel = cliOpts.LogLevel
	}
	if cliFlags["log-format"] {
		merged.LogFormat = cliOpts.LogFormat
	}
	if cliFlags["log-sensitive"] {
		merged.LogSensitive = cliOpts.LogSensitive
	}
	if cliFlags["metrics"] {
		merged.MetricsEnabled = cliOpts.MetricsEnabled
	}
	if cliFlags["metrics-port"] {
		merged.MetricsPort = cliOpts.MetricsPort
	}
	if cliFlags["watch"] {
		merged.WatchConfig = cliOpts.WatchConfig
	}

	return merged
}
