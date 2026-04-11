package main

import (
	"fmt"
	"runtime/debug"

	"github.com/jr200-labs/nats-iam-broker/internal/version"
	"github.com/spf13/cobra"
)

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the version of nats-iam-broker",
		Run: func(_ *cobra.Command, _ []string) {
			// Source of truth: internal/version.Version, set via
			// ldflags at build time:
			//   go build -ldflags "-X github.com/jr200-labs/nats-iam-broker/internal/version.Version=v1.2.3"
			// If unset (dev builds), fall back to runtime/debug build
			// info which surfaces the module version when running via
			// `go install` from a tagged commit.
			v := version.Version
			if info, ok := debug.ReadBuildInfo(); ok && v == "dev" {
				if info.Main.Version != "" && info.Main.Version != "(devel)" {
					v = info.Main.Version
				}
			}
			fmt.Println(v)
		},
	}
}
