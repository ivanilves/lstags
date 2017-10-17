package util

import (
	"errors"
	"regexp"
	"strings"
)

// SeparateFilterAndRepo separates repository name from optional regex filter
func SeparateFilterAndRepo(repoWithFilter string) (string, string, error) {
	parts := strings.Split(repoWithFilter, "~")

	repository := parts[0]

	if len(parts) < 2 {
		return repository, ".*", nil
	}

	if len(parts) > 2 {
		return "", "", errors.New("Unable to trim filter from repository (too many '~'!): " + repoWithFilter)
	}

	f := parts[1]

	if !strings.HasPrefix(f, "/") || !strings.HasSuffix(f, "/") {
		return "", "", errors.New("Filter should be passed in a form: /REGEXP/")
	}

	filter := f[1 : len(f)-1]

	return repository, filter, nil
}

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
