package jsonconfig

import (
	"testing"
)

const configFile = "../../fixtures/docker/config.json"
const irrelevantConfigFile = "../../fixtures/docker/config.json.irrelevant"
const invalidConfigFile = "../../fixtures/docker/config.json.invalid"

func getExpectedValues() map[string]string {
	values := make(map[string]string)

	values["registry.company.io"] = "user1:pass1"
	values["registry.hub.docker.com"] = "user2:pass2"

	return values
}

func TestLoad(t *testing.T) {
	c, err := Load(configFile)

	if err != nil {
		t.Fatalf("Error while loading '%s': %s", configFile, err.Error())
	}

	for registry, expected := range getExpectedValues() {
		username, password, defined := c.GetCredentials(registry)

		if !defined {
			t.Fatalf("Unable to get credentials from registry: %s", registry)
		}

		value := username + ":" + password

		if value != expected {
			t.Fatalf(
				"Unexpected 'username:password' for registry '%s': '%s' (expected: '%s')",
				registry,
				value,
				expected,
			)
		}
	}

	if c.IsEmpty() {
		t.Fatalf("Expected to load data set from config file: %s", configFile)
	}
}

func TestLoadWithIrrelevantConfigFile(t *testing.T) {
	c, err := Load(irrelevantConfigFile)

	if err != nil {
		t.Fatalf("Expected to not fail while loading irrelevant config file '%s'\nLoaded: %#v", irrelevantConfigFile, c)
	}

	if !c.IsEmpty() {
		t.Fatalf("Expected to load empty data set from irrelevant config file '%s'\nLoaded: %#v", irrelevantConfigFile, c)
	}
}

func TestLoadWithInvalidConfigFile(t *testing.T) {
	c, err := Load(invalidConfigFile)

	if err == nil {
		t.Fatalf("Expected to fail while loading invalid config file '%s'\nLoaded: %#v", invalidConfigFile, c)
	}
}

func TestLoadWithAbsentConfigFile(t *testing.T) {
	_, err := Load("i/exist/only/in/your/magination")

	if err == nil {
		t.Fatalf("Expected to fail while trying to load absent config file")
	}
}
