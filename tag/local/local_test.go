package local

import (
	"testing"
)

func TestFormatRepoNameForPublicRegistry(t *testing.T) {
	const registry = "registry.hub.docker.com"

	expectations := map[string]string{
		"library/nginx": "nginx",
		"hype/cube":     "hype/cube",
	}

	for input, expected := range expectations {
		output := FormatRepoName(input, registry)

		if output != expected {
			t.Fatalf(
				"Got unexpected local repo name: %s => %s\n* Expected: %s",
				input,
				output,
				expected,
			)
		}
	}
}

func TestFormatRepoNameForPrivateRegistry(t *testing.T) {
	const registry = "registry.nerd.io"

	expectations := map[string]string{
		"empollon/nginx":             "registry.nerd.io/empollon/nginx",
		"registry.nerd.io/hype/cube": "registry.nerd.io/hype/cube",
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
