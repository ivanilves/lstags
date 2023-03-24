package request

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"strings"
	"time"
)

func getRequestID() string {
	data := make([]byte, 10)

	for i := range data {
		data[i] = byte(rand.Intn(256))
	}

	return fmt.Sprintf("%x", sha256.Sum256(data))[0:7]
}

func getResponseBody(resp *http.Response) string {
	b, _ := ioutil.ReadAll(resp.Body)
	resp.Body = ioutil.NopCloser(bytes.NewBuffer(b))

	return string(b)
}

func perform(url, auth, mode string, trace bool) (resp *http.Response, err error) {
	hc := &http.Client{}
	rid := getRequestID()

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", auth)
	req.Header.Set("Accept", "application/json")

	switch mode {
	case "v1":
		req.Header.Add("Accept", "application/vnd.docker.distribution.manifest.v1+json")
	case "v2":
		req.Header.Add("Accept", "application/vnd.docker.distribution.manifest.v2+json")
		req.Header.Add("Accept", "application/vnd.oci.image.index.v1+json")
		req.Header.Add("Accept", "application/vnd.oci.image.manifest.v1+json")
	default:
		return nil, errors.New("Unknown request mode: " + mode)
	}

	resp, err = hc.Do(req)
	if err != nil {
		return nil, err
	}

	if trace {
		fmt.Printf("%s|@URL: %s\n", rid, url)
		for k, v := range req.Header {
			fmt.Printf("%s|@REQ-HEADER: %-40s = %s\n", rid, k, v)
		}
		for k, v := range resp.Header {
			fmt.Printf("%s|@RESP-HEADER: %-40s = %s\n", rid, k, v)
		}
		fmt.Printf("%s|--- BODY BEGIN ---\n", rid)
		for _, line := range strings.Split(getResponseBody(resp), "\n") {
			fmt.Printf("%s|%s\n", rid, line)
		}
		fmt.Printf("%s|--- BODY END ---\n", rid)
	}

	if resp.StatusCode != 200 && resp.StatusCode != 404 {
		return resp, errors.New("Bad response status: " + resp.Status + " >> " + url)
	}

	return resp, nil
}

// Perform performs the required HTTP(S) request, retrying if applicable
func Perform(url, auth, mode string, trace bool, retries int, delay time.Duration) (resp *http.Response, nextlink string, err error) {
	tries := 1

	if retries > 0 {
		tries = tries + retries
	}

	for try := 1; try <= tries; try++ {
		resp, err := perform(url, auth, mode, trace)

		if err == nil {
			return resp, getNextLink(resp.Header["Link"]), nil
		}

		if resp != nil {
			if resp.StatusCode != 429 && resp.StatusCode >= 400 && resp.StatusCode < 500 {
				return nil, "", err
			}
		}

		if try < tries {
			fmt.Printf(
				"Will retry '%s' [%s] in a %v\n=> Error: %s\n",
				url,
				mode,
				delay,
				err.Error(),
			)

			time.Sleep(delay)

			delay += delay
		}
	}

	return resp, getNextLink(resp.Header["Link"]), err
}

func getNextLink(headers []string) string {
	if len(headers) == 0 {
		return ""
	}

	nextlink := headers[0]

	nextlink = strings.Split(nextlink, "?")[1]
	nextlink = strings.Split(nextlink, ">")[0]

	return nextlink
}
