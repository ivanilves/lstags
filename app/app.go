package app

import (
	"errors"
	"os"
	"regexp"
	"strings"

	"github.com/jessevdk/go-flags"
)

const dockerHub = "registry.hub.docker.com"

// Options represents configuration options we extract from passed command line arguments
type Options struct {
	DefaultRegistry    string `short:"r" long:"default-registry" default:"registry.hub.docker.com" description:"Default Docker registry to use" env:"DEFAULT_REGISTRY"`
	DockerJSON         string `short:"j" long:"docker-json" default:"~/.docker/config.json" description:"JSON file with credentials (use it, please <3)" env:"DOCKER_JSON"`
	Username           string `short:"u" long:"username" default:"" description:"Override Docker registry username (not recommended, please use JSON file)" env:"USERNAME"`
	Password           string `short:"p" long:"password" default:"" description:"Override Docker registry password (not recommended, please use JSON file)" env:"PASSWORD"`
	ConcurrentRequests int    `short:"c" long:"concurrent-requests" default:"32" description:"Limit of concurrent requests to the registry" env:"CONCURRENT_REQUESTS"`
	Pull               bool   `short:"P" long:"pull" description:"Pull Docker images matched by filter (will use local Docker deamon)" env:"PULL"`
	PushRegistry       string `short:"U" long:"push-registry" description:"[Re]Push pulled images to a specified remote registry" env:"PUSH_REGISTRY"`
	PushPrefix         string `short:"R" long:"push-prefix" description:"[Re]Push pulled images with a specified repo path prefix" env:"PUSH_PREFIX"`
	InsecureRegistry   bool   `short:"i" long:"insecure-registry" description:"Use insecure plain-HTTP connection to registries (not recommended!)" env:"INSECURE_REGISTRY"`
	TraceRequests      bool   `short:"T" long:"trace-requests" description:"Trace Docker registry HTTP requests" env:"TRACE_REQUESTS"`
	DoNotFail          bool   `short:"N" long:"do-not-fail" description:"Do not fail on errors (could be dangerous!)" env:"DO_NOT_FAIL"`
	Version            bool   `short:"V" long:"version" description:"Show version and exit"`
	Positional         struct {
		Repositories []string `positional-arg-name:"REPO1 REPO2 REPOn" description:"Docker repositories to operate on, e.g.: alpine nginx~/1\\.13\\.5$/ busybox~/1.27.2/"`
	} `positional-args:"yes" required:"yes"`
}

// ParseFlags parses command line arguments and applies some additional post-processing
func ParseFlags() (*Options, error) {
	var err error

	o := &Options{}

	_, err = flags.Parse(o)
	if err != nil {
		os.Exit(1)
	}

	err = o.postprocess()
	if err != nil {
		return nil, err
	}

	return o, nil
}

func (o *Options) postprocess() error {
	if !o.Version && len(o.Positional.Repositories) == 0 {
		return errors.New("Need at least one repository name, e.g. 'nginx~/^1\\\\.13/' or 'mesosphere/chronos'")
	}

	if o.PushRegistry != "" {
		o.Pull = true
	}

	return nil
}

// GetWebSchema gets web schema we will use to talk to Docker registry (HTTP||HTTPS)
func (o *Options) GetWebSchema() string {
	if o.InsecureRegistry {
		return "http://"
	}

	return "https://"
}

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

// GetRegistryNameFromRepo tries to get Docker registry name from repository name
// .. if it is not possible it returns default registry name (usually Docker Hub)
func GetRegistryNameFromRepo(repository, defaultRegistry string) string {
	r := strings.Split(repository, "/")[0]

	if isHostname(r) {
		return r
	}

	return defaultRegistry
}

// GeneratePathFromHostname generates "/"-delimited path from a hostname[:port]
func GeneratePathFromHostname(hostname string) string {
	allParts := strings.Split(hostname, ":")
	hostPart := allParts[0]

	return "/" + strings.Replace(hostPart, ".", "/", -1)
}
