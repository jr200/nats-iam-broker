package broker

import (
	"testing"

	"github.com/jr200-labs/nats-iam-broker/internal/version"
)

func TestResolveServiceVersion(t *testing.T) {
	// Save and restore the package-level ldflags var so tests don't
	// leak state between cases or pollute other tests in the package.
	originalLdflagsVersion := version.Version
	t.Cleanup(func() { version.Version = originalLdflagsVersion })

	tests := []struct {
		name           string
		envValue       string // empty = unset
		ldflagsVersion string
		yamlVersion    string
		want           string
	}{
		{
			name:           "env var wins over everything",
			envValue:       "v9.9.9-from-env",
			ldflagsVersion: "v1.2.3",
			yamlVersion:    "1.0.0",
			want:           "v9.9.9-from-env",
		},
		{
			name:           "ldflags wins when env unset and ldflags non-dev",
			envValue:       "",
			ldflagsVersion: "v1.2.3",
			yamlVersion:    "1.0.0",
			want:           "v1.2.3",
		},
		{
			name:           "yaml fallback when env unset and ldflags is dev",
			envValue:       "",
			ldflagsVersion: "dev",
			yamlVersion:    "1.0.0",
			want:           "1.0.0",
		},
		{
			name:           "literal dev fallback when nothing is set",
			envValue:       "",
			ldflagsVersion: "dev",
			yamlVersion:    "",
			want:           "dev",
		},
		{
			name:           "yaml fallback when ldflags empty (defensive)",
			envValue:       "",
			ldflagsVersion: "",
			yamlVersion:    "1.0.0",
			want:           "1.0.0",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(IAMServiceVersionEnv, tc.envValue)
			version.Version = tc.ldflagsVersion

			got := ResolveServiceVersion(tc.yamlVersion)
			if got != tc.want {
				t.Errorf("ResolveServiceVersion(%q) with env=%q ldflags=%q: got %q, want %q",
					tc.yamlVersion, tc.envValue, tc.ldflagsVersion, got, tc.want)
			}
		})
	}
}
