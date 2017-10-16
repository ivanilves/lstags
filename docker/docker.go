package docker

import (
	"strings"
)

// DefaultRegistry is a registry we use if none could be resolved from image ref
var DefaultRegistry = "registry.hub.docker.com"

// GetRegistry tries to get Docker registry name from a repository or reference
// .. if it is not possible it returns default registry name (usually Docker Hub)
func GetRegistry(repoOrRef string) string {
	r := strings.Split(repoOrRef, "/")[0]

	if isHostname(r) {
		return r
	}

	return DefaultRegistry
}

func isHostname(s string) bool {
	if strings.Contains(s, ".") {
		return true
	}

	if strings.Contains(s, ":") {
		return true
	}

	if s == "localhost" {
		return true
	}

	return false
}
