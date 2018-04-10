package config

import (
	"testing"
)

var configFile = "../../fixtures/docker/config.json"

func TestGetRegistryAuth(t *testing.T) {
	examples := map[string]string{
		"registry.company.io":     "eyAidXNlcm5hbWUiOiAidXNlcjEiLCAicGFzc3dvcmQiOiAicGFzczEiIH0=",
		"registry.hub.docker.com": "eyAidXNlcm5hbWUiOiAidXNlcjIiLCAicGFzc3dvcmQiOiAicGFzczIiIH0=",
		"registry.mindundi.org":   "",
	}

	c, err := Load(configFile)

	if err != nil {
		t.Fatalf("Error while loading '%s': %s", configFile, err.Error())
	}

	for registry, expectedAuth := range examples {
		auth := c.GetRegistryAuth(registry)

		if auth != expectedAuth {
			t.Fatalf(
				"Unexpected authentication string for registry '%s': %s (expected: %s)",
				registry,
				auth,
				expectedAuth,
			)
		}
	}
}

func TestLoad(t *testing.T) {
	examples := map[string]string{
		"registry.company.io":     "user1:pass1",
		"registry.hub.docker.com": "user2:pass2",
	}

	c, err := Load(configFile)

	if err != nil {
		t.Fatalf("Error while loading '%s': %s", configFile, err.Error())
	}

	for registry, expected := range examples {
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
	irrelevantConfigFile := "../../fixtures/docker/config.json.irrelevant"

	c, err := Load(irrelevantConfigFile)

	if err != nil {
		t.Fatalf("Expected to not fail while loading irrelevant config file '%s'\nLoaded: %#v", irrelevantConfigFile, c)
	}

	if !c.IsEmpty() {
		t.Fatalf("Expected to load empty data set from irrelevant config file '%s'\nLoaded: %#v", irrelevantConfigFile, c)
	}
}

func TestLoadWithInvalidConfigFile(t *testing.T) {
	invalidConfigFile := "../../fixtures/docker/config.json.invalid"

	c, err := Load(invalidConfigFile)

	if err == nil {
		t.Fatalf("Expected to fail while loading invalid config file '%s'\nLoaded: %#v", invalidConfigFile, c)
	}
}

func TestLoadWithAbsentConfigFile(t *testing.T) {
	var err error

	_, err = Load("i/exist/only/in/your/magination")
	if err == nil {
		t.Fatalf("Expected to fail while trying to load absent config file")
	}

	DefaultDockerJSON = "i/exist/only/in/your/magination"

	_, err = Load("i/exist/only/in/your/magination")
	if err != nil {
		t.Fatalf(
			"Expected NOT to fail while trying to load absent config file from a default path: %s",
			err.Error(),
		)
	}
}

// Badauth file = valid JSON file with auth encoded well, but NOT having username:password form
func TestLoadWithBadAuthConfigFile(t *testing.T) {
	badAuthConfigFile := "../../fixtures/docker/config.json.badauth"
	badAuthRegistry := "registry.valencia.io"

	_, err := Load(badAuthConfigFile)
	if err == nil {
		t.Fatalf("Expected to fail while loading config file with incorrect auth")
	}

	DefaultDockerJSON = badAuthConfigFile

	c, err := Load(badAuthConfigFile)
	if err != nil {
		t.Fatalf(
			"Expected NOT to fail while loading config file with incorrect auth from a default path: %s",
			err.Error(),
		)
	}

	_, _, defined := c.GetCredentials(badAuthRegistry)
	if defined {
		t.Fatalf("Should NOT get credentials from a registry record with incorrect auth: %s", badAuthRegistry)
	}
}

// Corrupt file = valid JSON file with badly encoded auth
func TestLoadWithCorruptConfigFile(t *testing.T) {
	corruptConfigFile := "../../fixtures/docker/config.json.corrupt"

	_, err := Load(corruptConfigFile)
	if err == nil {
		t.Fatalf("Expected to fail while loading corrupt config file")
	}

	DefaultDockerJSON = corruptConfigFile

	if _, err := Load(corruptConfigFile); err == nil {
		t.Fatalf(
			"Expected to fail while loading corrupt config file from a default path: %s",
			err.Error(),
		)
	}
}
