package main

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/jessevdk/go-flags"

	"github.com/ivanilves/lstags/auth"
	"github.com/ivanilves/lstags/tag"
	"github.com/ivanilves/lstags/tag/local"
	"github.com/ivanilves/lstags/tag/registry"
)

type options struct {
	Registry      string `short:"r" long:"registry" default:"registry.hub.docker.com" description:"Docker registry to use" env:"REGISTRY"`
	Username      string `short:"u" long:"username" default:"" description:"Docker registry username" env:"USERNAME"`
	Password      string `short:"p" long:"password" default:"" description:"Docker registry password" env:"PASSWORD"`
	Concurrency   int    `short:"c" long:"concurrency" default:"32" description:"Concurrent request limit while querying registry" env:"CONCURRENCY"`
	TraceRequests bool   `short:"T" long:"trace-requests" description:"Trace HTTP requests to registry" env:"TRACE_REQUESTS"`
	Positional    struct {
		Repository string `positional-arg-name:"REPOSITORY" description:"Docker repository to list tags from"`
	} `positional-args:"yes"`
}

func suicide(err error) {
	fmt.Printf("%s\n", err.Error())
	os.Exit(1)
}

func shortify(str string, length int) string {
	if len(str) <= length {
		return str
	}

	return str[0:length]
}

func getRepoRegistryName(repository, registry string) string {
	if !strings.Contains(repository, "/") {
		return "library/" + repository
	}

	if strings.HasPrefix(repository, registry) {
		return strings.Replace(repository, registry+"/", "", 1)
	}

	return repository
}

func getRepoLocalName(repository, registry string) string {
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

func getAuthorization(t auth.TokenResponse) string {
	return t.Method() + " " + t.Token()
}

func main() {
	o := options{}

	_, err := flags.Parse(&o)
	if err != nil {
		suicide(err)
	}
	if o.Positional.Repository == "" {
		suicide(errors.New("You should provide a repository name, e.g. 'nginx' or 'mesosphere/chronos'"))
	}
	registry.TraceRequests = o.TraceRequests
	repoRegistryName := getRepoRegistryName(o.Positional.Repository, o.Registry)
	repoLocalName := getRepoLocalName(o.Positional.Repository, o.Registry)

	tresp, err := auth.NewToken(o.Registry, repoRegistryName, o.Username, o.Password)
	if err != nil {
		suicide(err)
	}

	authorization := getAuthorization(tresp)

	registryTags, err := registry.FetchTags(o.Registry, repoRegistryName, authorization, o.Concurrency)
	if err != nil {
		suicide(err)
	}
	localTags, err := local.FetchTags(repoLocalName)
	if err != nil {
		suicide(err)
	}

	sortKeys, joinedTags := tag.Join(registryTags, localTags)

	const format = "%-12s %-45s %-15s %s\n"
	fmt.Printf(format, "<STATE>", "<DIGEST>", "<(local) ID>", "<TAG>")
	for _, sortKey := range sortKeys {
		tg := joinedTags[sortKey]

		fmt.Printf(
			format,
			tg.GetState(),
			shortify(tg.GetDigest(), 40),
			tg.GetImageID(),
			repoLocalName+":"+tg.GetName(),
		)
	}
}
