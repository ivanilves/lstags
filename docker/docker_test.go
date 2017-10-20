package docker

import (
	"testing"
)

func TestGetRegistry(t *testing.T) {
	examples := map[string]string{
		"mesosphere/marathon":             DefaultRegistry,
		"bogohost/my/inner/troll":         DefaultRegistry,
		"bogohost/my/inner/troll:1.0.1":   DefaultRegistry,
		"registry.hipsta.io/hype/hotshit": "registry.hipsta.io",
		"localhost/my/image":              "localhost",
		"localhost/my/image:latest":       "localhost",
		"bogohost:5000/mymymy/img":        "bogohost:5000",
		"bogohost:5000/mymymy/img:0.0.1":  "bogohost:5000",
		"bogohost:5000/mymymy/img:edge":   "bogohost:5000",
	}

	for repoOrRef, expectedRegistry := range examples {
		registry := GetRegistry(repoOrRef)

		if registry != expectedRegistry {
			t.Fatalf(
				"Got unexpected Docker registry name '%s' from repo/ref '%s' (expected: '%s')",
				registry,
				repoOrRef,
				expectedRegistry,
			)
		}
	}
}

func TestGetRepoNameForDockerHub(t *testing.T) {
	examples := map[string]string{
		"library/nginx": "nginx",
		"hype/cube":     "hype/cube",
	}

	for input, expected := range examples {
		output := GetRepoName(input, "registry.hub.docker.com")

		if output != expected {
			t.Fatalf(
				"Got unexpected repo name: %s => %s\n* Expected: %s",
				input,
				output,
				expected,
			)
		}
	}
}

func TestGetRepoNameForPrivateRegistry(t *testing.T) {
	const registry = "registry.nerd.io"

	examples := map[string]string{
		"empollon/nginx":             registry + "/empollon/nginx",
		"registry.nerd.io/hype/cube": registry + "/hype/cube",
	}

	for input, expected := range examples {
		output := GetRepoName(input, registry)

		if output != expected {
			t.Fatalf(
				"Got unexpected repo name: %s => %s\n* Expected: %s",
				input,
				output,
				expected,
			)
		}
	}
}

func TestGetRepoPath(t *testing.T) {
	const registry = "registry.nerd.io"

	examples := map[string]string{
		"nginx": "library/nginx",
		"registry.nerd.io/hype/cube": "hype/cube",
		"observability/metrix":       "observability/metrix",
	}

	for input, expected := range examples {
		output := GetRepoPath(input, registry)

		if output != expected {
			t.Fatalf(
				"Got unexpected repo path: %s => %s\n* Expected: %s",
				input,
				output,
				expected,
			)
		}
	}
}

func TestWebSchema(t *testing.T) {
	examples := map[string]string{
		"localhost":        "http://",
		"localhost:4000":   "http://",
		"127.0.0.1":        "http://",
		"127.0.0.1:5000":   "http://",
		"remotehost":       "https://",
		"reg.hype.io":      "https://",
		"reg.hype.io:3128": "https://",
	}

	for input, expected := range examples {
		output := WebSchema(input)

		if output != expected {
			t.Fatalf(
				"Got unexpected schema '%s' for registry hostname '%s', while expecting for '%s'",
				output,
				input,
				expected,
			)
		}
	}

	InsecureRegistryEx = ".*"

	for input := range examples {
		output := WebSchema(input)

		if output != "http://" {
			t.Fatalf(
				"Expected schema 'http://' for registry hostname '%s' in this case",
				input,
			)
		}
	}

	InsecureRegistryEx = "i.do.not.match.anything"

	for input := range examples {
		output := WebSchema(input)

		if output != "https://" {
			t.Fatalf(
				"Expected schema 'https://' for registry hostname '%s' in this case",
				input,
			)
		}
	}
}
