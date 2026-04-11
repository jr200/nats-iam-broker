package broker

import (
	"os"
	"strings"

	"github.com/jr200-labs/nats-iam-broker/internal/version"
)

// IAMServiceVersionEnv is the environment variable name an operator can
// set to override the version string the broker reports at runtime.
// Used by the Helm chart to inject {{ .Values.image.tag }} so the
// deployed image tag flows through to logs / NATS micro registration /
// OTel resource attributes without requiring a rebuild or YAML edit.
const IAMServiceVersionEnv = "IAM_SERVICE_VERSION"

// ResolveServiceVersion returns the effective version string the broker
// should use for self-identification, picking the first non-empty source
// from this precedence chain:
//
//  1. The IAM_SERVICE_VERSION environment variable (operator override).
//  2. The compile-time injected internal/version.Version (set by ldflags
//     in the Makefile build target). This is the canonical source for
//     CI-built binaries.
//  3. The yamlVersion argument, which the caller passes from the broker
//     YAML config's `service.version` field. Optional in the schema —
//     historically required, kept as a backward-compat fallback.
//  4. The literal string "dev" so callers always get a non-empty value.
//
// Any leading 'v' prefix is stripped from the result before returning.
// This matters because NATS micro.AddService validates the Version field
// as bare semver — `1.3.0` is valid, `v1.3.0` is not. Real-world inputs
// almost always carry the 'v' prefix (git tags, container image tags,
// ldflags-injected `v$(VERSION)`), so the resolver normalises here once
// rather than forcing every caller to remember.
//
// The chain is intentionally ordered so that the runtime environment
// wins over the binary, which wins over static config — matching the
// usual "deploy-time > build-time > config-time" precedence operators
// expect from observability tooling.
func ResolveServiceVersion(yamlVersion string) string {
	return strings.TrimPrefix(rawServiceVersion(yamlVersion), "v")
}

// rawServiceVersion returns the unnormalised version from the precedence
// chain, before the leading-'v' strip. Split out so the trim happens in
// exactly one place and the chain logic stays trivially testable.
func rawServiceVersion(yamlVersion string) string {
	if v := os.Getenv(IAMServiceVersionEnv); v != "" {
		return v
	}
	if version.Version != "" && version.Version != "dev" {
		return version.Version
	}
	if yamlVersion != "" {
		return yamlVersion
	}
	return "dev"
}
