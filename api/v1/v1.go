package v1

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"runtime"
	"strings"
	"time"

	"github.com/ivanilves/lstags/api/v1/collection"
	dockerclient "github.com/ivanilves/lstags/docker/client"
	dockerconfig "github.com/ivanilves/lstags/docker/config"
	"github.com/ivanilves/lstags/repository"
	"github.com/ivanilves/lstags/tag"
	"github.com/ivanilves/lstags/tag/local"
	"github.com/ivanilves/lstags/tag/remote"
	"github.com/ivanilves/lstags/util/wait"
)

// Config holds API instance configuration
type Config struct {
	DockerJSONConfigFile string
	ConcurrentRequests   int
	TraceRequests        bool
	RetryRequests        int
	RetryDelay           time.Duration
	InsecureRegistryEx   string
	VerboseLogging       bool
}

// PushConfig holds push-specific configuration
type PushConfig struct {
	Prefix        string
	Registry      string
	UpdateChanged bool
}

// API represents application API instance
type API struct {
	config       Config
	dockerClient *dockerclient.DockerClient
}

// fn gives the name of the calling function (e.g. enriches log.Debugf() output)
// + optionally attaches free form string labels (mainly to identify goroutines)
func fn(labels ...string) string {
	function, _, _, _ := runtime.Caller(1)

	longname := runtime.FuncForPC(function).Name()

	nameparts := strings.Split(longname, ".")
	shortname := nameparts[len(nameparts)-1]

	if labels == nil {
		return fmt.Sprintf("[%s()]", shortname)
	}

	return fmt.Sprintf("[%s():%s]", shortname, strings.Join(labels, ":"))
}

// CollectTags collects information on tags present in remote registry and [local] Docker daemon,
// makes required comparisons between them and spits organized info back as collection.Collection
func (api *API) CollectTags(refs ...string) (*collection.Collection, error) {
	if len(refs) == 0 {
		return nil, fmt.Errorf("no image references passed")
	}

	log.Debugf("%s references: %+v", fn(), refs)

	repos, err := repository.ParseRefs(refs)
	if err != nil {
		return nil, err
	}
	for _, repo := range repos {
		log.Debugf("%s repository: %+v", fn(), repo)
	}

	done := make(chan error, len(repos))
	tags := make(map[string][]*tag.Tag)

	for _, repo := range repos {
		go func(repo *repository.Repository, done chan error) {
			log.Infof("ANALYZE %s", repo.Ref())

			username, password, _ := api.dockerClient.Config().GetCredentials(repo.Registry())

			remoteTags, err := remote.FetchTags(repo, username, password)
			if err != nil {
				done <- err
				return
			}
			log.Debugf("%s remote tags: %+v", fn(repo.Ref()), remoteTags)

			localTags, _ := local.FetchTags(repo, api.dockerClient)

			log.Debugf("%s local tags: %+v", fn(repo.Ref()), localTags)

			sortedKeys, tagNames, joinedTags := tag.Join(
				remoteTags,
				localTags,
				repo.Tags(),
			)
			log.Debugf("%s joined tags: %+v", fn(repo.Ref()), joinedTags)

			tags[repo.Ref()] = tag.Collect(sortedKeys, tagNames, joinedTags)

			done <- nil

			log.Infof("FETCHED %s", repo.Ref())

			return
		}(repo, done)
	}

	if err := wait.Until(done); err != nil {
		return nil, err
	}
	log.Debugf("%s tags: %+v", fn(), tags)

	return collection.New(refs, tags)
}

func getPushPrefix(prefix, defaultPrefix string) string {
	if prefix == "" {
		return defaultPrefix
	}

	if prefix[0:1] != "/" {
		prefix = "/" + prefix
	}

	if prefix[len(prefix)-1:] != "/" {
		prefix = prefix + "/"
	}

	return prefix
}

// CollectPushTags blends passed collection with information fetched from [local] "push" registry,
// makes required comparisons between them and spits organized info back as collection.Collection
func (api *API) CollectPushTags(cn *collection.Collection, push PushConfig) (*collection.Collection, error) {
	log.Debugf(
		"%s collection: %+v (%d repos / %d tags)",
		fn(), cn, cn.RepoCount(), cn.TagCount(),
	)
	log.Debugf("%s push config: %+v", fn(), push)

	refs := make([]string, len(cn.Refs()))
	done := make(chan error, len(cn.Refs()))
	tags := make(map[string][]*tag.Tag)

	for i, repo := range cn.Repos() {
		go func(repo *repository.Repository, i int, done chan error) {
			refs[i] = repo.Ref()

			pushRef := fmt.Sprintf(
				"%s%s~/.*/",
				push.Registry,
				getPushPrefix(push.Prefix, repo.PushPrefix())+repo.Path(),
			)

			log.Debugf("%s 'push' reference: %+v", fn(repo.Ref()), pushRef)

			pushRepo, _ := repository.ParseRef(pushRef)

			log.Infof("[PULL/PUSH] ANALYZE %s => %s", repo.Ref(), pushRef)

			username, password, _ := api.dockerClient.Config().GetCredentials(push.Registry)

			pushedTags, err := remote.FetchTags(pushRepo, username, password)
			if err != nil {
				if !strings.Contains(err.Error(), "404 Not Found") {
					done <- err
					return
				}

				log.Warnf("%s repo not found: %+s", fn(repo.Ref()), pushRef)

				pushedTags = make(map[string]*tag.Tag)
			}
			log.Debugf("%s pushed tags: %+v", fn(repo.Ref()), pushedTags)

			remoteTags := cn.TagMap(repo.Ref())
			log.Debugf("%s remote tags: %+v", fn(repo.Ref()), remoteTags)

			sortedKeys, tagNames, joinedTags := tag.Join(
				remoteTags,
				pushedTags,
				repo.Tags(),
			)
			log.Debugf("%s joined tags: %+v", fn(repo.Ref()), joinedTags)

			tagsToPush := make([]*tag.Tag, 0)
			for _, key := range sortedKeys {
				name := tagNames[key]
				tg := joinedTags[name]

				if tg.NeedsPush(push.UpdateChanged) {
					tagsToPush = append(tagsToPush, tg)
				}
			}
			log.Debugf("%s tags to push: %+v", fn(repo.Ref()), tagsToPush)

			tags[repo.Ref()] = tagsToPush

			done <- nil

			return
		}(repo, i, done)
	}

	if err := wait.Until(done); err != nil {
		return nil, err
	}
	log.Debugf("%s 'push' tags: %+v", fn(), tags)

	return collection.New(refs, tags)
}

// PullTags compares images from remote registry and Docker daemon and pulls
// images that match tag spec passed and are not present in Docker daemon.
func (api *API) PullTags(cn *collection.Collection) error {
	log.Debugf(
		"%s collection: %+v (%d repos / %d tags)",
		fn(), cn, cn.RepoCount(), cn.TagCount(),
	)

	done := make(chan error, cn.TagCount())

	for _, ref := range cn.Refs() {
		repo := cn.Repo(ref)
		tags := cn.Tags(ref)

		log.Debugf("%s repository: %+v", fn(), repo)
		for _, tg := range tags {
			log.Debugf("%s tag: %+v", fn(), tg)
		}

		go func(repo *repository.Repository, tags []*tag.Tag, done chan error) {
			for _, tg := range tags {
				if !tg.NeedsPull() {
					done <- nil
					continue
				}

				ref := repo.Name() + ":" + tg.Name()

				log.Infof("PULLING %s", ref)

				done <- api.dockerClient.Pull(ref)
			}
		}(repo, tags, done)
	}

	return wait.Until(done)
}

// PushTags compares images from remote and "push" (usually local) registries,
// pulls images that are present in remote registry, but are not in "push" one
// and then [re-]pushes them to the "push" registry.
func (api *API) PushTags(cn *collection.Collection, push PushConfig) error {
	log.Debugf(
		"%s 'push' collection: %+v (%d repos / %d tags)",
		fn(), cn, cn.RepoCount(), cn.TagCount(),
	)
	log.Debugf("%s push config: %+v", fn(), push)

	done := make(chan error, cn.TagCount())

	if cn.TagCount() == 0 {
		log.Infof("%s No tags to push", fn())
		return nil
	}

	for _, ref := range cn.Refs() {
		repo := cn.Repo(ref)
		tags := cn.Tags(ref)

		log.Debugf("%s repository: %+v", fn(), repo)
		for _, tg := range tags {
			log.Debugf("%s tag: %+v", fn(), tg)
		}

		go func(repo *repository.Repository, tags []*tag.Tag, done chan error) {
			for _, tg := range tags {
				srcRef := repo.Name() + ":" + tg.Name()
				dstRef := push.Registry + getPushPrefix(push.Prefix, repo.PushPrefix()) + repo.Path() + ":" + tg.Name()

				log.Infof("[PULL/PUSH] PUSHING %s => %s", srcRef, dstRef)

				done <- api.dockerClient.RePush(srcRef, dstRef)
			}
		}(repo, tags, done)
	}

	return wait.Until(done)
}

// New creates new instance of application API
func New(config Config) (*API, error) {
	if config.VerboseLogging {
		log.SetLevel(log.DebugLevel)
	}
	log.Debugf("%s API config: %+v", fn(), config)

	if config.ConcurrentRequests == 0 {
		config.ConcurrentRequests = 1
	}
	remote.ConcurrentRequests = config.ConcurrentRequests
	remote.TraceRequests = config.TraceRequests
	remote.RetryRequests = config.RetryRequests
	remote.RetryDelay = config.RetryDelay

	dockerclient.RetryPulls = config.RetryRequests
	dockerclient.RetryDelay = config.RetryDelay

	if config.InsecureRegistryEx != "" {
		repository.InsecureRegistryEx = config.InsecureRegistryEx
	}

	if config.DockerJSONConfigFile == "" {
		config.DockerJSONConfigFile = dockerconfig.DefaultDockerJSON
	}
	dockerConfig, err := dockerconfig.Load(config.DockerJSONConfigFile)
	if err != nil {
		return nil, err
	}
	dockerClient, err := dockerclient.New(dockerConfig)
	if err != nil {
		return nil, err
	}

	return &API{
		config:       config,
		dockerClient: dockerClient,
	}, nil
}
