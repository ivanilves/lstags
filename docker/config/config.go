package config

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/ivanilves/lstags/docker/config/credhelper"
	"github.com/moby/moby/api/types"

	"github.com/ivanilves/lstags/util/fix"
)

// DefaultDockerJSON is the defalt path for Docker JSON config file
var DefaultDockerJSON = "~/.docker/config.json"

// Config encapsulates configuration loaded from Docker 'config.json' file
type Config struct {
	Auths       map[string]Auth `json:"auths"`
	usernames   map[string]string
	passwords   map[string]string
	CredsStore  string            `json:"credsStore,omitempty"`
	CredHelpers map[string]string `json:"credHelpers,omitempty"`
}

// Auth contains Docker registry username and password in base64-encoded form
type Auth struct {
	B64Auth string `json:"auth"`
}

// IsEmpty return true if structure has no relevant data inside
func (c *Config) IsEmpty() bool {
	return len(c.Auths) == 0
}

// GetCredentials gets per-registry credentials from loaded Docker config
func (c *Config) GetCredentials(registry string) (string, string, bool) {
	if _, defined := c.usernames[registry]; !defined {
		username, password, err := credhelper.GetCredentials(
			registry,
			c.CredsStore,
			c.CredHelpers,
		)

		if err != nil {
			return "", "", false
		}

		return username, password, true
	}

	return c.usernames[registry], c.passwords[registry], true
}

func getAuthJSONString(username, password string) string {
	b, err := json.Marshal(types.AuthConfig{
		Username: username,
		Password: password,
	})

	// Because of the shape of the struct and inputs involved, this should never
	// happen. We preserve the non error-propagating API for callers, but want
	// some visibility into this that's better than simply swallowing the error.
	if err != nil {
		panic(err)
	}

	return string(b)
}

// GetRegistryAuth gets per-registry base64 authentication string
func (c *Config) GetRegistryAuth(registry string) string {
	username, password, defined := c.GetCredentials(registry)
	if !defined {
		return ""
	}
	authJSONString := getAuthJSONString(username, password)
	log.Debugf("JSON Auth String: %s", authJSONString)

	return base64.StdEncoding.EncodeToString(
		[]byte(authJSONString),
	)
}

// Load loads a Config object from Docker JSON configuration file specified
func Load(fileName string) (*Config, error) {
	f, err := os.Open(fix.Path(fileName))
	defer f.Close()
	if err != nil {
		if fileName == DefaultDockerJSON {
			return &Config{}, nil
		}
		return nil, err
	}

	c, err := parseConfig(f)
	if err != nil {
		return nil, err
	}

	c.usernames = make(map[string]string)
	c.passwords = make(map[string]string)
	for registry, a := range c.Auths {
		b, err := base64.StdEncoding.DecodeString(a.B64Auth)
		if err != nil {
			return nil, err
		}

		authenticationToken := string(b)
		usernameAndPassword := strings.SplitN(authenticationToken, ":", 2)

		if len(usernameAndPassword) == 2 {
			c.usernames[registry] = usernameAndPassword[0]
			c.passwords[registry] = usernameAndPassword[1]
			continue
		}

		if len(usernameAndPassword) == 1 && len(usernameAndPassword[0]) == 0 {
			// Defined but empty auth string means we will use credsStore or CredHelpers
			continue
		}

		if fileName != DefaultDockerJSON {
			errStr := "Invalid auth for Docker registry: %s\nBase64-encoded string is wrong: %s (%s)\n"
			return nil, fmt.Errorf(
				errStr,
				registry,
				a.B64Auth,
				authenticationToken,
			)
		}
	}

	return c, nil
}

func parseConfig(f *os.File) (*Config, error) {
	c := &Config{}

	err := json.NewDecoder(f).Decode(c)
	if err != nil {
		return nil, err
	}

	return c, nil
}
