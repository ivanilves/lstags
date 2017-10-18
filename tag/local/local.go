package local

import (
	"strings"

	"github.com/docker/docker/api/types"

	"github.com/ivanilves/lstags/tag"
)

// FetchTags looks up Docker repo tags and IDs present on local Docker daemon
func FetchTags(repo string, imageSummaries []types.ImageSummary) (map[string]*tag.Tag, error) {
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
			tagNames = append(tagNames, fields[len(fields)-1])
		}
	}

	return tagNames
}
