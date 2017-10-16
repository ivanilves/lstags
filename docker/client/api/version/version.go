package version

import (
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/tv42/httpunix"
)

// Detect detects Docker API version through the passed Docker socket
func Detect(dockerSocket string) (string, error) {
	hc := http.Client{Transport: getTransport(dockerSocket)}

	resp, err := hc.Get("http+unix://docker/version")
	if err != nil {
		return "", err
	}

	return parseJSON(resp.Body)
}

func getTransport(dockerSocket string) *httpunix.Transport {
	t := &httpunix.Transport{
		DialTimeout:           200 * time.Millisecond,
		RequestTimeout:        2 * time.Second,
		ResponseHeaderTimeout: 2 * time.Second,
	}
	t.RegisterLocation("docker", dockerSocket)

	return t
}

func parseJSON(data io.ReadCloser) (string, error) {
	v := struct {
		APIVersion string `json:"ApiVersion"`
	}{}

	err := json.NewDecoder(data).Decode(&v)
	if err != nil {
		return "", err
	}

	return v.APIVersion, nil
}
