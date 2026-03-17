//go:build integration

package integration

import (
	"context"

	"github.com/jr200/nats-iam-broker/internal/broker"
)

// brokerStartWithContext calls the broker package's StartWithContext function.
func brokerStartWithContext(ctx context.Context, configFiles []string) error {
	return broker.StartWithContext(ctx, configFiles, &broker.Options{
		LogLevel:  "warn",
		LogFormat: "human",
	}, nil)
}
