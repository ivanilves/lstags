// Package repository provides Repository abstraction to handle Docker repositories.
// This includes parsing and validation of repo specification passed in a text form,
// as well as association of the Docker repository with tags (images) it should contain.
package repository

import (
	"fmt"
	"regexp"
	"strings"
)

// InsecureRegistryEx contains a regex string to match insecure (non-HTTPS) registries
var InsecureRegistryEx = `^(127\..*|::1|localhost)(:[0-9]+)?$`

// RefSpec is the description of a valid Docker repository specification
const RefSpec = "[REGISTRY[:PORT]/]REPOSITORY[:TAG|=TAG1,TAG2,TAGn|~/FILTER_REGEXP/]"

const (
	refWithNothing   = "[REGISTRY[:PORT]/]REPOSITORY"
	refWithSingleTag = "[REGISTRY[:PORT]/]REPOSITORY:TAG"
	refWithManyTags  = "[REGISTRY[:PORT]/]REPOSITORY=TAG1,TAG2,TAGn"
	refWithFilter    = "[REGISTRY[:PORT]/]REPOSITORY~/FILTER_REGEXP/"
)

const (
	registryEx = `[a-z0-9][a-z0-9\-\.]+[a-z0-9](:[0-9]+)?/`
	repoPathEx = `[a-z0-9_][a-z0-9_\-\.\/]+[a-z0-9_]`
	tagEx      = `[a-zA-Z0-9_\-\.]+`
	filterEx   = `\/.*\/`
)

var validRefExprs = map[string]*regexp.Regexp{
	refWithNothing:   regexp.MustCompile(fmt.Sprintf("^(%s)?%s$", registryEx, repoPathEx)),
	refWithSingleTag: regexp.MustCompile(fmt.Sprintf("^(%s)?%s:%s$", registryEx, repoPathEx, tagEx)),
	refWithManyTags:  regexp.MustCompile(fmt.Sprintf("^(%s)?%s=%s(,%s)*$", registryEx, repoPathEx, tagEx, tagEx)),
	refWithFilter:    regexp.MustCompile(fmt.Sprintf("^(%s)?%s~%s$", registryEx, repoPathEx, filterEx)),
}

const defaultRegistry = "registry.hub.docker.com"

// Repository is a parsed, valid Docker repository reference
type Repository struct {
	ref      string
	registry string
	fullRepo string
	repoTags []string
	filterRE *regexp.Regexp
	isSecure bool
	isSingle bool
}

// Ref gets original repository reference string
func (r *Repository) Ref() string {
	return r.ref
}

// Registry gets registry ADDR[:PORT]
func (r *Repository) Registry() string {
	return r.registry
}

// IsDefaultRegistry tells us if we use default registry (DockerHub)
func (r *Repository) IsDefaultRegistry() bool {
	return r.registry == defaultRegistry
}

// Full gives us repository in a "full" form REGISTRY[:PORT]/REPOSITORY
func (r *Repository) Full() string {
	return r.fullRepo
}

// Name is same as Full() but cuts leading REGISTRY[:PORT]/ if we use default registry (DockerHub)
func (r *Repository) Name() string {
	if r.IsDefaultRegistry() {
		return strings.Join(strings.Split(r.Full(), "/")[1:], "/")
	}

	return r.Full()
}

// Path gives us repository path without registry hostname e.g. "library/alpine"
func (r *Repository) Path() string {
	path := strings.Join(strings.Split(r.Full(), "/")[1:], "/")

	if r.IsDefaultRegistry() && !strings.Contains(path, "/") {
		return "library/" + path
	}

	return path
}

// PushPath returns a repository path with a custom path element separator
func (r *Repository) PushPath(pathSeparator string) string {
	path := r.Path()

	path = strings.Join(strings.Split(path, "/"), pathSeparator)

	return path
}

// HasTags tells us if we've specified some concrete tags for this repository
func (r *Repository) HasTags() bool {
	return r.repoTags != nil && len(r.repoTags) != 0
}

// Tags gives us list of tags we specified for this repository
// (It will return `[]string{}` if we have not specified any)
func (r *Repository) Tags() []string {
	if !r.HasTags() {
		return []string{}
	}

	return r.repoTags
}

// HasFilter tells us if we've specified /FILTER/ regexp to match tags for this repository
func (r *Repository) HasFilter() bool {
	return r.filterRE != nil
}

// Filter gives us a string form of /FILTER/ regexp we use to match repository tags
func (r *Repository) Filter() string {
	if !r.HasFilter() {
		return ""
	}

	return r.filterRE.String()
}

// IsSecure tells us if we use secure (HTTPS) connection for this registry/repository
func (r *Repository) IsSecure() bool {
	return r.isSecure
}

// WebSchema tells us we use "http://" or "https://" to connect to this registry/repository
func (r *Repository) WebSchema() string {
	if !r.IsSecure() {
		return "http://"
	}

	return "https://"
}

// IsSingle tells us if we created repo from reference having only single tag specified
func (r *Repository) IsSingle() bool {
	return r.isSingle
}

// MatchTag matches passed tag against repository tag and filter specification
func (r *Repository) MatchTag(tag string) bool {
	return r.isTagSpecified(tag) || r.doesTagMatchesFilter(tag)
}

func (r *Repository) isTagSpecified(tag string) bool {
	if r.HasFilter() {
		return false
	}

	for _, t := range r.Tags() {
		if t == tag {
			return true
		}
	}

	return false
}

func (r *Repository) doesTagMatchesFilter(tag string) bool {
	if !r.HasFilter() {
		return false
	}

	return r.filterRE.MatchString(tag)
}

// PushPrefix generates prefix path for repository in a "push" registry
func (r *Repository) PushPrefix() string {
	allParts := strings.Split(r.Registry(), ":")
	hostPart := allParts[0]

	return "/" + strings.Replace(hostPart, ".", "/", -1) + "/"
}

func validateRef(ref string) (string, error) {
	for spec, re := range validRefExprs {
		if re.MatchString(ref) {
			return spec, nil
		}
	}

	return "", fmt.Errorf(
		"repository reference '%s' failed to match specification: %s", ref, RefSpec,
	)
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

// GetRegistry extracts registry address from the repository reference
func GetRegistry(ref string) string {
	ref = strings.Split(ref, "~")[0]

	if !strings.Contains(ref, "/") {
		return defaultRegistry
	}

	registry := strings.Split(ref, "/")[0]

	if isHostname(registry) {
		return registry
	}

	return defaultRegistry
}

func getFullRef(ref, registry string) string {
	if strings.HasPrefix(ref, registry) {
		return ref
	}

	return registry + "/" + ref
}

// ParseRef takes a string repository reference and transforms it into a Repository structure
func ParseRef(ref string) (*Repository, error) {
	spec, err := validateRef(ref)
	if err != nil {
		return nil, err
	}

	var registry = GetRegistry(ref)

	fullRef := getFullRef(ref, registry)

	var fullRepo string
	var repoTags []string
	var filterRE *regexp.Regexp
	var isSingle bool

	switch spec {
	case refWithNothing:
		fullRepo = fullRef
		filterRE = regexp.MustCompile(".*")
	case refWithSingleTag:
		refParts := strings.Split(fullRef, ":")
		fullRepo = strings.Replace(fullRef, ":"+refParts[len(refParts)-1], "", 1)
		repoTags = []string{refParts[len(refParts)-1]}
		isSingle = true
	case refWithManyTags:
		refParts := strings.Split(fullRef, "=")
		fullRepo = refParts[0]
		repoTags = strings.Split(refParts[1], ",")
		isSingle = true
	case refWithFilter:
		refParts := strings.Split(fullRef, "~")
		fullRepo = refParts[0]
		filterRE = regexp.MustCompile(refParts[1][1 : len(refParts[1])-1])
	default:
		return nil, fmt.Errorf("unknown repository  reference specification: %s", spec)
	}

	return &Repository{
		ref:      ref,
		registry: registry,
		fullRepo: fullRepo,
		repoTags: repoTags,
		filterRE: filterRE,
		isSecure: !regexp.MustCompile(InsecureRegistryEx).MatchString(registry),
		isSingle: isSingle,
	}, nil
}

// ParseRefs is a shorthand for ParseRef to parse multiple repository references at once
func ParseRefs(refs []string) ([]*Repository, error) {
	repos := make([]*Repository, len(refs))

	for i, ref := range refs {
		repo, err := ParseRef(ref)
		if err != nil {
			return nil, err
		}

		repos[i] = repo
	}

	return repos, nil
}
