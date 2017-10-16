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
