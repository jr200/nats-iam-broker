package internal

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
)

func WaitForInterrupt() {
	sigch := make(chan os.Signal, 1)
	signal.Notify(sigch, os.Interrupt)
	<-sigch
}

func IgnoreError[T any](val T, _ error) T {
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
	const expectedNumberOfDelimiters = 2
	parts := strings.Split(delim, ",")
	if len(parts) != expectedNumberOfDelimiters {
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
