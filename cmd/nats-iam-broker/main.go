package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/jr200/nats-iam-broker/internal/broker"
	"go.uber.org/zap"
)

var serverOpts *broker.Options

func main() {
	if len(os.Args) > 1 && os.Args[1] == "decrypt" {
		os.Exit(runDecrypt(os.Args[2:]))
	}

	configFiles, cliFlags := parseFlags()

	exitCode := run(configFiles, cliFlags)
	os.Exit(exitCode)
}

func run(configFiles []string, cliFlags map[string]bool) int {
	defer func() { _ = zap.L().Sync() }()

	if err := broker.Start(configFiles, serverOpts, cliFlags); err != nil {
		fmt.Fprintf(os.Stderr, "[service stderr]: %v\n", err)
		return 1
	}
	return 0
}

func parseFlags() ([]string, map[string]bool) {
	var logLevel string
	var logHumanReadable bool

	flag.StringVar(&logLevel, "log", "info", "set log-level: disabled, panic, fatal, error, warn, info, debug, trace")
	flag.BoolVar(&logHumanReadable, "log-human", false, "use human-readable logging output")

	serverOpts = broker.DefaultServerOptions()
	flag.BoolVar(&serverOpts.LogSensitive, "log-sensitive", false, "enable sensitive logging (for debugging)")
	flag.BoolVar(&serverOpts.MetricsEnabled, "metrics", false, "enable Prometheus metrics endpoint")
	flag.IntVar(&serverOpts.MetricsPort, "metrics-port", broker.DefaultMetricsPort, "port for the metrics HTTP server")
	flag.BoolVar(&serverOpts.WatchConfig, "watch", false, "enable hot-reload of config files via file watching")
	flag.Parse()

	// Map CLI flag values into the Options struct for MergeOptions
	serverOpts.LogLevel = logLevel
	if logHumanReadable {
		serverOpts.LogFormat = "human"
	} else {
		serverOpts.LogFormat = "json"
	}

	// Collect which flags were explicitly set on the command line
	cliFlags := make(map[string]bool)
	flag.Visit(func(f *flag.Flag) {
		cliFlags[f.Name] = true
	})

	configFiles := flag.Args()
	if len(configFiles) == 0 {
		w := flag.CommandLine.Output() // may be os.Stderr - but not necessarily
		fmt.Fprintf(w, "usage: %s [...flags...] config_1.yaml ... config_n.yaml\n", os.Args[0])
		flag.PrintDefaults()
		fmt.Fprintln(w, "")
		os.Exit(1)
	}

	return configFiles, cliFlags
}
