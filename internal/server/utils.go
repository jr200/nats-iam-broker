package server

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
)

func waitForInterrupt() {
	sigch := make(chan os.Signal, 1)
	signal.Notify(sigch, os.Interrupt)
	<-sigch
}

func IgnoreError[T any](val T, err error) T {
	return val
}

func CollapseError(val any, err error) string {
	if err != nil {
		return err.Error()
	}

	switch v := val.(type) {
	case string:
		return v
	case fmt.Stringer:
		return v.String()
	default:
		return fmt.Sprintf("%v", v)
	}
}

func ParseDelimiters(delim string) (string, string) {

	parts := strings.Split(delim, ",")
	if len(parts) != 2 {
		fmt.Fprintf(os.Stderr, "Invalid 'delim' format. Expected 'left,right'.\n")
		os.Exit(1)
	}

	leftDelim, rightDelim := parts[0], parts[1]
	if leftDelim == "" || rightDelim == "" {
		fmt.Fprintf(os.Stderr, "Invalid 'delim' format. Neither delimiter can be empty. Expected 'left,right'.\n")
		os.Exit(1)
	}

	return parts[0], parts[1]
}
