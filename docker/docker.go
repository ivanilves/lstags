package docker

import (
	"strings"

	"github.com/ivanilves/lstags/util"
)

// DefaultRegistry is a registry we use if none could be resolved from image ref
var DefaultRegistry = "registry.hub.docker.com"

// InsecureRegistryEx contains a regex string to match insecure registries
var InsecureRegistryEx = "^(127\\..*|::1|localhost)(:[0-9]+)?$"

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

// GetRepoName gets full repository name
func GetRepoName(repository, registry string) string {
	if registry == "registry.hub.docker.com" {
		if strings.HasPrefix(repository, "library/") {
			return strings.Replace(repository, "library/", "", 1)
		}

		return repository
	}

	if strings.HasPrefix(repository, registry) {
		return repository
	}

	return registry + "/" + repository
}

// GetRepoPath gets repository path
func GetRepoPath(repository, registry string) string {
	if !strings.Contains(repository, "/") {
		return "library/" + repository
	}

	if strings.HasPrefix(repository, registry) {
		return strings.Replace(repository, registry+"/", "", 1)
	}

	return repository
}

// WebSchema tells us if we should use HTTP or HTTPS
func WebSchema(registry string) string {
	if util.DoesMatch(registry, InsecureRegistryEx) {
		return "http://"
	}

	return "https://"
}
