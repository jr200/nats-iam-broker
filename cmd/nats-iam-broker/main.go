package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func main() {
	rootCmd := newRootCmd()
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "nats-iam-broker",
		Short: "NATS IAM Broker — OIDC-to-NATS auth callout service",
		Long: `nats-iam-broker is a NATS auth callout microservice that validates
OIDC tokens from identity providers and mints short-lived NATS JWTs
with permissions defined by RBAC role bindings.`,
		SilenceUsage: true,
	}

	root.AddCommand(newServeCmd())
	root.AddCommand(newDecryptCmd())
	root.AddCommand(newVersionCmd())

	return root
}
