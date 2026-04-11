package main

import (
	"fmt"
	"os"

	"github.com/jr200-labs/nats-iam-broker/internal/broker"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"go.uber.org/zap"
)

func newServeCmd() *cobra.Command {
	opts := broker.DefaultServerOptions()

	cmd := &cobra.Command{
		Use:   "serve [flags] config1.yaml [config2.yaml ...]",
		Short: "Start the IAM broker service",
		Long: `Start the NATS IAM broker auth callout service.

One or more YAML configuration files must be provided. Files are merged
in order: maps merge recursively, arrays concatenate, and later primitive
values override earlier ones. Glob patterns are supported.`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			defer func() { _ = zap.L().Sync() }()

			// Collect which flags were explicitly set on the command line
			cliFlags := make(map[string]bool)
			cmd.Flags().Visit(func(f *pflag.Flag) {
				cliFlags[f.Name] = true
			})

			if err := broker.Start(args, opts, cliFlags); err != nil {
				fmt.Fprintf(os.Stderr, "[service stderr]: %v\n", err)
				return err
			}
			return nil
		},
	}

	// Logging flags
	flags := cmd.Flags()
	flags.StringVar(&opts.LogLevel, "log-level", "info", "set log level: disabled, panic, fatal, error, warn, info, debug, trace")
	flags.StringVar(&opts.LogFormat, "log-format", "json", "set log format: json, human")
	flags.BoolVar(&opts.LogSensitive, "log-sensitive", false, "enable sensitive logging (for debugging)")

	// Metrics flags
	flags.BoolVar(&opts.MetricsEnabled, "metrics", false, "enable Prometheus metrics endpoint")
	flags.IntVar(&opts.MetricsPort, "metrics-port", broker.DefaultMetricsPort, "port for the metrics HTTP server")

	// Config flags
	flags.BoolVar(&opts.WatchConfig, "watch", false, "enable hot-reload of config files via file watching")

	return cmd
}
