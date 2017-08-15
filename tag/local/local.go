package local

import (
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/moby/moby/client"
	"golang.org/x/net/context"
)

func getImageListOptions(repo string) (types.ImageListOptions, error) {
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

func GetTags(repo string) (map[string]string, error) {
	ctx := context.Background()
	cli, err := client.NewEnvClient()
	if err != nil {
		return nil, err
	}

	listOptions, err := getImageListOptions(repo)
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
