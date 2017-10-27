package basic

import (
	"errors"
	"net/http"
	"strings"
)

// TokenResponse implementation for Basic authentication
type TokenResponse struct {
	T string
}

// Method is set to "Basic"
func (tr TokenResponse) Method() string {
	return "Basic"
}

// Token Basic token
func (tr TokenResponse) Token() string {
	return tr.T
}

// ExpiresIn is set to 0 for Basic authentication
func (tr TokenResponse) ExpiresIn() int {
	return 0
}

// AuthHeader returns contents of the Authorization HTTP header
func (tr TokenResponse) AuthHeader() string {
	return tr.Method() + " " + tr.Token()
}

func getTokenFromHeader(header string) string {
	fields := strings.Split(header, " ")

	return fields[1]
}

// RequestToken performs Basic authentication and extracts token from response header
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
