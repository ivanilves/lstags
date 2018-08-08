package remote

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

	"github.com/ivanilves/lstags/repository"
	"github.com/ivanilves/lstags/tag"
	"github.com/ivanilves/lstags/tag/remote/auth"
)

// ConcurrentRequests defines maximum number of concurrent requests we could maintain against the registry
var ConcurrentRequests = 32

// WaitBetween defines how much we will wait between batches of requests
var WaitBetween time.Duration

// RetryRequests is a number of retries we do in case of request failure
var RetryRequests = 0

// RetryDelay is a delay between retries of failed requests to the registry
var RetryDelay = 5 * time.Second

// TraceRequests defines if we should print out HTTP request URLs and response headers/bodies
var TraceRequests = false

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
		return resp, errors.New("Bad response status: " + resp.Status + " >> " + url)
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

func httpRetriableRequest(url, authorization, mode string) (*http.Response, error) {
	tries := 1

	if RetryRequests > 0 {
		tries = tries + RetryRequests
	}

	var resp *http.Response
	var err error

	for try := 1; try <= tries; try++ {
		resp, err := httpRequest(url, authorization, mode)

		if err == nil {
			return resp, nil
		}

		if resp != nil {
			if resp.StatusCode >= 400 && resp.StatusCode < 500 {
				return nil, err
			}
		}

		if try < tries {
			fmt.Printf(
				"Will retry '%s' [%s] in a %v\n=> Error: %s\n",
				url,
				mode,
				RetryDelay,
				err.Error(),
			)

			time.Sleep(RetryDelay)

			RetryDelay += RetryDelay
		}
	}

	return resp, err
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

func fetchTagNames(repo *repository.Repository, authorization string) ([]string, error) {
	url := repo.WebSchema() + repo.Registry() + "/v2/" + repo.Path() + "/tags/list"

	resp, err := httpRetriableRequest(url, authorization, "v2")
	if err != nil {
		return nil, err
	}

	tagNames, err := parseTagNamesJSON(resp.Body)
	if err != nil {
		return make([]string, 0), nil
	}

	return tagNames, nil
}

type imageMetadata struct {
	Created     int64
	ContainerID string
}

func extractMetadataFromHistory(s string) (imageMetadata, error) {
	var history struct {
		Created     string `json:"created"`
		ContainerID string `json:"container"`
	}

	err := json.Unmarshal([]byte(s), &history)
	if err != nil {
		return imageMetadata{}, err
	}

	t, err := time.Parse(time.RFC3339, history.Created)
	if err != nil {
		return imageMetadata{}, err
	}

	return imageMetadata{t.Unix(), history.ContainerID}, nil
}

func fetchMetadata(url, authorization string) (imageMetadata, error) {
	resp, err := httpRetriableRequest(url, authorization, "v1")
	if err != nil {
		return imageMetadata{}, nil
	}

	var v1manifest struct {
		History []map[string]string `json:"history"`
	}

	decodingError := json.NewDecoder(resp.Body).Decode(&v1manifest)
	if decodingError != nil {
		return imageMetadata{}, decodingError
	}

	if len(v1manifest.History) > 0 {
		metadata, err := extractMetadataFromHistory(v1manifest.History[0]["v1Compatibility"])
		if err != nil {
			return imageMetadata{}, err
		}

		return metadata, nil
	}

	return imageMetadata{}, errors.New("no source to fetch image creation date/time from")
}

func fetchDigest(url, authorization string) (string, error) {
	resp, err := httpRetriableRequest(url, authorization, "v2")
	if err != nil {
		return "", err
	}

	digests, defined := resp.Header["Docker-Content-Digest"]
	if !defined {
		return "", errors.New("header 'Docker-Content-Digest' not found in HTTP response")
	}

	return digests[0], nil
}

func fetchDetails(repo *repository.Repository, tagName, authorization string) (string, imageMetadata, error) {
	url := repo.WebSchema() + repo.Registry() + "/v2/" + repo.Path() + "/manifests/" + tagName

	dc := make(chan string, 0)
	mc := make(chan imageMetadata, 0)
	ec := make(chan error, 0)

	go func(url, authorization string, dc chan string, ec chan error) {
		digest, err := fetchDigest(url, authorization)
		if err != nil {
			ec <- err
		}

		dc <- digest
	}(url, authorization, dc, ec)

	go func(url, authorization string, mc chan imageMetadata, ec chan error) {
		metadata, err := fetchMetadata(url, authorization)
		if err != nil {
			ec <- err
		}

		mc <- metadata
	}(url, authorization, mc, ec)

	var digest string
	var metadata imageMetadata

	waitForDigest := true
	waitForMetadata := true
	for waitForDigest || waitForMetadata {
		select {
		case digest = <-dc:
			waitForDigest = false
		case metadata = <-mc:
			waitForMetadata = false
		case err := <-ec:
			if err != nil {
				return "", imageMetadata{}, err
			}
		}
	}

	return digest, metadata, nil
}

type detailResponse struct {
	TagName     string
	Digest      string
	Created     int64
	ContainerID string
	Error       error
}

func validateConcurrentRequests() (int, error) {
	const min = 1
	const max = 256

	if ConcurrentRequests < min {
		return 0, errors.New("Concurrent requests limit could not be lower than " + strconv.Itoa(min))
	}

	if ConcurrentRequests > max {
		return 0, errors.New("Concurrent requests limit could not be higher than " + strconv.Itoa(max))
	}

	return ConcurrentRequests, nil
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

// FetchTags looks up Docker repoPath tags present on remote Docker registry
func FetchTags(repo *repository.Repository, username, password string) (map[string]*tag.Tag, error) {
	batchLimit, err := validateConcurrentRequests()
	if err != nil {
		return nil, err
	}

	tr, err := auth.NewToken(repo, username, password)
	if err != nil {
		return nil, err
	}

	authorization := tr.AuthHeader()

	allTagNames, err := fetchTagNames(repo, authorization)
	if err != nil {
		return nil, err
	}

	tagNames := make([]string, 0)
	for _, tagName := range allTagNames {
		if repo.MatchTag(tagName) {
			tagNames = append(tagNames, tagName)
		}
	}

	tags := make(map[string]*tag.Tag)

	batchSteps, batchRemain := calculateBatchSteps(len(tagNames), batchLimit)

	var stepSize int
	var tagIndex = 0
	for b := 1; b <= batchSteps; b++ {
		stepSize = calculateBatchStepSize(b, batchSteps, batchRemain, batchLimit)

		ch := make(chan detailResponse, stepSize)

		for s := 1; s <= stepSize; s++ {
			go func(
				repo *repository.Repository,
				tagName, authorization string,
				ch chan detailResponse,
			) {
				digest, metadata, err := fetchDetails(repo, tagName, authorization)

				ch <- detailResponse{
					TagName: tagName,
					Digest:  digest,
					Created: metadata.Created,
					Error:   err,
				}
			}(repo, tagNames[tagIndex], authorization, ch)

			tagIndex++

			time.Sleep(WaitBetween)
		}

		for s := 1; s <= stepSize; s++ {
			dr := <-ch

			if dr.Error != nil {
				if strings.Contains(dr.Error.Error(), "404 Not Found") {
					println(dr.Error.Error())

					continue
				}

				return nil, dr.Error
			}

			tt, err := tag.New(dr.TagName, dr.Digest, tag.Options{Created: dr.Created})
			if err != nil {
				return nil, err
			}

			tags[tt.Name()] = tt
		}
	}

	return tags, nil
}
