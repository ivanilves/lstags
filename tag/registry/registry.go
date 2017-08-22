package registry

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
)

func getAuthorizationType(authorization string) string {
	return strings.Split(authorization, " ")[0]
}

func httpRequest(url, authorization string) (*http.Response, error) {
	hc := &http.Client{}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", authorization)

	if getAuthorizationType(authorization) != "Basic" {
		req.Header.Set("Accept", "application/vnd.docker.distribution.manifest.v2+json")
	}

	resp, err := hc.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		return nil, errors.New("Bad response status: " + resp.Status + " >> " + url)
	}

	return resp, nil
}

type tagNameInfo struct {
	TagNames []string `json:"tags"`
}

func parseTagNamesJSON(data io.ReadCloser) ([]string, error) {
	tn := tagNameInfo{}

	err := json.NewDecoder(data).Decode(&tn)
	if err != nil {
		return nil, err
	}

	return tn.TagNames, nil
}

func fetchTagNames(registry, repo, authorization string) ([]string, error) {
	url := "https://" + registry + "/v2/" + repo + "/tags/list"

	resp, err := httpRequest(url, authorization)
	if err != nil {
		return nil, err
	}

	tagNames, err := parseTagNamesJSON(resp.Body)
	if err != nil {
		return make([]string, 0), nil
	}

	return tagNames, nil
}

func fetchRepoDigest(registry, repo, tagName, authorization string) (string, error) {
	url := "https://" + registry + "/v2/" + repo + "/manifests/" + tagName

	resp, err := httpRequest(url, authorization)
	if err != nil {
		return "[" + err.Error() + "]", nil
	}

	repoDigest, defined := resp.Header["Docker-Content-Digest"]
	if !defined {
		return "", errors.New("HTTP header 'Docker-Content-Digest' not found in response.")
	}

	return repoDigest[0], nil
}

const batchLimit = 32

type digestResponse struct {
	TagName    string
	RepoDigest string
	Error      error
}

func calculateBatchSteps(count, limit int) (int, int) {
	total := count / limit
	remain := count % limit

	if remain == 0 {
		return total, 0
	}

	return total + 1, remain
}

func calculateBatchStepSize(stepNumber, stepsTotal, remain, limit int) int {
	if remain != 0 && stepNumber == stepsTotal {
		return remain
	}

	return limit
}

func FetchTags(registry, repo, authorization string) (map[string]string, error) {
	tagNames, err := fetchTagNames(registry, repo, authorization)
	if err != nil {
		return nil, err
	}

	tags := make(map[string]string)

	batchSteps, batchRemain := calculateBatchSteps(len(tagNames), batchLimit)

	var stepSize int
	var tagIndex = 0
	for b := 1; b <= batchSteps; b++ {
		stepSize = calculateBatchStepSize(b, batchSteps, batchRemain, batchLimit)

		ch := make(chan digestResponse, stepSize)

		for s := 1; s <= stepSize; s++ {
			go func(registry, repo, tagName, authorization string, ch chan digestResponse) {
				repoDigest, err := fetchRepoDigest(registry, repo, tagName, authorization)

				ch <- digestResponse{TagName: tagName, RepoDigest: repoDigest, Error: err}
			}(registry, repo, tagNames[tagIndex], authorization, ch)

			tagIndex++
		}

		for s := 1; s <= stepSize; s++ {
			resp := <-ch

			if resp.Error != nil {
				return nil, resp.Error
			}

			tags[resp.TagName] = resp.RepoDigest
		}
	}

	return tags, nil
}
