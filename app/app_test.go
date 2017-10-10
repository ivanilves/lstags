package app

import (
	"testing"
)

func TestGeneratePathFromHostname(t *testing.T) {
	examples := map[string]string{
		"localhost":               "/localhost",
		"localhost:5000":          "/localhost",
		"registry.company.com":    "/registry/company/com",
		"dockerz.hipster.io:8443": "/dockerz/hipster/io",
	}

	for input, expected := range examples {
		output := GeneratePathFromHostname(input)

		if output != expected {
			t.Fatalf(
				"Unexpected path '%s' generated from hostname '%s' (expected: '%s')",
				output,
				input,
				expected,
			)
		}
	}
}
