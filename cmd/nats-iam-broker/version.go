package main

import (
	"fmt"
	"runtime/debug"

	"github.com/spf13/cobra"
)

// version is set via ldflags at build time: -ldflags "-X main.version=v1.0.0"
var version = "dev"

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the version of nats-iam-broker",
		Run: func(_ *cobra.Command, _ []string) {
			v := version
			if info, ok := debug.ReadBuildInfo(); ok && v == "dev" {
				if info.Main.Version != "" && info.Main.Version != "(devel)" {
					v = info.Main.Version
				}
			}
			fmt.Println(v)
		},
	}
}
