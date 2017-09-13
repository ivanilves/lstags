package registry

import (
	"testing"
)

func TestFormatRepoName(t *testing.T) {
	const registry = "registry.nerd.io"

	expectations := map[string]string{
		"nginx": "library/nginx",
		"registry.nerd.io/hype/cube": "hype/cube",
		"observability/metrix":       "observability/metrix",
	}

	for input, expected := range expectations {
		output := FormatRepoName(input, registry)

		if output != expected {
			t.Fatalf(
				"Got unexpected registry repo name: %s => %s\n* Expected: %s",
				input,
				output,
				expected,
			)
		}
	}
}
