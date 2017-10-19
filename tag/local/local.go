package local

import (
	"strings"

	dockerclient "github.com/ivanilves/lstags/docker/client"
	"github.com/ivanilves/lstags/tag"
)

// FetchTags looks up Docker repo tags and IDs present on local Docker daemon
func FetchTags(repoName string, dc *dockerclient.DockerClient) (map[string]*tag.Tag, error) {
	imageSummaries, err := dc.ListImagesForRepo(repoName)
	if err != nil {
		return nil, err
	}

	tags := make(map[string]*tag.Tag)

	for _, imageSummary := range imageSummaries {
		repoDigest := extractRepoDigest(imageSummary.RepoDigests)
		tagNames := extractTagNames(imageSummary.RepoTags, repoName)

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

func extractRepoDigest(repoDigests []string) string {
	if len(repoDigests) == 0 {
		return ""
	}

	digestString := repoDigests[0]
	digestFields := strings.Split(digestString, "@")

	return digestFields[1]
}

func extractTagNames(repoTags []string, repoName string) []string {
	tagNames := make([]string, 0)

	for _, tag := range repoTags {
		if strings.HasPrefix(tag, repoName+":") {
			fields := strings.Split(tag, ":")
			tagNames = append(tagNames, fields[len(fields)-1])
		}
	}

	return tagNames
}
