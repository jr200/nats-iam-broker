package server

// Options holds all configuration options for the server
type Options struct {
	// LogSensitive enables logging of sensitive information (for debugging)
	LogSensitive bool
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
		LogSensitive: false,
	}
}
