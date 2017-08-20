package bearer

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

type TokenResponse struct {
	T string `json:"token"`
	E int    `json:"expires_in"`
}

func (tr TokenResponse) Method() string {
	return "Bearer"
}

func (tr TokenResponse) Token() string {
	return tr.T
}

func (tr TokenResponse) ExpiresIn() int {
	return tr.E
}

func decodeTokenResponse(data io.ReadCloser) (*TokenResponse, error) {
	tr := TokenResponse{}

	err := json.NewDecoder(data).Decode(&tr)
	if err != nil {
		return nil, err
	}

	return &tr, nil
}

func RequestToken(realm, service, repository string) (*TokenResponse, error) {
	url := realm + "?service=" + service + "&scope=repository:" + repository + ":pull"

	hc := &http.Client{}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := hc.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		return nil, errors.New("[AUTH::BEARER] Bad response status: " + resp.Status + " >> " + url)
	}

	return decodeTokenResponse(resp.Body)
}
