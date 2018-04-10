package config

import (
	"errors"
	"io/ioutil"

	"gopkg.in/yaml.v2"

	"github.com/ivanilves/lstags/util/fix"
)

// Config holds repository list (e.g. loadable from YAML file)
// NB!
// There are no mapping between main.Options and config.Config!
// We could implement it, but we need to see an explicit demand
// for this feature from our stakeholders.
type Config struct {
	Repositories []string `yaml:"repositories"`
}

// LoadYAMLFile loads YAML file into Config structure
func LoadYAMLFile(path string) (*Config, error) {
	data, err := ioutil.ReadFile(fix.Path(path))
	if err != nil {
		return nil, err
	}

	structure := struct {
		ConfigRoot Config `yaml:"lstags"`
	}{}

	if err := yaml.Unmarshal([]byte(data), &structure); err != nil {
		return nil, err
	}

	if structure.ConfigRoot.Repositories == nil {
		return nil, errors.New("no repos could be loaded from: " + path)
	}

	return &structure.ConfigRoot, nil
}
