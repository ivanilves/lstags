package main

import (
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/jessevdk/go-flags"

	"github.com/ivanilves/lstags/app"
	"github.com/ivanilves/lstags/auth"
	"github.com/ivanilves/lstags/docker/jsonconfig"
	"github.com/ivanilves/lstags/tag"
	"github.com/ivanilves/lstags/tag/local"
	"github.com/ivanilves/lstags/tag/registry"
)

type options struct {
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
	Version            bool   `short:"V" long:"version" description:"Show version and exit"`
	Positional         struct {
		Repositories []string `positional-arg-name:"REPO1 REPO2 REPOn" description:"Docker repositories to operate on, e.g.: alpine nginx~/1\\.13\\.5$/ busybox~/1.27.2/"`
	} `positional-args:"yes" required:"yes"`
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

func getPullAuth(username, password string) string {
	if username == "" && password == "" {
		return ""
	}

	jsonString := fmt.Sprintf("{ \"username\": \"%s\", \"password\": \"%s\" }", username, password)

	return base64.StdEncoding.EncodeToString([]byte(jsonString))
}

func main() {
	o := options{}

	_, err := flags.Parse(&o)
	if err != nil {
		os.Exit(1)
	}
	if o.Version {
		fmt.Printf("VERSION: %s\n", getVersion())
		os.Exit(0)
	}
	if len(o.Positional.Repositories) == 0 {
		suicide(errors.New("Need at least one repository name, e.g. 'nginx~/^1\\\\.13/' or 'mesosphere/chronos'"))
	}

	if o.PushRegistry != "" {
		o.Pull = true
	}

	if o.InsecureRegistry {
		auth.WebSchema = "http://"
		registry.WebSchema = "http://"
	}

	registry.TraceRequests = o.TraceRequests

	const format = "%-12s %-45s %-15s %-25s %s\n"
	fmt.Printf(format, "<STATE>", "<DIGEST>", "<(local) ID>", "<Created At>", "<TAG>")

	repoCount := len(o.Positional.Repositories)
	pullCount := 0
	pushCount := 0

	pullAuths := make(map[string]string)

	var pushAuth string
	if o.PushRegistry != "" {
		pushUsername, pushPassword, err := assignCredentials(o.PushRegistry, o.Username, o.Password, o.DockerJSON)
		if err != nil {
			suicide(err)
		}

		pushAuth = getPullAuth(pushUsername, pushPassword)
	}

	type tagResult struct {
		Tags     []*tag.Tag
		Repo     string
		Path     string
		Registry string
	}

	trc := make(chan tagResult, repoCount)

	for _, r := range o.Positional.Repositories {
		go func(r string, o options, trc chan tagResult) {
			repository, filter, err := trimFilter(r)
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

			pullAuths[repoLocalName] = getPullAuth(username, password)

			tresp, err := auth.NewToken(registryName, repoRegistryName, username, password)
			if err != nil {
				suicide(err)
			}

			authorization := getAuthorization(tresp)

			registryTags, err := registry.FetchTags(registryName, repoRegistryName, authorization, o.ConcurrentRequests)
			if err != nil {
				suicide(err)
			}
			localTags, err := local.FetchTags(repoLocalName)
			if err != nil {
				suicide(err)
			}

			sortedKeys, names, joinedTags := tag.Join(registryTags, localTags)

			tags := make([]*tag.Tag, 0)
			for _, key := range sortedKeys {
				name := names[key]

				tg := joinedTags[name]

				if !matchesFilter(tg.GetName(), filter) {
					continue
				}

				if tg.NeedsPull() {
					pullCount++
				}
				pushCount++

				tags = append(tags, tg)
			}

			trc <- tagResult{Tags: tags, Repo: repoLocalName, Path: repoRegistryName, Registry: registryName}
		}(r, o, trc)
	}

	tagResults := make([]tagResult, repoCount)
	repoNumber := 0
	for tr := range trc {
		repoNumber++
		tagResults = append(tagResults, tr)
		if repoNumber >= repoCount {
			close(trc)
		}
	}

	for _, tr := range tagResults {
		for _, tg := range tr.Tags {
			fmt.Printf(
				format,
				tg.GetState(),
				tg.GetShortDigest(),
				tg.GetImageID(),
				tg.GetCreatedString(),
				tr.Repo+":"+tg.GetName(),
			)
		}
	}

	if o.Pull {
		done := make(chan bool, pullCount)

		for _, tr := range tagResults {
			go func(tags []*tag.Tag, repo string, done chan bool) {
				for _, tg := range tags {
					if tg.NeedsPull() {
						ref := repo + ":" + tg.GetName()

						fmt.Printf("PULLING %s\n", ref)
						err := local.Pull(ref, pullAuths[repo])
						if err != nil {
							suicide(err)
						}

						done <- true
					}

				}
			}(tr.Tags, tr.Repo, done)
		}

		pullNumber := 0
		if pullCount > 0 {
			for range done {
				pullNumber++

				if pullNumber >= pullCount {
					close(done)
				}
			}
		}
	}

	if o.Pull && o.PushRegistry != "" {
		done := make(chan bool, pullCount)

		for _, tr := range tagResults {
			go func(tags []*tag.Tag, repo, path, registry string, done chan bool) {
				for _, tg := range tags {
					prefix := o.PushPrefix
					if prefix == "" {
						prefix = app.GeneratePathFromHostname(registry)
					}

					srcRef := repo + ":" + tg.GetName()
					dstRef := o.PushRegistry + prefix + "/" + path + ":" + tg.GetName()

					fmt.Printf("PUSHING %s => %s\n", srcRef, dstRef)

					err := local.Tag(srcRef, dstRef)
					if err != nil {
						suicide(err)
					}

					err = local.Push(dstRef, pushAuth)
					if err != nil {
						suicide(err)
					}

					done <- true
				}
			}(tr.Tags, tr.Repo, tr.Path, tr.Registry, done)
		}

		pushNumber := 0
		if pushCount > 0 {
			for range done {
				pushNumber++

				if pushNumber >= pushCount {
					close(done)
				}
			}
		}
	}

}
