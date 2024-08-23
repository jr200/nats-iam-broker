package main

import (
	"flag"
	"fmt"
	"os"

	server "github.com/jr200/nats-iam-broker/internal/server"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func main() {
	configFiles, args := parseFlags()

	if err := server.Start(configFiles, args); err != nil {
		fmt.Fprintf(os.Stderr, "[service stderr]: %v\n", err)
		os.Exit(1)
	}
}

func parseFlags() ([]string, server.ConfigParams) {
	var logLevel string
	var logHumanReadable bool
	var delim string

	flag.StringVar(&logLevel, "log", "info", "set log-level: disabled, panic, fatal, error, warn, info, debug, trace")
	flag.BoolVar(&logHumanReadable, "log-human", false, "use human-readable logging output")
	flag.StringVar(&delim, "delim", "{{,}}", "set delimiters in format 'left,right'")
	flag.Parse()

	leftDelim, rightDelim := server.ParseDelimiters(delim)

	configs := flag.Args()
	if len(configs) == 0 {
		w := flag.CommandLine.Output() // may be os.Stderr - but not necessarily
		fmt.Fprintf(w, "usage: %s [...flags...] config_1.yaml ... config_n.yaml\n", os.Args[0])
		flag.PrintDefaults()
		fmt.Fprintln(w, "")
		os.Exit(1)
	}

	configureLogging(logLevel, logHumanReadable)

	args := server.ConfigParams{
		LeftDelim:  leftDelim,
		RightDelim: rightDelim,
	}

	return configs, args
}

func configureLogging(logLevel string, logHumanReadable bool) {
	level, err := zerolog.ParseLevel(logLevel)
	if err != nil {
		level = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(level)

	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	if logHumanReadable {
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stdout})
	}
}
