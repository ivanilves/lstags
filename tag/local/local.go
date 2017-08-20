package local

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	// This "Moby" thing does not work for me...
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"

	"github.com/tv42/httpunix"
	"golang.org/x/net/context"
)

const dockerSocket = "/var/run/docker.sock"

type apiVersionResponse struct {
	ApiVersion string `json:"ApiVersion"`
}

func getApiTransport() *httpunix.Transport {
	t := &httpunix.Transport{
		DialTimeout:           200 * time.Millisecond,
		RequestTimeout:        2 * time.Second,
		ResponseHeaderTimeout: 2 * time.Second,
	}
	t.RegisterLocation("docker", dockerSocket)

	return t
}

func parseApiVersionJSON(data io.ReadCloser) (string, error) {
	v := apiVersionResponse{}

	err := json.NewDecoder(data).Decode(&v)
	if err != nil {
		return "", err
	}

	return v.ApiVersion, nil
}

func detectApiVersion() (string, error) {
	hc := http.Client{Transport: getApiTransport()}

	resp, err := hc.Get("http+unix://docker/version")
	if err != nil {
		return "", err
	}

	return parseApiVersionJSON(resp.Body)
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

func FetchTags(repo string) (map[string]string, error) {
	ctx := context.Background()

	apiVersion, err := detectApiVersion()
	if err != nil {
		return nil, err
	}

	cli, err := client.NewClient("unix://"+dockerSocket, apiVersion, nil, nil)
	if err != nil {
		return nil, err
	}

	listOptions, err := newImageListOptions(repo)
	if err != nil {
		return nil, err
	}
	imageSummaries, err := cli.ImageList(ctx, listOptions)
	if err != nil {
		return nil, err
	}

	tags := make(map[string]string)

	for _, imageSummary := range imageSummaries {
		repoDigest := extractRepoDigest(imageSummary.RepoDigests)
		tagNames := extractTagNames(imageSummary.RepoTags, repo)

		for _, tagName := range tagNames {
			tags[tagName] = repoDigest
		}
	}

	return tags, nil
}
