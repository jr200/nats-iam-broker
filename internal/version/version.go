// Package version exposes the build-time version string for the
// nats-iam-broker binary so it can be read by both the CLI (`version`
// subcommand) and the broker runtime (service self-identification in
// logs, NATS micro registration, OTel resource attributes).
//
// The default value "dev" is overwritten at build time via:
//
//	go build -ldflags "-X github.com/jr200-labs/nats-iam-broker/internal/version.Version=v1.2.3"
//
// The Makefile's `build` target wires this from the VERSION read out of
// pyproject.toml. Goreleaser / CI builds inherit the same flag via the
// reusable workflow in jr200-labs/github-action-templates, which calls
// `make build`.
//
// At runtime, the broker prefers (in order):
//  1. The IAM_SERVICE_VERSION environment variable, if non-empty
//     (lets operators override at deploy time without rebuilding)
//  2. This compiled-in Version, if non-"dev"
//  3. The `service.version` field in the broker YAML config, if present
//  4. The literal string "dev"
//
// See internal/broker.ResolveServiceVersion for the resolution helper.
package version

// Version is the build-time injected version string. Defaults to "dev"
// for local `go run` / `go build` invocations that don't pass ldflags.
var Version = "dev"
