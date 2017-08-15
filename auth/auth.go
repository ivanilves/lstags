package auth

import (
	"errors"
	"net/http"
	"strings"

	"github.com/ivanilves/lstags/auth/bearer"
)

type TokenResponse interface {
	Method() string
	Token() string
	ExpiresIn() int
}

func parseAuthHeader(headers http.Header) (string, string, error) {
	header, defined := headers["Www-Authenticate"]
	if !defined {
		return "", "", errors.New("Missing 'Www-Authenticate' header")
	}
	fields := strings.SplitN(header[0], " ", 2)
	if len(fields) != 2 {
		return "", "", errors.New("Unexpected 'Www-Authenticate' header: " + header[0])
	}

	return fields[0], fields[1], nil
}

func parseParamString(method string, paramString string) (map[string]string, error) {
	params := make(map[string]string)

	for _, keyValueString := range strings.Split(paramString, ",") {
		kv := strings.Split(keyValueString, "=")
		if len(kv) != 2 {
			return nil, errors.New("Could not split that into key/value pair: " + keyValueString)
		}
		params[kv[0]] = strings.Trim(kv[1], "\"")
	}

	return validateParams(method, params)
}

func validateParams(method string, params map[string]string) (map[string]string, error) {
	var defined bool

	_, defined = params["realm"]
	if !defined {
		return nil, errors.New("Required parameter not defined: realm")
	}

	_, defined = params["service"]
	if !defined && method == "Bearer" {
		return nil, errors.New("Parameter (required for 'Bearer' method) not defined: service")
	}

	return params, nil
}

func NewToken(registry, repository string) (TokenResponse, error) {
	url := registry + "/v2"

	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}

	method, paramString, err := parseAuthHeader(resp.Header)
	if err != nil {
		return nil, err
	}
	params, err := parseParamString(method, paramString)
	if err != nil {
		return nil, err
	}

	switch method {
	case "Bearer":
		return bearer.RequestToken(params["realm"], params["service"], repository)
	default:
		return nil, errors.New("Unknown authentication method: " + method)
	}
}

func NewAuthorization(registry, repository string) (string, error) {
	t, err := NewToken(registry, repository)
	if err != nil {
		return "", err
	}

	return t.Method() + " " + t.Token(), nil
}
