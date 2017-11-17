package util

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

const repoRefDesc = "[REGISTRY[:PORT]/]REPOSITORY[~/FILTER_REGEXP/][=TAG1,TAG2,TAGn]"
const repoRefExpr = "^[a-z0-9_][a-z0-9_\\-\\.\\/:]+[a-z0-9_](~\\/.*\\/)?(=.*)?$"

const tagNameExpr = "^[a-z0-9_\\.\\-]+$"

// ParseRepoRef extracts:
// * repository name
// * tag filter
// * assumed tag names (if any)
// from the repo reference string passed
func ParseRepoRef(repoRef string) (string, string, []string, error) {
	if !DoesMatch(repoRef, repoRefExpr) {
		err := errors.New(repoRef + " is not a valid repo spec: " + repoRefDesc)

		return "", "", nil, err
	}

	repoWithFilter, assumedTagNames, err := SeparateAssumedTagNamesAndRepo(
		repoRef,
	)
	if err != nil {
		return "", "", nil, err
	}

	repository, filter, err := SeparateFilterAndRepo(repoWithFilter)
	if err != nil {
		return repoWithFilter, "", assumedTagNames, err
	}

	for _, name := range assumedTagNames {
		if !DoesMatch(name, filter) {
			reason := fmt.Sprintf(
				"Assumed tag '%s' does not match filter '%s', repo ref: %s",
				name,
				filter,
				repoRef,
			)

			return repository, filter, assumedTagNames, errors.New(reason)
		}
	}

	return repository, filter, assumedTagNames, nil
}

// SeparateAssumedTagNamesAndRepo separates repo ref from assumed tag names
func SeparateAssumedTagNamesAndRepo(repoRef string) (string, []string, error) {
	var reason string
	parts := strings.Split(repoRef, "=")

	repoWithFilter := parts[0]

	if len(parts) < 2 {
		return repoWithFilter, nil, nil
	}

	if len(parts) > 2 {
		reason = "Unable to trim assumed tags from repo ref (too many '='!): "

		return "", nil, errors.New(reason + repoRef)
	}

	assumedTagNames := strings.Split(parts[1], ",")

	for _, name := range assumedTagNames {
		if !DoesMatch(name, tagNameExpr) {
			err := errors.New("Invalid tag name: " + name)

			return repoWithFilter, nil, err
		}
	}

	return repoWithFilter, assumedTagNames, nil
}

// SeparateFilterAndRepo separates repository name from optional regex filter
func SeparateFilterAndRepo(repoWithFilter string) (string, string, error) {
	var reason string

	parts := strings.Split(repoWithFilter, "~")

	repository := parts[0]

	if len(parts) < 2 {
		return repository, ".*", nil
	}

	if len(parts) > 2 {
		reason = "Unable to trim filter from repository (too many '~'!): "

		return "", "", errors.New(reason + repoWithFilter)
	}

	f := parts[1]

	if !strings.HasPrefix(f, "/") || !strings.HasSuffix(f, "/") {
		reason = "Filter should be passed in a form: /REGEXP/: "

		return "", "", errors.New(reason + repoWithFilter)
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
