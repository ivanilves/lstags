package bearer

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

// TokenResponse implementation for Bearer authentication
type TokenResponse struct {
	T string `json:"token"`
	E int    `json:"expires_in"`
}

// Method is set to "Bearer"
func (tr TokenResponse) Method() string {
	return "Bearer"
}

// Token Bearer token
func (tr TokenResponse) Token() string {
	return tr.T
}

// ExpiresIn token lifetime in seconds
func (tr TokenResponse) ExpiresIn() int {
	return tr.E
}

// AuthHeader returns contents of the Authorization HTTP header
func (tr TokenResponse) AuthHeader() string {
	return tr.Method() + " " + tr.Token()
}

func decodeTokenResponse(data io.ReadCloser) (*TokenResponse, error) {
	tr := TokenResponse{}

	err := json.NewDecoder(data).Decode(&tr)
	if err != nil {
		return nil, err
	}

	return &tr, nil
}

// RequestToken requests Bearer token from authentication service
func RequestToken(realm, service, repository, username, password string) (*TokenResponse, error) {
	url := realm + "?service=" + service + "&scope=repository:" + repository + ":pull"

	hc := &http.Client{}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	if username != "" && password != "" {
		req.SetBasicAuth(username, password)
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
