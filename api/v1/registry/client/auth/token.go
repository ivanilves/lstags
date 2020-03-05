package auth

import (
	"errors"
	"net/http"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/ivanilves/lstags/api/v1/registry/client/auth/basic"
	basicstore "github.com/ivanilves/lstags/api/v1/registry/client/auth/basic/store"
	"github.com/ivanilves/lstags/api/v1/registry/client/auth/bearer"
	"github.com/ivanilves/lstags/api/v1/registry/client/auth/none"
)

// BasicStore stores explicitly set BASIC authorization headers
var BasicStore basicstore.Store

// Token is an abstraction for aggregated token-related information we get from authentication services
type Token interface {
	Method() string
	String() string
	ExpiresIn() int
}

type authHeader string

func extractAuthHeader(hh []string) (authHeader, error) {
	if len(hh) == 0 {
		return "None realm=none", nil
	}

	h := hh[0]

	if len(strings.SplitN(h, " ", 2)) != 2 {
		return "", errors.New("Unexpected 'Www-Authenticate' header: " + h)
	}

	return authHeader(h), nil
}

func getAuthMethod(h authHeader) string {
	return strings.SplitN(string(h), " ", 2)[0]
}

func getAuthParams(h authHeader) map[string]string {
	params := make(map[string]string)

	paramString := strings.SplitN(string(h), " ", 2)[1]

	for _, keyValueString := range strings.Split(paramString, ",") {
		kv := strings.Split(keyValueString, "=")
		if len(kv) == 2 {
			params[kv[0]] = strings.Trim(kv[1], "\"")
		}
	}

	return params
}

// NewToken creates a new instance of Token in two steps:
// * detects authentication type ("Bearer", "Basic" or "None")
// * delegates actual authentication to the type-specific implementation
func NewToken(url, username, password, scope string) (Token, error) {
	var method = ""
	var params = make(map[string]string)

	storedBasicAuth := BasicStore.GetByURL(url)

	if storedBasicAuth == nil {
		resp, err := http.Get(url)
		if err != nil {
			return nil, err
		}

		authHeader, err := extractAuthHeader(resp.Header["Www-Authenticate"])
		if err != nil {
			return nil, err
		}

		method = strings.ToLower(getAuthMethod(authHeader))
		params = getAuthParams(authHeader)
	} else {
		method = "basic"

		username = storedBasicAuth.Username
		password = storedBasicAuth.Password
	}

	switch method {
	case "none":
		return none.RequestToken()
	case "basic":
		t, err := basic.RequestToken(url, username, password)
		if err != nil {
			log.Debug(err.Error())

			return none.RequestToken()
		}

		return t, nil
	case "bearer":
		params["scope"] = scope
		return bearer.RequestToken(username, password, params)
	default:
		return nil, errors.New("Unknown authentication method: " + method)
	}
}
