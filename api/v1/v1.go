package v1

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"strings"
	"time"

	"github.com/ivanilves/lstags/api/v1/collection"
	dockerclient "github.com/ivanilves/lstags/docker/client"
	dockerconfig "github.com/ivanilves/lstags/docker/config"
	"github.com/ivanilves/lstags/repository"
	"github.com/ivanilves/lstags/tag"
	"github.com/ivanilves/lstags/tag/local"
	"github.com/ivanilves/lstags/tag/remote"
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
	PushPrefix              string
	PushRegistry            string
	UpdateChangedTagsOnPush bool
}

// API represents application API instance
type API struct {
	config       Config
	dockerClient *dockerclient.DockerClient
}

// CollectTags collects information on tags present in remote registry and [local] Docker daemon,
// makes required comparisons between them and spits organized info back as collection.Collection
func (api *API) CollectTags(refs []string) (*collection.Collection, error) {
	log.Debugf("Will process repository references: %+v\n", refs)

	repos, err := repository.ParseRefs(refs)
	if err != nil {
		return nil, err
	}
	log.Debugf("Processed repository references. Got: %+v\n", repos)

	done := make(chan error, len(repos))
	tags := make(map[string][]*tag.Tag)

	for _, repo := range repos {
		go func(repo *repository.Repository, tags map[string][]*tag.Tag, done chan error) {
			log.Infof("ANALYZE %s\n", repo.Ref())

			username, password, _ := api.dockerClient.Config().GetCredentials(repo.Registry())

			remoteTags, err := remote.FetchTags(repo, username, password)
			if err != nil {
				done <- err
				return
			}
			log.Debugf("Remote tags: %+v\n", remoteTags)

			localTags, err := local.FetchTags(repo, api.dockerClient)
			if err != nil {
				done <- err
				return
			}
			log.Debugf("Local tags: %+v\n", localTags)

			sortedKeys, tagNames, joinedTags := tag.Join(
				remoteTags,
				localTags,
				repo.Tags(),
			)
			log.Debugf("Joined tags: %+v\n", joinedTags)

			tags[repo.Ref()] = tag.Collect(sortedKeys, tagNames, joinedTags)

			done <- nil
			return
		}(repo, tags, done)
	}

	if err := waitForDone(done); err != nil {
		return nil, err
	}

	cn, err := collection.New(refs, tags)
	if err != nil {
		return nil, err
	}

	return cn, nil
}

// CollectPushTags blends passed collection with information fetched from [local] "push" registry,
// makes required comparisons between them and spits organized info back as collection.Collection
func (api *API) CollectPushTags(cn *collection.Collection, pushConfig PushConfig) (*collection.Collection, error) {
	log.Warnf("Collection: %#v\n", cn)

	refs := make([]string, len(cn.Refs()))
	done := make(chan error, len(cn.Refs()))
	tags := make(map[string][]*tag.Tag)

	for i, repo := range cn.Repos() {
		go func(repo *repository.Repository, tags map[string][]*tag.Tag, done chan error) {
			pushPrefix := pushConfig.PushPrefix
			if pushPrefix == "" {
				pushPrefix = repo.PushPrefix()
			}

			var pushRepoPath string
			pushRepoPath = pushPrefix + "/" + repo.Path()
			pushRepoPath = pushRepoPath[1:] // Leading "/" in prefix should be removed!

			username, password, _ := api.dockerClient.Config().GetCredentials(pushConfig.PushRegistry)

			pushRef := fmt.Sprintf("%s/%s~/.*/", pushConfig.PushRegistry, pushRepoPath)

			refs[i] = repo.Ref()

			pushRepo, _ := repository.ParseRef(pushRef)

			log.Infof("[PULL/PUSH] ANALYZE %s => %s\n", repo.Ref(), pushRef)

			pushedTags, err := remote.FetchTags(pushRepo, username, password)
			if err != nil {
				if !strings.Contains(err.Error(), "404 Not Found") {
					done <- err
					return
				}

				pushedTags = make(map[string]*tag.Tag)
			}

			localTags := cn.TagMap(repo.Ref())
			sortedKeys, tagNames, joinedTags := tag.Join(
				localTags,
				pushedTags,
				repo.Tags(),
			)

			log.Warnf("Local tags: %#v\n", localTags)
			log.Warnf("Joined tags: %#v\n", joinedTags)

			pushTags := make([]*tag.Tag, 0)
			for _, key := range sortedKeys {
				name := tagNames[key]

				tg := localTags[name]

				if tg.NeedsPush(pushConfig.UpdateChangedTagsOnPush) {
					pushTags = append(pushTags, tg)
				}
			}

			tags[repo.Ref()] = pushTags

			done <- nil
			return
		}(repo, tags, done)
	}

	if err := waitForDone(done); err != nil {
		return nil, err
	}

	log.Warnf("Push tags: %#v\n", tags)

	pushcn, err := collection.New(refs, tags)
	if err != nil {
		return nil, err
	}

	return pushcn, nil
}

// PullTags compares images from remote registry and Docker daemon and pulls
// images that match tag spec passed and are not present in Docker daemon.
func (api *API) PullTags(cn *collection.Collection) error {
	done := make(chan error, cn.TagCount())

	for _, ref := range cn.Refs() {
		repo := cn.Repo(ref)
		tags := cn.Tags(ref)

		go func(repo *repository.Repository, tags []*tag.Tag, done chan error) {
			for _, tg := range tags {
				ref := repo.Name() + ":" + tg.Name()

				log.Infof("PULLING %s\n", ref)
				err := api.dockerClient.Pull(ref)
				if err != nil {
					done <- err
					return
				}

				done <- nil
			}
		}(repo, tags, done)
	}

	return waitForDone(done)
}

// PushTags compares images from remote and "push" (usually local) registries,
// pulls images that are present in remote registry, but are not in "push" one
// and then [re-]pushes them to the "push" registry.
func (api *API) PushTags(cn *collection.Collection, pushConfig PushConfig) error {
	log.Warnf("Push collection: %#v", cn)

	done := make(chan error, cn.TagCount())

	if cn.TagCount() == 0 {
		log.Warnf("No tags found\n")
		return nil
	}

	for _, ref := range cn.Refs() {
		repo := cn.Repo(ref)
		tags := cn.Tags(ref)

		go func(repo *repository.Repository, tags []*tag.Tag, done chan error) {
			for _, tg := range tags {
				var err error

				srcRef := repo.Name() + ":" + tg.Name()
				dstRef := pushConfig.PushRegistry + pushConfig.PushPrefix + "/" + repo.Path() + ":" + tg.Name()

				log.Infof("[PULL/PUSH] PULLING %s\n", srcRef)
				err = api.dockerClient.Pull(srcRef)
				if err != nil {
					done <- err
					return
				}

				log.Infof("[PULL/PUSH] PUSHING %s => %s\n", srcRef, dstRef)
				err = api.dockerClient.Tag(srcRef, dstRef)
				if err != nil {
					done <- err
					return
				}
				err = api.dockerClient.Push(dstRef)
				if err != nil {
					done <- err
					return
				}

				done <- nil
			}
		}(repo, tags, done)
	}

	return waitForDone(done)
}

func waitForDone(done chan error) error {
	defer close(done)

	i := 0
	for err := range done {
		if err != nil {
			return err
		}
		if i >= len(done)-1 {
			return nil
		}

		i++
	}

	return fmt.Errorf("how did you get here? :-/")
}

// New creates new instance of application API
func New(config Config) (*API, error) {
	remote.ConcurrentRequests = config.ConcurrentRequests

	remote.TraceRequests = config.TraceRequests

	remote.RetryRequests = config.RetryRequests
	remote.RetryDelay = config.RetryDelay

	dockerclient.RetryPulls = config.RetryRequests
	dockerclient.RetryDelay = config.RetryDelay

	if config.InsecureRegistryEx != "" {
		repository.InsecureRegistryEx = config.InsecureRegistryEx
	}

	if config.VerboseLogging {
		log.SetLevel(log.DebugLevel)
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
