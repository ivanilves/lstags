// Package v1 provides lstags v1 API to be used both by the application
// itself and by external projects
package v1

import (
	"bufio"
	"bytes"
	"fmt"
	"html/template"
	"io"
	"runtime"
	"strings"
	"time"

	"github.com/Masterminds/sprig/v3"
	log "github.com/sirupsen/logrus"

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
	// DockerJSONConfigFile is a path to Docker JSON config file
	DockerJSONConfigFile string
	// ConcurrentRequests defines how much requests to registry we could run in parallel
	ConcurrentRequests int
	// WaitBetween defines how much we will wait between batches of requests (incl. pull and push)
	WaitBetween time.Duration
	// TraceRequests sets if we will print out registry HTTP request traces
	TraceRequests bool
	// RetryRequests defines how much retries we will do to the failed HTTP request
	RetryRequests int
	// RetryDelay defines how much we will wait between failed HTTP request and retry
	RetryDelay time.Duration
	// InsecureRegistryEx is a regex string to match insecure (non-HTTPS) registries
	InsecureRegistryEx string
	// VerboseLogging sets if we will print debug log messages
	VerboseLogging bool
	// DryRun sets if we will dry run pull or push
	DryRun bool
}

// PushConfig holds push-specific configuration (where to push and with which prefix)
type PushConfig struct {
	// Prefix is prepended to the repository path while pushing to the registry
	Prefix string
	// Registry is an address of the Docker registry in which we push our images
	Registry string
	// UpdateChanged tells us if we will re-push (update/overwrite) images having same tag, but different digest
	UpdateChanged bool
	// PathSeparator defines which path separator to use (default: "/")
	PathSeparator string
	// PathTemplate is a template to change push path, sprig functions are supprted
	PathTemplate string
	// TagTemplate is a template to change push tag, sprig functions are supprted
	TagTemplate string
}

// API represents configured application API instance,
// the main abstraction you are supposed to work with
type API struct {
	config       Config
	dockerClient *dockerclient.DockerClient
}

// rtags is a structure to send collection of referenced tags using chan
type rtags struct {
	ref  string
	tags []*tag.Tag
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

func getBatchedSlices(batchSize int, unbatched ...string) [][]string {
	batchedSlices := make([][]string, 0)

	index := 0

	for range unbatched {
		batchedSlice := make([]string, 0)

		for c := 0; c < batchSize; c++ {
			batchedSlice = append(batchedSlice, unbatched[index])

			index++

			if index == len(unbatched) {
				break
			}
		}

		batchedSlices = append(batchedSlices, batchedSlice)

		if index == len(unbatched) {
			break
		}
	}

	return batchedSlices
}

func receiveTags(tagc chan rtags) map[string][]*tag.Tag {
	tags := make(map[string][]*tag.Tag)

	step := 1
	size := cap(tagc)
	for t := range tagc {
		log.Debugf("[%s] receiving tags: %+v", t.ref, t.tags)

		tags[t.ref] = t.tags

		if step >= size {
			close(tagc)
		}

		step++
	}

	return tags
}

// CollectTags collects information on tags present in remote registry and [local] Docker daemon,
// makes required comparisons between them and spits organized info back as collection.Collection
func (api *API) CollectTags(refs ...string) (*collection.Collection, error) {
	if len(refs) == 0 {
		return nil, fmt.Errorf("no image references passed")
	}

	_, err := repository.ParseRefs(refs)
	if err != nil {
		return nil, err
	}

	tagc := make(chan rtags, len(refs))

	batchedSlicesOfRefs := getBatchedSlices(api.config.ConcurrentRequests, refs...)

	for bindex, brefs := range batchedSlicesOfRefs {
		log.Infof("BATCH %d of %d", bindex+1, len(batchedSlicesOfRefs))

		log.Debugf("%s references: %+v", fn(), brefs)

		repos, _ := repository.ParseRefs(brefs)
		for _, repo := range repos {
			log.Debugf("%s repository: %+v", fn(), repo)
		}

		done := make(chan error, len(repos))

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
				log.Debugf("%s sending joined tags: %+v", fn(repo.Ref()), joinedTags)

				tagc <- rtags{ref: repo.Ref(), tags: tag.Collect(sortedKeys, tagNames, joinedTags)}
				done <- nil

				log.Infof("FETCHED %s", repo.Ref())

				return
			}(repo, done)
		}

		if err := wait.Until(done); err != nil {
			return nil, err
		}

		time.Sleep(api.config.WaitBetween)
	}

	tags := receiveTags(tagc)

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

	pushPathTemplate, terr := makePushPathTemplate(push)
	if terr != nil {
		return nil, terr
	}

	refs := make([]string, len(cn.Refs()))
	done := make(chan error, len(cn.Refs()))
	tagc := make(chan rtags, len(refs))

	for i, repo := range cn.Repos() {
		go func(repo *repository.Repository, i int, done chan error) {
			refs[i] = repo.Ref()

			pushPath, perr := pushPathTemplate(getPushPrefix(push.Prefix, repo.PushPrefix()), repo.PushPath(push.PathSeparator), repo.Name())
			if perr != nil {
				done <- perr
				return
			}
			pushRef := fmt.Sprintf(
				"%s%s~/.*/",
				push.Registry,
				pushPath,
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
			log.Debugf("%s sending 'push' tags: %+v", fn(repo.Ref()), tagsToPush)

			tagc <- rtags{ref: repo.Ref(), tags: tagsToPush}
			done <- nil

			return
		}(repo, i, done)

		time.Sleep(api.config.WaitBetween)
	}

	if err := wait.Until(done); err != nil {
		return nil, err
	}

	tags := receiveTags(tagc)

	log.Debugf("%s 'push' tags: %+v", fn(), tags)

	return collection.New(refs, tags)
}

func makePushPathTemplate(push PushConfig) (func(pushPrefix, pushPath, name string) (string, error), error) {
	tpl, err := template.New("push-path-template").
		Funcs(sprig.FuncMap()).Parse(push.PathTemplate)
	if err != nil {
		return nil, err
	}

	return func(pushPrefix, pushPath, name string) (string, error) {
		var tout bytes.Buffer
		err = tpl.Execute(&tout, struct{ Prefix, Path, Name string }{pushPrefix, pushPath, name})
		if err != nil {
			return "", err
		}
		return tout.String(), nil
	}, nil
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
				if api.config.DryRun {
					log.Infof("[DRY-RUN] PULLED %s", ref)
					done <- nil
					continue
				}

				resp, err := api.dockerClient.Pull(ref)
				if err != nil {
					done <- err
					return
				}

				logDebugData(resp)

				done <- nil
			}
		}(repo, tags, done)

		time.Sleep(api.config.WaitBetween)
	}

	return wait.WithTolerance(done)
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

	pushPathTemplate, terr := makePushPathTemplate(push)
	if terr != nil {
		return terr
	}
	pushTagTemplate, terr := makePushTagTemplate(push)
	if terr != nil {
		return terr
	}

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
				pushPrefix := getPushPrefix(push.Prefix, repo.PushPrefix())
				pushPath := repo.PushPath(push.PathSeparator)
				fullPath, perr := pushPathTemplate(pushPrefix, pushPath, repo.Name())
				if perr != nil {
					done <- perr
					return
				}
				tagName, err := pushTagTemplate(pushPrefix, pushPath, repo.Name(), tg.Name())
				if err != nil {
					done <- err
					return
				}
				dstRef := push.Registry + fullPath + ":" + tagName

				log.Infof("[PULL/PUSH] PUSHING %s => %s", srcRef, dstRef)
				if api.config.DryRun {
					log.Infof("[DRY-RUN] PUSHED %s => %s", srcRef, dstRef)
					done <- nil
					continue
				}

				pullResp, err := api.dockerClient.Pull(srcRef)
				if err != nil {
					done <- err
					return
				}
				logDebugData(pullResp)

				api.dockerClient.Tag(srcRef, dstRef)

				pushResp, err := api.dockerClient.Push(dstRef)
				if err != nil {
					done <- err
					return
				}
				logDebugData(pushResp)

				done <- err
			}
		}(repo, tags, done)

		time.Sleep(api.config.WaitBetween)
	}

	return wait.WithTolerance(done)
}

func makePushTagTemplate(push PushConfig) (func(pushPrefix, pushPath, name, tag string) (string, error), error) {
	tpl, err := template.New("push-tag-template").
		Funcs(sprig.FuncMap()).Parse(push.TagTemplate)
	if err != nil {
		return nil, err
	}

	return func(pushPrefix, pushPath, name, tag string) (string, error) {
		var tout bytes.Buffer
		err = tpl.Execute(&tout, struct{ Prefix, Path, Name, Tag string }{pushPrefix, pushPath, name, tag})
		if err != nil {
			return "", err
		}
		return tout.String(), nil
	}, nil
}

func logDebugData(data io.Reader) {
	scanner := bufio.NewScanner(data)
	for scanner.Scan() {
		log.Debug(scanner.Text())
	}
}

// New creates new instance of application API
func New(config Config) (*API, error) {
	if config.VerboseLogging {
		log.SetLevel(log.DebugLevel)
	}
	log.Debugf("%s API config: %+v", fn(), config)

	if config.ConcurrentRequests == 0 {
		config.ConcurrentRequests = 8
	}
	remote.ConcurrentRequests = config.ConcurrentRequests
	remote.WaitBetween = config.WaitBetween
	remote.TraceRequests = config.TraceRequests
	remote.RetryRequests = config.RetryRequests
	remote.RetryDelay = config.RetryDelay

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
