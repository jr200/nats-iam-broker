package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/jr200/nats-iam-broker/internal/broker"
	"github.com/jr200/nats-iam-broker/internal/logging"
	"go.uber.org/zap"
)

var serverOpts *broker.Options

func main() {
	configFiles := parseFlags()

	exitCode := run(configFiles)
	os.Exit(exitCode)
}

func run(configFiles []string) int {
	defer func() { _ = zap.L().Sync() }()

	if err := broker.Start(configFiles, serverOpts); err != nil {
		fmt.Fprintf(os.Stderr, "[service stderr]: %v\n", err)
		return 1
	}
	return 0
}

func parseFlags() []string {
	var logLevel string
	var logHumanReadable bool

	flag.StringVar(&logLevel, "log", "info", "set log-level: disabled, panic, fatal, error, warn, info, debug, trace")
	flag.BoolVar(&logHumanReadable, "log-human", false, "use human-readable logging output")

	serverOpts = broker.DefaultServerOptions()
	flag.BoolVar(&serverOpts.LogSensitive, "log-sensitive", false, "enable sensitive logging (for debugging)")
	flag.BoolVar(&serverOpts.MetricsEnabled, "metrics", false, "enable Prometheus metrics endpoint")
	flag.IntVar(&serverOpts.MetricsPort, "metrics-port", broker.DefaultMetricsPort, "port for the metrics HTTP server")
	flag.Parse()

	configFiles := flag.Args()
	if len(configFiles) == 0 {
		w := flag.CommandLine.Output() // may be os.Stderr - but not necessarily
		fmt.Fprintf(w, "usage: %s [...flags...] config_1.yaml ... config_n.yaml\n", os.Args[0])
		flag.PrintDefaults()
		fmt.Fprintln(w, "")
		os.Exit(1)
	}

	logging.Setup(logLevel, logHumanReadable)

	return configFiles
}
