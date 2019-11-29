package main

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/jessevdk/go-flags"

	v1 "github.com/ivanilves/lstags/api/v1"
	"github.com/ivanilves/lstags/config"
)

// Options represents configuration options we extract from passed command line arguments
type Options struct {
	YAMLConfig         string        `short:"f" long:"yaml-config" description:"YAML file to load repositories from" env:"YAML_CONFIG"`
	DockerJSON         string        `short:"j" long:"docker-json" default:"~/.docker/config.json" description:"JSON file with credentials" env:"DOCKER_JSON"`
	Pull               bool          `short:"p" long:"pull" description:"Pull Docker images matched by filter (will use local Docker deamon)" env:"PULL"`
	Push               bool          `short:"P" long:"push" description:"Push Docker images matched by filter to some registry (See 'push-registry')" env:"PUSH"`
	DryRun             bool          `long:"dry-run" description:"Dry run pull or push" env:"DRY_RUN"`
	PushRegistry       string        `short:"r" long:"push-registry" description:"[Re]Push pulled images to a specified remote registry" env:"PUSH_REGISTRY"`
	PushPrefix         string        `short:"R" long:"push-prefix" description:"[Re]Push pulled images with a specified repo path prefix" env:"PUSH_PREFIX"`
	PushPathTemplate   string        `long:"push-path-template" default:"{{ .Prefix }}{{ .Path }}" description:"[Re]Push pulled images with a go template to change repo path, sprig functions are supported" env:"PUSH_PATH_TEMPLATE"`
	PushTagTemplate    string        `long:"push-tag-template" default:"{{ .Tag }}" description:"[Re]Push pulled images with a go template to change repo tag, sprig functions are supported" env:"PUSH_TAG_TEMPLATE"`
	NoSSLVerify        bool          `short:"k" long:"no-ssl-verify" description:"Allow registry without certificate verify" env:"NO_SSL_VERIFY"`
	PushUpdate         bool          `short:"U" long:"push-update" description:"Update our pushed images if remote image digest changes" env:"PUSH_UPDATE"`
	PathSeparator      string        `short:"s" long:"path-separator" default:"/" description:"Configure path separator for registries that only allow single folder depth" env:"PATH_SEPARATOR"`
	ConcurrentRequests int           `short:"c" long:"concurrent-requests" default:"16" description:"Limit of concurrent requests to the registry" env:"CONCURRENT_REQUESTS"`
	WaitBetween        time.Duration `short:"w" long:"wait-between" default:"0" description:"Time to wait between batches of requests (incl. pulls and pushes)" env:"WAIT_BETWEEN"`
	RetryRequests      int           `short:"y" long:"retry-requests" default:"2" description:"Number of retries for failed Docker registry requests" env:"RETRY_REQUESTS"`
	RetryDelay         time.Duration `short:"D" long:"retry-delay" default:"2s" description:"Delay between retries of failed registry requests" env:"RETRY_DELAY"`
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

var exitCode = 0

var doNotFail = false

func suicide(err error, critical bool) {
	fmt.Printf("%s\n", err.Error())

	if !doNotFail || critical {
		os.Exit(1)
	}

	exitCode = 254 // not typical error code, for "git grep" friendliness
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

	if len(o.Positional.Repositories) == 0 && o.YAMLConfig == "" {
		return nil, errors.New(`Need at least one repository name, e.g. 'nginx~/^1\.13/' or 'mesosphere/chronos'`)
	}

	if len(o.Positional.Repositories) != 0 && o.YAMLConfig != "" {
		return nil, errors.New("Load repositories from YAML or from CLI args, not from both at the same time")
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
		WaitBetween:          o.WaitBetween,
		TraceRequests:        o.TraceRequests,
		RetryRequests:        o.RetryRequests,
		RetryDelay:           o.RetryDelay,
		InsecureRegistryEx:   o.InsecureRegistryEx,
		VerboseLogging:       o.Verbose,
		DryRun:               o.DryRun,
	}

	if o.NoSSLVerify {
		http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}
	api, err := v1.New(apiConfig)
	if err != nil {
		suicide(err, true)
	}

	for {
		repositories := o.Positional.Repositories

		if o.YAMLConfig != "" {
			yc, err := config.LoadYAMLFile(o.YAMLConfig)
			if err != nil {
				suicide(err, !o.DaemonMode)
			}

			repositories = yc.Repositories
		}

		collection, err := api.CollectTags(repositories...)
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
				PathTemplate:  o.PushPathTemplate,
				TagTemplate:   o.PushTagTemplate,
				UpdateChanged: o.PushUpdate,
				PathSeparator: o.PathSeparator,
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
			os.Exit(exitCode)
		}

		fmt.Printf("WAIT: %v\n-\n", o.PollingInterval)

		time.Sleep(o.PollingInterval)
	}
}
