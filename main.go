package main

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/jessevdk/go-flags"

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
	DockerJSON         string        `short:"j" long:"docker-json" default:"~/.docker/config.json" description:"JSON file with credentials" env:"DOCKER_JSON"`
	Pull               bool          `short:"p" long:"pull" description:"Pull Docker images matched by filter (will use local Docker deamon)" env:"PULL"`
	Push               bool          `short:"P" long:"push" description:"Push Docker images matched by filter to some registry (See 'push-registry')" env:"PUSH"`
	PushRegistry       string        `short:"r" long:"push-registry" description:"[Re]Push pulled images to a specified remote registry" env:"PUSH_REGISTRY"`
	PushPrefix         string        `short:"R" long:"push-prefix" description:"[Re]Push pulled images with a specified repo path prefix" env:"PUSH_PREFIX"`
	PushUpdate         bool          `short:"U" long:"push-update" description:"Update our pushed images if remote image digest changes" env:"PUSH_UPDATE"`
	ConcurrentRequests int           `short:"c" long:"concurrent-requests" default:"32" description:"Limit of concurrent requests to the registry" env:"CONCURRENT_REQUESTS"`
	RetryRequests      int           `short:"y" long:"retry-requests" default:"2" description:"Number of retries for failed registry HTTP requests" env:"RETRY_REQUESTS"`
	RetryDelay         time.Duration `short:"D" long:"retry-delay" default:"30s" description:"Delay between retries of failed registry requests" env:"RETRY_DELAY"`
	InsecureRegistryEx string        `short:"I" long:"insecure-registry-ex" description:"Expression to match insecure registry hostnames" env:"INSECURE_REGISTRY_EX"`
	TraceRequests      bool          `short:"T" long:"trace-requests" description:"Trace Docker registry HTTP requests" env:"TRACE_REQUESTS"`
	DoNotFail          bool          `short:"N" long:"do-not-fail" description:"Do not fail on non-critical errors (could be dangerous!)" env:"DO_NOT_FAIL"`
	Version            bool          `short:"V" long:"version" description:"Show version and exit"`
	Positional         struct {
		Repositories []string `positional-arg-name:"REPO1 REPO2 REPOn" description:"Docker repositories to operate on, e.g.: alpine nginx~/1\\.13\\.5$/ busybox~/1.27.2/"`
	} `positional-args:"yes" required:"yes"`
}

var doNotFail = false

func suicide(err error, critical bool) {
	fmt.Printf("%s\n", err.Error())

	if !doNotFail || critical {
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

	if o.PushRegistry != "localhost:5000" && o.PushRegistry != "" {
		o.Push = true
	}

	if o.Pull && o.Push {
		return nil, errors.New("You either '--pull' or '--push', not both")
	}

	remote.ConcurrentRequests = o.ConcurrentRequests

	remote.RetryRequests = o.RetryRequests

	remote.RetryDelay = o.RetryDelay

	if o.InsecureRegistryEx != "" {
		docker.InsecureRegistryEx = o.InsecureRegistryEx
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

	repoCount := len(o.Positional.Repositories)

	tcc := make(chan tag.Collection, repoCount)

	pullCount := 0
	pushCount := 0

	for _, repoWithFilter := range o.Positional.Repositories {
		go func(repoWithFilter string, tcc chan tag.Collection) {
			repository, filter, err := util.SeparateFilterAndRepo(repoWithFilter)
			if err != nil {
				suicide(err, true)
			}

			registry := docker.GetRegistry(repository)

			repoPath := docker.GetRepoPath(repository, registry)
			repoName := docker.GetRepoName(repository, registry)

			fmt.Printf("ANALYZE %s\n", repoName)

			username, password, _ := dockerConfig.GetCredentials(registry)

			remoteTags, err := remote.FetchTags(registry, repoPath, filter, username, password)
			if err != nil {
				suicide(err, true)
			}

			localTags, err := local.FetchTags(repoName, filter, dc)
			if err != nil {
				suicide(err, true)
			}

			sortedKeys, names, joinedTags := tag.Join(remoteTags, localTags)

			tags := make([]*tag.Tag, 0)
			pullTags := make([]*tag.Tag, 0)
			for _, key := range sortedKeys {
				name := names[key]

				tg := joinedTags[name]

				if tg.NeedsPull() {
					pullTags = append(pullTags, tg)
					pullCount++
				}

				tags = append(tags, tg)
			}

			var pushPrefix string
			pushTags := make([]*tag.Tag, 0)
			if o.Push {
				tags = make([]*tag.Tag, 0)

				pushPrefix = o.PushPrefix
				if pushPrefix == "" {
					pushPrefix = util.GeneratePathFromHostname(registry)
				}

				var pushRepoPath string
				pushRepoPath = pushPrefix + "/" + repoPath
				pushRepoPath = pushRepoPath[1:] // Leading "/" in prefix should be removed!

				username, password, _ := dockerConfig.GetCredentials(o.PushRegistry)

				alreadyPushedTags, err := remote.FetchTags(o.PushRegistry, pushRepoPath, username, password, filter)
				if err != nil {
					if !strings.Contains(err.Error(), "404 Not Found") {
						suicide(err, true)
					}

					alreadyPushedTags = make(map[string]*tag.Tag)
				}

				sortedKeys, names, joinedTags := tag.Join(remoteTags, alreadyPushedTags)
				for _, key := range sortedKeys {
					name := names[key]

					tg := joinedTags[name]

					if tg.NeedsPush(o.PushUpdate) {
						pushTags = append(pushTags, tg)
						pushCount++
					}

					tags = append(tags, tg)
				}
			}

			tcc <- tag.Collection{
				Registry:   registry,
				RepoName:   repoName,
				RepoPath:   repoPath,
				Tags:       tags,
				PullTags:   pullTags,
				PushTags:   pushTags,
				PushPrefix: pushPrefix,
			}
		}(repoWithFilter, tcc)
	}

	tagCollections := make([]tag.Collection, repoCount-1)

	r := 0
	for tc := range tcc {
		fmt.Printf("FETCHED %s\n", tc.RepoName)

		tagCollections = append(tagCollections, tc)
		r++

		if r >= repoCount {
			close(tcc)
		}
	}

	const format = "%-12s %-45s %-15s %-25s %s\n"
	fmt.Printf("-\n")
	fmt.Printf(format, "<STATE>", "<DIGEST>", "<(local) ID>", "<Created At>", "<TAG>")
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
	fmt.Printf("-\n")

	if o.Pull {
		done := make(chan bool, pullCount)

		for _, tc := range tagCollections {
			go func(tc tag.Collection, done chan bool) {
				for _, tg := range tc.PullTags {
					ref := tc.RepoName + ":" + tg.GetName()

					fmt.Printf("PULLING %s\n", ref)
					err := dc.Pull(ref)
					if err != nil {
						suicide(err, false)
					}

					done <- true
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

	if o.Push {
		done := make(chan bool, pushCount)

		for _, tc := range tagCollections {
			go func(tc tag.Collection, done chan bool) {
				for _, tg := range tc.PushTags {
					var err error

					srcRef := tc.RepoName + ":" + tg.GetName()
					dstRef := o.PushRegistry + tc.PushPrefix + "/" + tc.RepoPath + ":" + tg.GetName()

					fmt.Printf("[PULL/PUSH] PULLING %s\n", srcRef)
					err = dc.Pull(srcRef)
					if err != nil {
						suicide(err, false)
					}

					fmt.Printf("[PULL/PUSH] PUSHING %s => %s\n", srcRef, dstRef)
					err = dc.Tag(srcRef, dstRef)
					if err != nil {
						suicide(err, true)
					}
					err = dc.Push(dstRef)
					if err != nil {
						suicide(err, false)
					}

					done <- true
				}
			}(tc, done)
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
