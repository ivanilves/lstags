package config

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/ivanilves/lstags/util/fix"
)

// DefaultDockerJSON is the defalt path for Docker JSON config file
var DefaultDockerJSON = "~/.docker/config.json"

// Config encapsulates configuration loaded from Docker 'config.json' file
type Config struct {
	Auths     map[string]Auth `json:"auths"`
	usernames map[string]string
	passwords map[string]string
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
		return "", "", false
	}

	return c.usernames[registry], c.passwords[registry], true
}

// GetRegistryAuth gets per-registry base64 authentication string
func (c *Config) GetRegistryAuth(registry string) string {
	username, password, defined := c.GetCredentials(registry)
	if !defined {
		return ""
	}

	jsonString := fmt.Sprintf(`{ "username": "%s", "password": "%s" }`, username, password)

	return base64.StdEncoding.EncodeToString([]byte(jsonString))
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
		usernameAndPassword := strings.Split(authenticationToken, ":")

		if len(usernameAndPassword) != 2 {
			if fileName != DefaultDockerJSON {
				errStr := "Invalid auth for Docker registry: %s\nBase64-encoded string is wrong: %s (%s)\n"

				return nil, errors.New(
					fmt.Sprint(
						errStr,
						registry,
						a.B64Auth,
						authenticationToken,
					),
				)
			}

			continue
		}

		c.usernames[registry] = usernameAndPassword[0]
		c.passwords[registry] = usernameAndPassword[1]
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
