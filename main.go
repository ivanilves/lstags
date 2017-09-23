package main

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/jessevdk/go-flags"

	"github.com/ivanilves/lstags/auth"
	"github.com/ivanilves/lstags/docker/jsonconfig"
	"github.com/ivanilves/lstags/tag"
	"github.com/ivanilves/lstags/tag/local"
	"github.com/ivanilves/lstags/tag/registry"
)

type options struct {
	DefaultRegistry string `short:"r" long:"default-registry" default:"registry.hub.docker.com" description:"Docker registry to use by default" env:"DEFAULT_REGISTRY"`
	Username        string `short:"u" long:"username" default:"" description:"Docker registry username" env:"USERNAME"`
	Password        string `short:"p" long:"password" default:"" description:"Docker registry password" env:"PASSWORD"`
	DockerJSON      string `shord:"j" long:"docker-json" default:"~/.docker/config.json" env:"DOCKER_JSON"`
	Concurrency     int    `short:"c" long:"concurrency" default:"32" description:"Concurrent request limit while querying registry" env:"CONCURRENCY"`
	TraceRequests   bool   `short:"T" long:"trace-requests" description:"Trace registry HTTP requests" env:"TRACE_REQUESTS"`
	Version         bool   `short:"V" long:"version" description:"Show version and exit"`
	Positional      struct {
		Repository string `positional-arg-name:"REPOSITORY" description:"Docker repository to list tags from"`
	} `positional-args:"yes"`
}

func suicide(err error) {
	fmt.Printf("%s\n", err.Error())
	os.Exit(1)
}

func getVersion() string {
	return VERSION
}

func trimFilter(repoWithFilter string) (string, string, error) {
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

func matchesFilter(s, filter string) bool {
	matched, err := regexp.MatchString(filter, s)
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

func getRegistryName(repository, defaultRegistry string) string {
	r := strings.Split(repository, "/")[0]

	if isHostname(r) {
		return r
	}

	return defaultRegistry
}

func assignCredentials(registry, passedUsername, passedPassword, dockerJSON string) (string, string, error) {
	useDefaultDockerJSON := dockerJSON == "~/.docker/config.json"
	areCredentialsPassed := passedUsername != "" && passedPassword != ""

	c, err := jsonconfig.Load(dockerJSON)
	if err != nil {
		if useDefaultDockerJSON {
			return passedUsername, passedPassword, nil
		}

		return "", "", err
	}

	username, password, defined := c.GetCredentials(registry)
	if !defined || areCredentialsPassed {
		return passedUsername, passedPassword, nil
	}

	return username, password, nil
}

func getAuthorization(t auth.TokenResponse) string {
	return t.Method() + " " + t.Token()
}

func main() {
	o := options{}

	_, err := flags.Parse(&o)
	if err != nil {
		suicide(err)
	}
	if o.Version {
		println(getVersion())
		os.Exit(0)
	}
	if o.Positional.Repository == "" {
		suicide(errors.New("You should provide a repository name, e.g. 'nginx' or 'mesosphere/chronos'"))
	}

	registry.TraceRequests = o.TraceRequests

	repository, filter, err := trimFilter(o.Positional.Repository)
	if err != nil {
		suicide(err)
	}

	registryName := getRegistryName(repository, o.DefaultRegistry)

	repoRegistryName := registry.FormatRepoName(repository, registryName)
	repoLocalName := local.FormatRepoName(repository, registryName)

	username, password, err := assignCredentials(registryName, o.Username, o.Password, o.DockerJSON)
	if err != nil {
		suicide(err)
	}

	tresp, err := auth.NewToken(registryName, repoRegistryName, username, password)
	if err != nil {
		suicide(err)
	}

	authorization := getAuthorization(tresp)

	registryTags, err := registry.FetchTags(registryName, repoRegistryName, authorization, o.Concurrency)
	if err != nil {
		suicide(err)
	}
	localTags, err := local.FetchTags(repoLocalName)
	if err != nil {
		suicide(err)
	}

	sortedKeys, names, joinedTags := tag.Join(registryTags, localTags)

	const format = "%-12s %-45s %-15s %-25s %s\n"
	fmt.Printf(format, "<STATE>", "<DIGEST>", "<(local) ID>", "<Created At>", "<TAG>")
	for _, key := range sortedKeys {
		name := names[key]

		tg := joinedTags[name]

		if !matchesFilter(tg.GetName(), filter) {
			continue
		}

		fmt.Printf(
			format,
			tg.GetState(),
			tg.GetShortDigest(),
			tg.GetImageID(),
			tg.GetCreatedString(),
			repoLocalName+":"+tg.GetName(),
		)
	}
}
