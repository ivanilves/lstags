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

	"github.com/ivanilves/lstags/tag"
	"github.com/ivanilves/lstags/util"
)

// WebSchema defines how do we connect to remote web servers
const WebSchema = "https://"

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
	url := WebSchema + registry + "/v2/" + repo + "/tags/list"

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

	return imageMetadata{t.Unix(), history.ContainerID}, nil
}

func fetchMetadata(url, authorization string) (imageMetadata, error) {
	resp, err := httpRequest(url, authorization, "v1")
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
	resp, err := httpRequest(url, authorization, "v2")
	if err != nil {
		return "", err
	}

	digests, defined := resp.Header["Docker-Content-Digest"]
	if !defined {
		return "", errors.New("header 'Docker-Content-Digest' not found in HTTP response")
	}

	return digests[0], nil
}

func fetchDetails(registry, repo, tagName, authorization string) (string, imageMetadata, error) {
	url := WebSchema + registry + "/v2/" + repo + "/manifests/" + tagName

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

func validateConcurrentRequests(concurrentRequests int) (int, error) {
	const min = 1
	const max = 128

	if concurrentRequests < min {
		return 0, errors.New("Concurrent requests limit could not be lower than " + strconv.Itoa(min))
	}

	if concurrentRequests > max {
		return 0, errors.New("Concurrent requests limit could not be higher than " + strconv.Itoa(max))
	}

	return concurrentRequests, nil
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
func FetchTags(registry, repo, authorization string, concurrentRequests int, filter string) (map[string]*tag.Tag, error) {
	batchLimit, err := validateConcurrentRequests(concurrentRequests)
	if err != nil {
		return nil, err
	}

	allTagNames, err := fetchTagNames(registry, repo, authorization)
	if err != nil {
		return nil, err
	}

	tagNames := make([]string, 0)
	for _, tagName := range allTagNames {
		if util.DoesMatch(tagName, filter) {
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
			go func(registry, repo, tagName, authorization string, ch chan detailResponse) {
				digest, metadata, err := fetchDetails(registry, repo, tagName, authorization)

				ch <- detailResponse{TagName: tagName, Digest: digest, Created: metadata.Created, ContainerID: metadata.ContainerID, Error: err}
			}(registry, repo, tagNames[tagIndex], authorization, ch)

			tagIndex++
		}

		for s := 1; s <= stepSize; s++ {
			dr := <-ch

			if dr.Error != nil {
				return nil, dr.Error
			}

			tt, err := tag.New(dr.TagName, dr.Digest)
			if err != nil {
				return nil, err
			}

			tt.SetCreated(dr.Created)
			tt.SetContainerID(dr.ContainerID)

			tags[tt.GetName()] = tt
		}
	}

	return tags, nil
}
