package registry

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

func httpRequest(url, authorization string) (*http.Response, error) {
	hc := &http.Client{}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", authorization)
	req.Header.Set("Accept", "application/vnd.docker.distribution.manifest.v2+json")

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

func getTagNames(registry, repo, authorization string) ([]string, error) {
	url := registry + "/v2/" + repo + "/tags/list"

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

func getRepoDigest(registry, repo, tagName, authorization string) (string, error) {
	url := registry + "/v2/" + repo + "/manifests/" + tagName

	resp, err := httpRequest(url, authorization)
	if err != nil {
		return "", err
	}

	repoDigest, defined := resp.Header["Docker-Content-Digest"]
	if !defined {
		return "", errors.New("HTTP header 'Docker-Content-Digest' not found in response.")
	}

	return repoDigest[0], nil
}

func GetTags(registry, repo, authorization string) (map[string]string, error) {
	tagNames, err := getTagNames(registry, repo, authorization)
	if err != nil {
		return nil, err
	}

	tags := make(map[string]string)

	for _, tagName := range tagNames {
		repoDigest, err := getRepoDigest(registry, repo, tagName, authorization)
		if err != nil {
			return nil, err
		}

		tags[tagName] = repoDigest
	}

	return tags, nil
}
