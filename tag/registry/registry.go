package registry

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
)

var TraceRequests = false

func getAuthorizationType(authorization string) string {
	return strings.Split(authorization, " ")[0]
}

func getRequestID() string {
	data := make([]byte, 10)

	for i := range data {
		data[i] = byte(rand.Intn(256))
	}

	return fmt.Sprintf("%x", sha256.Sum256(data))[0:7]
}

func httpResponseBody(resp *http.Response) string {
	b, _ := ioutil.ReadAll(resp.Body)
	resp.Body = ioutil.NopCloser(bytes.NewBuffer(b))

	return string(b)
}

func httpRequest(url, authorization string) (*http.Response, error) {
	hc := &http.Client{}
	rid := getRequestID()

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", authorization)

	req.Header.Set("Accept", "application/json")
	req.Header.Add("Accept", "application/vnd.docker.distribution.manifest.v2+json")

	resp, err := hc.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		return nil, errors.New("Bad response status: " + resp.Status + " >> " + url)
	}

	if TraceRequests {
		fmt.Printf("%s|@URL: %s\n", rid, url)
		for k, v := range resp.Header {
			fmt.Printf("%s|@HEADER: %-40s = %s\n", rid, k, v)
		}
		fmt.Printf("%s|--- BODY BEGIN ---\n", rid)
		for _, line := range strings.Split(httpResponseBody(resp), "\n") {
			fmt.Printf("%s|%s\n", rid, line)
		}
		fmt.Printf("%s|--- BODY END ---\n", rid)
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
		return "", errors.New("header 'Docker-Content-Digest' not found in HTTP response")
	}

	return repoDigest[0], nil
}

type digestResponse struct {
	TagName    string
	RepoDigest string
	Error      error
}

func validateConcurrency(concurrency int) (int, error) {
	const min = 1
	const max = 128

	if concurrency < min {
		return 0, errors.New("Concurrency could not be lower than " + strconv.Itoa(min))
	}

	if concurrency > max {
		return 0, errors.New("Concurrency could not be higher than " + strconv.Itoa(max))
	}

	return concurrency, nil
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

// FetchTags looks up Docker repo tags present on remote Docker registry
func FetchTags(registry, repo, authorization string, concurrency int) (map[string]string, error) {
	batchLimit, err := validateConcurrency(concurrency)
	if err != nil {
		return nil, err
	}

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
