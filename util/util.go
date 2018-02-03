package util

import (
	"regexp"
	"strings"
)

// DoesMatch wraps over regexp.MatchString to cowardly escape errors
func DoesMatch(s, ex string) bool {
	matched, err := regexp.MatchString(ex, s)
	if err != nil {
		return false
	}

	return matched
}

// GeneratePathFromHostname generates "/"-delimited path from a hostname[:port]
func GeneratePathFromHostname(hostname string) string {
	allParts := strings.Split(hostname, ":")
	hostPart := allParts[0]

	return "/" + strings.Replace(hostPart, ".", "/", -1)
}
