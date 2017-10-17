package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/jessevdk/go-flags"

	"github.com/ivanilves/lstags/auth"
	"github.com/ivanilves/lstags/docker"
	dockerclient "github.com/ivanilves/lstags/docker/client"
	dockerconfig "github.com/ivanilves/lstags/docker/config"
	"github.com/ivanilves/lstags/tag"
	"github.com/ivanilves/lstags/tag/local"
	"github.com/ivanilves/lstags/tag/registry"
	"github.com/ivanilves/lstags/util"
)

// Options represents configuration options we extract from passed command line arguments
type Options struct {
	DockerJSON         string `short:"j" long:"docker-json" default:"~/.docker/config.json" description:"JSON file with credentials" env:"DOCKER_JSON"`
	Pull               bool   `short:"P" long:"pull" description:"Pull Docker images matched by filter (will use local Docker deamon)" env:"PULL"`
	PushRegistry       string `short:"U" long:"push-registry" description:"[Re]Push pulled images to a specified remote registry" env:"PUSH_REGISTRY"`
	PushPrefix         string `short:"R" long:"push-prefix" description:"[Re]Push pulled images with a specified repo path prefix" env:"PUSH_PREFIX"`
	ConcurrentRequests int    `short:"c" long:"concurrent-requests" default:"32" description:"Limit of concurrent requests to the registry" env:"CONCURRENT_REQUESTS"`
	TraceRequests      bool   `short:"T" long:"trace-requests" description:"Trace Docker registry HTTP requests" env:"TRACE_REQUESTS"`
	DoNotFail          bool   `short:"N" long:"do-not-fail" description:"Do not fail on non-critical errors (could be dangerous!)" env:"DO_NOT_FAIL"`
	Version            bool   `short:"V" long:"version" description:"Show version and exit"`
	Positional         struct {
		Repositories []string `positional-arg-name:"REPO1 REPO2 REPOn" description:"Docker repositories to operate on, e.g.: alpine nginx~/1\\.13\\.5$/ busybox~/1.27.2/"`
	} `positional-args:"yes" required:"yes"`
}

var doNotFail = false

func suicide(err error, critical bool) {
	fmt.Printf("%s\n", err.Error())

	if doNotFail || critical {
		os.Exit(1)
	}
}

func parseFlags() (*Options, error) {
	var err error

	o := &Options{}

	_, err = flags.Parse(o)
	if err != nil {
		os.Exit(1) // YES! Just exit!
	}

	if o.Version {
		fmt.Printf("VERSION: %s\n", getVersion())
		os.Exit(0)
	}

	if len(o.Positional.Repositories) == 0 {
		return nil, errors.New("Need at least one repository name, e.g. 'nginx~/^1\\\\.13/' or 'mesosphere/chronos'")
	}

	if o.PushRegistry != "" {
		o.Pull = true
	}

	registry.TraceRequests = o.TraceRequests

	doNotFail = o.DoNotFail

	return o, nil
}

func getVersion() string {
	return VERSION
}

func getAuthorization(t auth.TokenResponse) string {
	return t.Method() + " " + t.Token()
}

func main() {
	o, err := parseFlags()
	if err != nil {
		suicide(err, true)
	}

	dockerConfig, err := dockerconfig.Load(o.DockerJSON)
	if err != nil {
		suicide(err, true)
	}

	const format = "%-12s %-45s %-15s %-25s %s\n"
	fmt.Printf(format, "<STATE>", "<DIGEST>", "<(local) ID>", "<Created At>", "<TAG>")

	repoCount := len(o.Positional.Repositories)
	pullCount := 0
	pushCount := 0

	dc, err := dockerclient.New(dockerConfig)
	if err != nil {
		suicide(err, true)
	}

	type tagResult struct {
		Tags     []*tag.Tag
		Repo     string
		Path     string
		Registry string
	}

	trc := make(chan tagResult, repoCount)

	for _, r := range o.Positional.Repositories {
		go func(r string, o *Options, trc chan tagResult) {
			repository, filter, err := util.SeparateFilterAndRepo(r)
			if err != nil {
				suicide(err, true)
			}

			registryName := docker.GetRegistry(repository)

			repoRegistryName := registry.FormatRepoName(repository, registryName)
			repoLocalName := local.FormatRepoName(repository, registryName)

			username, password, _ := dockerConfig.GetCredentials(registryName)

			tresp, err := auth.NewToken(registryName, repoRegistryName, username, password)
			if err != nil {
				suicide(err, true)
			}

			authorization := getAuthorization(tresp)

			registryTags, err := registry.FetchTags(registryName, repoRegistryName, authorization, o.ConcurrentRequests)
			if err != nil {
				suicide(err, true)
			}

			imageSummaries, err := dc.ListImagesForRepo(repoLocalName)
			if err != nil {
				suicide(err, true)
			}
			localTags, err := local.FetchTags(repoLocalName, imageSummaries)
			if err != nil {
				suicide(err, true)
			}

			sortedKeys, names, joinedTags := tag.Join(registryTags, localTags)

			tags := make([]*tag.Tag, 0)
			for _, key := range sortedKeys {
				name := names[key]

				tg := joinedTags[name]

				if !util.DoesMatch(tg.GetName(), filter) {
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
						err := dc.Pull(ref)
						if err != nil {
							suicide(err, false)
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
						prefix = util.GeneratePathFromHostname(registry)
					}

					srcRef := repo + ":" + tg.GetName()
					dstRef := o.PushRegistry + prefix + "/" + path + ":" + tg.GetName()

					fmt.Printf("PUSHING %s => %s\n", srcRef, dstRef)

					err := dc.Tag(srcRef, dstRef)
					if err != nil {
						suicide(err, true)
					}

					err = dc.Push(dstRef)
					if err != nil {
						suicide(err, false)
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
