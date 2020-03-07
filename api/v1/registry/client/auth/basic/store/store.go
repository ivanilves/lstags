package store

import (
	"fmt"
	"net/url"
	"strings"
)

// Store stores BASIC authentication credentials
type Store struct {
	logins map[string]*Login
}

// Login stores username and password for BASIC authentication
type Login struct {
	Username string
	Password string
}

// LoadAll parses and loads a list of BASIC authentication strings
func (st *Store) LoadAll(aa []string) error {
	logins := make(map[string]*Login, 0)

	for _, a := range aa {
		registry, login, err := loadOne(strings.TrimSpace(a))

		if err != nil {
			return err
		}

		logins[registry] = login
	}

	st.logins = logins

	return nil
}

// GetByHostname gets a BASIC auth login for a registry hostname passed
func (st *Store) GetByHostname(registryHostname string) *Login {
	login, defined := st.logins[registryHostname]
	if !defined {
		return nil
	}

	return login
}

// GetByURL gets a BASIC auth login for a registry URL passed
func (st *Store) GetByURL(registryURL string) *Login {
	u, _ := url.Parse(registryURL)

	return st.GetByHostname(u.Host)
}

func loadOne(a string) (string, *Login, error) {
	const format = "REGISTRY[:PORT] username:password"

	var formatErr = fmt.Errorf(
		"invalid format for BASIC auth (should be: %s)",
		format,
	)

	ss := strings.SplitN(a, " ", 2)
	if len(ss) != 2 {
		return "", nil, formatErr
	}

	up := strings.SplitN(ss[1], ":", 2)
	if len(up) != 2 {
		return "", nil, formatErr
	}

	registry := ss[0]
	username := up[0]
	password := up[1]

	if password == "" {
		return "", nil, formatErr
	}

	return registry, &Login{Username: username, Password: password}, nil
}
