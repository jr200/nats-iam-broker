package server

// ServerOptions holds all configuration options for the server
type ServerOptions struct {
	// LogSensitive enables logging of sensitive information (for debugging)
	LogSensitive bool
}

// ServerContext holds both server options and other server state
type ServerContext struct {
	Options *ServerOptions
}

// NewServerContext creates a new ServerContext with the given options
func NewServerContext(opts *ServerOptions) *ServerContext {
	if opts == nil {
		opts = DefaultServerOptions()
	}
	return &ServerContext{
		Options: opts,
	}
}

// DefaultServerOptions returns a ServerOptions instance with default values
func DefaultServerOptions() *ServerOptions {
	return &ServerOptions{
		LogSensitive: false,
	}
}
