package server

import (
	"fmt"
	"os"
	"os/signal"
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
