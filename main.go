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
	"github.com/ivanilves/lstags/tag/remote"
	"github.com/ivanilves/lstags/util"
)

// Options represents configuration options we extract from passed command line arguments
type Options struct {
	DockerJSON         string `short:"j" long:"docker-json" default:"~/.docker/config.json" description:"JSON file with credentials" env:"DOCKER_JSON"`
	Pull               bool   `short:"p" long:"pull" description:"Pull Docker images matched by filter (will use local Docker deamon)" env:"PULL"`
	PushRegistry       string `short:"r" long:"push-registry" description:"[Re]Push pulled images to a specified remote registry" env:"PUSH_REGISTRY"`
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

	remote.TraceRequests = o.TraceRequests

	doNotFail = o.DoNotFail

	return o, nil
}

func getVersion() string {
	return VERSION
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

	dc, err := dockerclient.New(dockerConfig)
	if err != nil {
		suicide(err, true)
	}

	const format = "%-12s %-45s %-15s %-25s %s\n"
	fmt.Printf(format, "<STATE>", "<DIGEST>", "<(local) ID>", "<Created At>", "<TAG>")

	repoCount := len(o.Positional.Repositories)
	pullCount := 0
	pushCount := 0

	tcc := make(chan tag.Collection, repoCount)

	for _, repoWithFilter := range o.Positional.Repositories {
		go func(repoWithFilter string, concurrentRequests int, tcc chan tag.Collection) {
			repository, filter, err := util.SeparateFilterAndRepo(repoWithFilter)
			if err != nil {
				suicide(err, true)
			}

			registry := docker.GetRegistry(repository)

			repoPath := docker.GetRepoPath(repository, registry)
			repoName := docker.GetRepoName(repository, registry)

			username, password, _ := dockerConfig.GetCredentials(registry)

			tr, err := auth.NewToken(registry, repoPath, username, password)
			if err != nil {
				suicide(err, true)
			}

			remoteTags, err := remote.FetchTags(registry, repoPath, tr.AuthHeader(), concurrentRequests)
			if err != nil {
				suicide(err, true)
			}

			imageSummaries, err := dc.ListImagesForRepo(repoName)
			if err != nil {
				suicide(err, true)
			}
			localTags, err := local.FetchTags(repoName, imageSummaries)
			if err != nil {
				suicide(err, true)
			}

			sortedKeys, names, joinedTags := tag.Join(remoteTags, localTags)

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

			tcc <- tag.Collection{
				Registry: registry,
				RepoName: repoName,
				RepoPath: repoPath,
				Tags:     tags,
			}
		}(repoWithFilter, o.ConcurrentRequests, tcc)
	}

	tagCollections := make([]tag.Collection, repoCount)
	repoNumber := 0
	for tc := range tcc {
		tagCollections = append(tagCollections, tc)

		repoNumber++

		if repoNumber >= repoCount {
			close(tcc)
		}
	}

	for _, tc := range tagCollections {
		for _, tg := range tc.Tags {
			fmt.Printf(
				format,
				tg.GetState(),
				tg.GetShortDigest(),
				tg.GetImageID(),
				tg.GetCreatedString(),
				tc.RepoName+":"+tg.GetName(),
			)
		}
	}

	if o.Pull {
		done := make(chan bool, pullCount)

		for _, tc := range tagCollections {
			go func(tc tag.Collection, done chan bool) {
				for _, tg := range tc.Tags {
					if tg.NeedsPull() {
						ref := tc.RepoName + ":" + tg.GetName()

						fmt.Printf("PULLING %s\n", ref)
						err := dc.Pull(ref)
						if err != nil {
							suicide(err, false)
						}

						done <- true
					}

				}
			}(tc, done)
		}

		p := 0
		if pullCount > 0 {
			for range done {
				p++

				if p >= pullCount {
					close(done)
				}
			}
		}
	}

	if o.Pull && o.PushRegistry != "" {
		done := make(chan bool, pushCount)

		for _, tc := range tagCollections {
			go func(tc tag.Collection, pushRegistry, pushPrefix string, done chan bool) {
				for _, tg := range tc.Tags {
					if pushPrefix == "" {
						pushPrefix = util.GeneratePathFromHostname(tc.Registry)
					}

					srcRef := tc.RepoName + ":" + tg.GetName()
					dstRef := pushRegistry + pushPrefix + "/" + tc.RepoPath + ":" + tg.GetName()

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
			}(tc, o.PushRegistry, o.PushPrefix, done)
		}

		p := 0
		if pushCount > 0 {
			for range done {
				p++

				if p >= pushCount {
					close(done)
				}
			}
		}
	}

}
