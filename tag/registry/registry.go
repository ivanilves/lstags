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
	"time"

	"github.com/ivanilves/lstags/tag"
)

// TraceRequests defines if we should print out HTTP request URLs and response headers/bodies
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

func httpRequest(url, authorization, mode string) (*http.Response, error) {
	hc := &http.Client{}
	rid := getRequestID()

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", authorization)
	req.Header.Set("Accept", "application/json")

	switch mode {
	case "v1":
		req.Header.Add("Accept", "application/vnd.docker.distribution.manifest.v1+json")
	case "v2":
		req.Header.Add("Accept", "application/vnd.docker.distribution.manifest.v2+json")
	default:
		return nil, errors.New("Unknown request mode: " + mode)
	}

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

	resp, err := httpRequest(url, authorization, "v2")
	if err != nil {
		return nil, err
	}

	tagNames, err := parseTagNamesJSON(resp.Body)
	if err != nil {
		return make([]string, 0), nil
	}

	return tagNames, nil
}

func extractRepoCreatedFromHistory(s string) (int64, error) {
	type v1History struct {
		Created string `json:"created"`
	}

	history := v1History{}

	err := json.Unmarshal([]byte(s), &history)
	if err != nil {
		return 0, err
	}

	t, err := time.Parse(time.RFC3339, history.Created)

	return t.Unix(), nil
}

func fetchRepoDetails(registry, repo, tagName, authorization string) (string, int64, error) {
	url := "https://" + registry + "/v2/" + repo + "/manifests/" + tagName

	// First, we extract digest from first HTTP request
	respD, errD := httpRequest(url, authorization, "v2")
	if errD != nil {
		return "[" + errD.Error() + "]", 0, nil
	}

	digests, defined := respD.Header["Docker-Content-Digest"]
	if !defined {
		return "", 0, errors.New("header 'Docker-Content-Digest' not found in HTTP response")
	}

	// Second, we try to extract created date from second HTTP request
	type manifestResponse struct {
		History []map[string]string `json:"history"`
	}

	respC, errC := httpRequest(url, authorization, "v1")
	if errC != nil {
		return digests[0], -1, nil
	}

	mr := manifestResponse{}

	err := json.NewDecoder(respC.Body).Decode(&mr)
	if err != nil {
		return digests[0], -2, err
	}

	var created int64 = 0

	if len(mr.History) > 0 {
		created, err = extractRepoCreatedFromHistory(mr.History[0]["v1Compatibility"])
		if err != nil {
			created = -3
		}

	}

	//
	// NB! This function is a total shame and needs to be refactored!
	//

	return digests[0], created, nil
}

type detailResponse struct {
	TagName    string
	RepoDigest string
	Created    int64
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
func FetchTags(registry, repo, authorization string, concurrency int) (map[string]*tag.Tag, error) {
	batchLimit, err := validateConcurrency(concurrency)
	if err != nil {
		return nil, err
	}

	tagNames, err := fetchTagNames(registry, repo, authorization)
	if err != nil {
		return nil, err
	}

	tags := make(map[string]*tag.Tag)

	batchSteps, batchRemain := calculateBatchSteps(len(tagNames), batchLimit)

	var stepSize int
	var tagIndex = 0
	for b := 1; b <= batchSteps; b++ {
		stepSize = calculateBatchStepSize(b, batchSteps, batchRemain, batchLimit)

		ch := make(chan detailResponse, stepSize)

		for s := 1; s <= stepSize; s++ {
			go func(registry, repo, tagName, authorization string, ch chan detailResponse) {
				digest, created, err := fetchRepoDetails(registry, repo, tagName, authorization)

				ch <- detailResponse{TagName: tagName, RepoDigest: digest, Created: created, Error: err}
			}(registry, repo, tagNames[tagIndex], authorization, ch)

			tagIndex++
		}

		for s := 1; s <= stepSize; s++ {
			dr := <-ch

			if dr.Error != nil {
				return nil, dr.Error
			}

			tt, err := tag.New(dr.TagName, dr.RepoDigest)
			if err != nil {
				return nil, err
			}

			tt.SetCreated(dr.Created)

			tags[tt.SortKey()] = tt
		}
	}

	return tags, nil
}

// FormatRepoName formats repository name for use with Docker registry
func FormatRepoName(repository, registry string) string {
	if !strings.Contains(repository, "/") {
		return "library/" + repository
	}

	if strings.HasPrefix(repository, registry) {
		return strings.Replace(repository, registry+"/", "", 1)
	}

	return repository
}
