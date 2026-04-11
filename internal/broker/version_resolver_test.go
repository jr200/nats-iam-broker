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
			name:           "env var wins over everything (v-prefix stripped)",
			envValue:       "v9.9.9-from-env",
			ldflagsVersion: "v1.2.3",
			yamlVersion:    "1.0.0",
			want:           "9.9.9-from-env",
		},
		{
			name:           "ldflags wins when env unset (v-prefix stripped)",
			envValue:       "",
			ldflagsVersion: "v1.2.3",
			yamlVersion:    "1.0.0",
			want:           "1.2.3",
		},
		{
			name:           "yaml fallback when env unset and ldflags is dev",
			envValue:       "",
			ldflagsVersion: "dev",
			yamlVersion:    "1.0.0",
			want:           "1.0.0",
		},
		{
			name:           "yaml without v-prefix passes through unchanged",
			envValue:       "",
			ldflagsVersion: "dev",
			yamlVersion:    "2.5.1",
			want:           "2.5.1",
		},
		{
			name:           "yaml WITH v-prefix gets stripped too",
			envValue:       "",
			ldflagsVersion: "dev",
			yamlVersion:    "v2.5.1",
			want:           "2.5.1",
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
		{
			name:           "env var without v-prefix passes through unchanged",
			envValue:       "1.2.3-rc.1",
			ldflagsVersion: "dev",
			yamlVersion:    "",
			want:           "1.2.3-rc.1",
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
