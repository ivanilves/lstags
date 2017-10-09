package app

import (
	"strings"
)

// GenerateRegistryPrefix generates destination Docker registry prefix path from the source registry name
func GenerateRegistryPrefix(registry string) string {
	allParts := strings.Split(registry, ":")
	hostname := allParts[0]

	return "/" + strings.Replace(hostname, ".", "/", -1)
}
