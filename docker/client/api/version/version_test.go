package version

import (
	"testing"
)

const dockerSocket = "/var/run/docker.sock"
const invalidSocket = "/var/run/somethinginvalid.sock"

func TestDetect(t *testing.T) {
	_, err := Detect(dockerSocket)
	if err != nil {
		t.Fatalf(
			"Unable to detect Docker API version: %s",
			err.Error(),
		)
	}
}

func TestWithInvalidSocket(t *testing.T) {
	_, err := Detect(invalidSocket)
	if err == nil {
		t.Fatalf(
			"Unable to detect Docker API version: %s",
			err.Error(),
		)
	}
}
