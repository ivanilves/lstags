package basic

import (
	"errors"
	"net/http"
	"strings"
)

type TokenResponse struct {
	T string `json:"token"`
}

func (tr TokenResponse) Method() string {
	return "Basic"
}

func (tr TokenResponse) Token() string {
	return tr.T
}

func (tr TokenResponse) ExpiresIn() int {
	return 0
}

func getTokenFromHeader(header string) string {
	fields := strings.Split(header, " ")

	return fields[1]
}

func RequestToken(url, username, password string) (*TokenResponse, error) {
	hc := &http.Client{}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.SetBasicAuth(username, password)

	resp, err := hc.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 && resp.StatusCode != 403 {
		return nil, errors.New("[AUTH::BASIC] Bad response status: " + resp.Status + " >> " + url)
	}

	return &TokenResponse{T: getTokenFromHeader(req.Header["Authorization"][0])}, nil
}
