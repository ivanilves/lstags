package basic

import (
	"errors"
	"net/http"
	"strings"
)

// Token implementation for Basic authentication
type Token struct {
	T string
}

// Method is set to "Basic"
func (tk Token) Method() string {
	return "Basic"
}

// String form of Basic token
func (tk Token) String() string {
	return tk.T
}

// ExpiresIn is set to 0 for Basic authentication
func (tk Token) ExpiresIn() int {
	return 0
}

func getTokenFromHeader(header string) string {
	fields := strings.Split(header, " ")

	return fields[1]
}

// RequestToken performs Basic authentication and extracts token from response header
func RequestToken(url, username, password string) (*Token, error) {
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

	return &Token{T: getTokenFromHeader(req.Header["Authorization"][0])}, nil
}
