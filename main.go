package main

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/jessevdk/go-flags"

	"github.com/ivanilves/lstags/api/v1"
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
	RetryRequests      int           `short:"y" long:"retry-requests" default:"2" description:"Number of retries for failed Docker registry requests" env:"RETRY_REQUESTS"`
	RetryDelay         time.Duration `short:"D" long:"retry-delay" default:"30s" description:"Delay between retries of failed registry requests" env:"RETRY_DELAY"`
	InsecureRegistryEx string        `short:"I" long:"insecure-registry-ex" description:"Expression to match insecure registry hostnames" env:"INSECURE_REGISTRY_EX"`
	TraceRequests      bool          `short:"T" long:"trace-requests" description:"Trace Docker registry HTTP requests" env:"TRACE_REQUESTS"`
	DoNotFail          bool          `short:"N" long:"do-not-fail" description:"Do not fail on non-critical errors (could be dangerous!)" env:"DO_NOT_FAIL"`
	DaemonMode         bool          `short:"d" long:"daemon-mode" description:"Run as daemon instead of just execute and exit" env:"DAEMON_MODE"`
	PollingInterval    time.Duration `short:"i" long:"polling-interval" default:"60s" description:"Wait between polls when running in daemon mode" env:"POLLING_INTERVAL"`
	Verbose            bool          `short:"v" long:"verbose" description:"Give verbose output while running application" env:"VERBOSE"`
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
		os.Exit(1) // YES! Just exit! Flags will compain on errors on it's own behalf
	}

	if o.Version {
		fmt.Printf("VERSION: %s\n", getVersion())
		os.Exit(0)
	}

	if len(o.Positional.Repositories) == 0 {
		return nil, errors.New(`Need at least one repository name, e.g. 'nginx~/^1\.13/' or 'mesosphere/chronos'`)
	}

	if o.PushRegistry != "localhost:5000" && o.PushRegistry != "" {
		o.Push = true
	}

	if o.Pull && o.Push {
		return nil, errors.New("You either '--pull' or '--push', not both")
	}

	doNotFail = o.DoNotFail || o.DaemonMode

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

	apiConfig := v1.Config{
		DockerJSONConfigFile: o.DockerJSON,
		ConcurrentRequests:   o.ConcurrentRequests,
		TraceRequests:        o.TraceRequests,
		RetryRequests:        o.RetryRequests,
		RetryDelay:           o.RetryDelay,
		InsecureRegistryEx:   o.InsecureRegistryEx,
		VerboseLogging:       o.Verbose,
	}

	api, err := v1.New(apiConfig)
	if err != nil {
		suicide(err, true)
	}

	for {
		collection, err := api.CollectTags(o.Positional.Repositories...)
		if err != nil {
			suicide(err, !o.DaemonMode)
		}

		const format = "%-12s %-45s %-15s %-25s %s:%s\n"
		fmt.Printf("-\n")
		fmt.Printf(format, "<STATE>", "<DIGEST>", "<(local) ID>", "<Created At>", "<IMAGE>", "<TAG>")
		for _, ref := range collection.Refs() {
			repo := collection.Repo(ref)
			tags := collection.Tags(ref)

			for _, tg := range tags {
				fmt.Printf(
					format,
					tg.GetState(),
					tg.GetShortDigest(),
					tg.GetImageID(),
					tg.GetCreatedString(),
					repo.Name(),
					tg.Name(),
				)
			}
		}
		fmt.Printf("-\n")

		if o.Pull {
			if err := api.PullTags(collection); err != nil {
				suicide(err, false)
			}
		}

		if o.Push {
			pushConfig := v1.PushConfig{
				Registry:      o.PushRegistry,
				Prefix:        o.PushPrefix,
				UpdateChanged: o.PushUpdate,
			}

			pushCollection, err := api.CollectPushTags(collection, pushConfig)
			if err != nil {
				suicide(err, false)
			}

			if err := api.PushTags(pushCollection, pushConfig); err != nil {
				suicide(err, false)
			}
		}

		if !o.DaemonMode {
			os.Exit(0)
		}

		fmt.Printf("WAIT: %v\n-\n", o.PollingInterval)

		time.Sleep(o.PollingInterval)
	}
}
