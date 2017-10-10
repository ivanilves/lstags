package app

import (
	"strings"
)

// GeneratePathFromHostname generates "/"-delimited path from a hostname[:port]
func GeneratePathFromHostname(hostname string) string {
	allParts := strings.Split(hostname, ":")
	hostPart := allParts[0]

	return "/" + strings.Replace(hostPart, ".", "/", -1)
}
