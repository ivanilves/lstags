package jsonconfig

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"strings"
)

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
	_, defined := c.usernames[registry]
	if !defined {
		return "", "", false
	}

	return c.usernames[registry], c.passwords[registry], true
}

// Load loads a Config object from Docker JSON configuration file specified
func Load(fileName string) (*Config, error) {
	fileName = fixPath(fileName)

	f, err := os.Open(fileName)
	defer f.Close()
	if err != nil {
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

		usernameAndPassword := strings.Split(string(b), ":")

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

func fixPath(path string) string {
	return strings.Replace(path, "~", os.Getenv("HOME"), 1)
}
