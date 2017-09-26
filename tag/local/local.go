package local

import (
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	// This "Moby" thing does not work for me...
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"

	"github.com/tv42/httpunix"
	"golang.org/x/net/context"

	"github.com/ivanilves/lstags/tag"
)

const dockerSocket = "/var/run/docker.sock"

type apiVersionResponse struct {
	APIVersion string `json:"ApiVersion"`
}

func getAPITransport() *httpunix.Transport {
	t := &httpunix.Transport{
		DialTimeout:           200 * time.Millisecond,
		RequestTimeout:        2 * time.Second,
		ResponseHeaderTimeout: 2 * time.Second,
	}
	t.RegisterLocation("docker", dockerSocket)

	return t
}

func parseAPIVersionJSON(data io.ReadCloser) (string, error) {
	v := apiVersionResponse{}

	err := json.NewDecoder(data).Decode(&v)
	if err != nil {
		return "", err
	}

	return v.APIVersion, nil
}

func detectAPIVersion() (string, error) {
	hc := http.Client{Transport: getAPITransport()}

	resp, err := hc.Get("http+unix://docker/version")
	if err != nil {
		return "", err
	}

	return parseAPIVersionJSON(resp.Body)
}

func newClient() (*client.Client, error) {
	apiVersion, err := detectAPIVersion()
	if err != nil {
		return nil, err
	}

	cli, err := client.NewClient("unix://"+dockerSocket, apiVersion, nil, nil)
	if err != nil {
		return nil, err
	}

	return cli, err
}

func newImageListOptions(repo string) (types.ImageListOptions, error) {
	repoFilter := "reference=" + repo
	filterArgs := filters.NewArgs()

	filterArgs, err := filters.ParseFlag(repoFilter, filterArgs)
	if err != nil {
		return types.ImageListOptions{}, err
	}

	return types.ImageListOptions{Filters: filterArgs}, nil
}

func extractRepoDigest(repoDigests []string) string {
	if len(repoDigests) == 0 {
		return ""
	}

	digestString := repoDigests[0]
	digestFields := strings.Split(digestString, "@")

	return digestFields[1]
}

func extractTagNames(repoTags []string, repo string) []string {
	tagNames := make([]string, 0)

	for _, tag := range repoTags {
		if strings.HasPrefix(tag, repo+":") {
			fields := strings.Split(tag, ":")
			tagNames = append(tagNames, fields[1])
		}
	}

	return tagNames
}

// FetchTags looks up Docker repo tags and IDs present on local Docker daemon
func FetchTags(repo string) (map[string]*tag.Tag, error) {
	cli, err := newClient()
	if err != nil {
		return nil, err
	}

	listOptions, err := newImageListOptions(repo)
	if err != nil {
		return nil, err
	}
	imageSummaries, err := cli.ImageList(context.Background(), listOptions)
	if err != nil {
		return nil, err
	}

	tags := make(map[string]*tag.Tag)

	for _, imageSummary := range imageSummaries {
		repoDigest := extractRepoDigest(imageSummary.RepoDigests)
		tagNames := extractTagNames(imageSummary.RepoTags, repo)

		if repoDigest == "" {
			repoDigest = "this.image.is.bad.it.has.no.digest.fuuu!"
		}

		for _, tagName := range tagNames {
			tg, err := tag.New(tagName, repoDigest)
			if err != nil {
				return nil, err
			}

			tg.SetImageID(imageSummary.ID)

			tg.SetCreated(imageSummary.Created)

			tags[tg.GetName()] = tg
		}
	}

	return tags, nil
}

// FormatRepoName formats repository name for use with local Docker daemon
func FormatRepoName(repository, registry string) string {
	if registry == "registry.hub.docker.com" {
		if strings.HasPrefix(repository, "library/") {
			return strings.Replace(repository, "library/", "", 1)
		}

		return repository
	}

	if strings.HasPrefix(repository, registry) {
		return repository
	}

	return registry + "/" + repository
}

// PullImage pulls Docker image specified locally
func PullImage(ref string) error {
	cli, err := newClient()
	if err != nil {
		return err
	}

	resp, err := cli.ImagePull(context.Background(), ref, types.ImagePullOptions{})
	if err != nil {
		return err
	}

	_, err = ioutil.ReadAll(resp)

	return err
}
