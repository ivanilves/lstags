package api

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"strings"
	"time"

	dockerclient "github.com/ivanilves/lstags/docker/client"
	dockerconfig "github.com/ivanilves/lstags/docker/config"
	"github.com/ivanilves/lstags/repository"
	"github.com/ivanilves/lstags/tag"
	"github.com/ivanilves/lstags/tag/local"
	"github.com/ivanilves/lstags/tag/remote"
)

// Config holds API instance configuration
type Config struct {
	CollectPushTags         bool
	UpdateChangedTagsOnPush bool
	PushPrefix              string
	PushRegistry            string
	DockerJSONConfigFile    string
	ConcurrentHTTPRequests  int
	TraceHTTPRequests       bool
	RetryRequests           int
	RetryDelay              time.Duration
	InsecureRegistryEx      string
}

// API represents application API instance
type API struct {
	config       Config
	dockerClient *dockerclient.DockerClient
}

// CollectTags collects information on tags present in:
// * remote registry
// * [local] Docker daemon
// * local "push" registry, if specified
// makes required comparisons between them and spits organized info back as []tag.Collection
func (api *API) CollectTags(repoRefs []string) ([]tag.Collection, error) {
	repoCount := len(repoRefs)

	tcc := make(chan tag.Collection, repoCount)

	done := make(chan error, repoCount)

	tagCollections := make([]tag.Collection, repoCount-1)

	for _, repoRef := range repoRefs {
		go func(repoRef string, tcc chan tag.Collection, done chan error) {
			repo, err := repository.ParseRef(repoRef)
			if err != nil {
				done <- err
				return
			}

			log.Infof("ANALYZE %s\n", repo.Name())

			username, password, _ := api.dockerClient.Config().GetCredentials(repo.Registry())

			remoteTags, err := remote.FetchTags(
				repo,
				username,
				password,
			)
			if err != nil {
				done <- err
				return
			}

			localTags, err := local.FetchTags(repo, api.dockerClient)
			if err != nil {
				done <- err
				return
			}

			sortedKeys, names, joinedTags := tag.Join(
				remoteTags,
				localTags,
				repo.Tags(),
			)

			tags := make([]*tag.Tag, 0)
			pullTags := make([]*tag.Tag, 0)
			for _, key := range sortedKeys {
				name := names[key]

				tg := joinedTags[name]

				if tg.NeedsPull() {
					pullTags = append(pullTags, tg)
				}

				tags = append(tags, tg)
			}

			var pushPrefix string
			pushTags := make([]*tag.Tag, 0)
			if api.config.CollectPushTags {
				tags = make([]*tag.Tag, 0)

				pushPrefix = api.config.PushPrefix
				if pushPrefix == "" {
					pushPrefix = repo.PushPrefix()
				}

				var pushRepoPath string
				pushRepoPath = pushPrefix + "/" + repo.Path()
				pushRepoPath = pushRepoPath[1:] // Leading "/" in prefix should be removed!

				username, password, _ := api.dockerClient.Config().GetCredentials(api.config.PushRegistry)

				pushRef := fmt.Sprintf("%s/%s~/.*/", api.config.PushRegistry, pushRepoPath)

				pushRepo, _ := repository.ParseRef(pushRef)

				alreadyPushedTags, err := remote.FetchTags(
					pushRepo,
					username,
					password,
				)
				if err != nil {
					if !strings.Contains(err.Error(), "404 Not Found") {
						done <- err
						return
					}

					alreadyPushedTags = make(map[string]*tag.Tag)
				}

				sortedKeys, names, joinedTags := tag.Join(
					remoteTags,
					alreadyPushedTags,
					repo.Tags(),
				)

				for _, key := range sortedKeys {
					name := names[key]

					tg := joinedTags[name]

					if tg.NeedsPush(api.config.UpdateChangedTagsOnPush) {
						pushTags = append(pushTags, tg)
					}

					tags = append(tags, tg)
				}
			}

			tcc <- tag.Collection{
				Registry:   repo.Registry(),
				RepoName:   repo.Name(),
				RepoPath:   repo.Path(),
				Tags:       tags,
				PullTags:   pullTags,
				PushTags:   pushTags,
				PushPrefix: pushPrefix,
			}
		}(repoRef, tcc, done)

		done <- nil
	}

	var r int

	r = 0
	for err := range done {
		if err != nil {
			return nil, err
		}

		r++

		if r >= repoCount {
			close(done)
		}
	}

	r = 0
	for tc := range tcc {
		fmt.Printf("FETCHED %s\n", tc.RepoName)

		tagCollections = append(tagCollections, tc)
		r++

		if r >= repoCount {
			close(tcc)
		}
	}

	return tagCollections, nil
}

// PullTags compares images from remote registry and Docker daemon and pulls
// images that match tag spec passed and are not present in Docker daemon.
func (api *API) PullTags(tagCollections []tag.Collection) error {
	var pullCount = 0
	for _, tc := range tagCollections {
		pullCount += len(tc.PullTags)
	}

	done := make(chan error, pullCount)

	for _, tc := range tagCollections {
		go func(tc tag.Collection, done chan error) {
			for _, tg := range tc.PullTags {
				ref := tc.RepoName + ":" + tg.GetName()

				log.Infof("PULLING %s\n", ref)
				err := api.dockerClient.Pull(ref)
				if err != nil {
					done <- err
					return
				}

				done <- nil
			}
		}(tc, done)
	}

	p := 0
	if pullCount > 0 {
		for err := range done {
			if err != nil {
				return err
			}

			p++

			if p >= pullCount {
				close(done)
			}
		}
	}

	return nil
}

// PushTags compares images from remote and "push" (usually local) registries,
// pulls images that are present in remote registry, but are not in "push" one
// and then [re-]pushes them to the "push" registry.
func (api *API) PushTags(tagCollections []tag.Collection) error {
	var pushCount = 0
	for _, tc := range tagCollections {
		pushCount += len(tc.PushTags)
	}

	done := make(chan error, pushCount)

	for _, tc := range tagCollections {
		go func(tc tag.Collection, done chan error) {
			for _, tg := range tc.PushTags {
				var err error

				srcRef := tc.RepoName + ":" + tg.GetName()
				dstRef := api.config.PushRegistry + tc.PushPrefix + "/" + tc.RepoPath + ":" + tg.GetName()

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
		}(tc, done)
	}

	p := 0
	if pushCount > 0 {
		for err := range done {
			if err != nil {
				return err
			}

			p++

			if p >= pushCount {
				close(done)
			}
		}
	}

	return nil
}

// New creates new instance of application API
func New(config Config) (*API, error) {
	remote.ConcurrentRequests = config.ConcurrentHTTPRequests

	remote.RetryRequests = config.RetryRequests
	remote.RetryDelay = config.RetryDelay

	dockerclient.RetryPulls = config.RetryRequests
	dockerclient.RetryDelay = config.RetryDelay

	if config.InsecureRegistryEx != "" {
		repository.InsecureRegistryEx = config.InsecureRegistryEx
	}

	remote.TraceRequests = config.TraceHTTPRequests

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
