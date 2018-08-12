package bearer

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

// Token implementation for Bearer authentication
type Token struct {
	T string `json:"token"`
	E int    `json:"expires_in"`
}

// Method is set to "Bearer"
func (tk Token) Method() string {
	return "Bearer"
}

// String form of Bearer token
func (tk Token) String() string {
	return tk.T
}

// ExpiresIn token lifetime in seconds
func (tk Token) ExpiresIn() int {
	return tk.E
}

func decodeTokenResponse(data io.ReadCloser) (*Token, error) {
	tk := Token{}

	err := json.NewDecoder(data).Decode(&tk)
	if err != nil {
		return nil, err
	}

	return &tk, nil
}

// RequestToken requests Bearer token from authentication service
func RequestToken(username, password string, params map[string]string) (*Token, error) {
	url := params["realm"] + "?service=" + params["service"] + "&scope=" + params["scope"]

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
