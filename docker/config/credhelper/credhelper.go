package credhelper

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
)

type storedCredentials struct {
	Username string
	Secret   string
}

// GetCredentials gets Docker registry credentials either from "credsStore" or "credHelpers"
func GetCredentials(registry, credsStore string, credHelpers map[string]string) (string, string, error) {
	if credsStore != "" {
		c, err := getCredentials(registry, credsStore)

		if err == nil {
			return c.Username, c.Secret, nil
		}

		os.Stderr.WriteString("[credhelper][credsStore] Error: " + err.Error() + "\n")
	}

	provider, defined := credHelpers[registry]
	if defined {
		c, err := getCredentials(registry, provider)

		if err == nil {
			return c.Username, c.Secret, nil
		}

		os.Stderr.WriteString("[credhelper][credHelpers] Error: " + err.Error() + "\n")
	}

	return "", "", errors.New("No working credential helpers found for this registry: " + registry)
}

func getCredentials(registry, provider string) (*storedCredentials, error) {
	cmd := exec.Command("docker-credential-"+provider, "get")

	var stdout bytes.Buffer

	cmd.Stdin = bytes.NewBuffer([]byte(registry))
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		return nil, err
	}

	var c storedCredentials
	if err := json.NewDecoder(&stdout).Decode(&c); err != nil {
		return nil, err
	}

	return &c, nil
}
