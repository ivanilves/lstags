package util

import (
	"testing"
)

func TestDoesMatch(t *testing.T) {
	examples := []struct {
		s       string
		pattern string
		matched bool
	}{
		{"latest", "^latest$", true},
		{"v1.0.1", "^v1\\.0\\.1$", true},
		{"barbos", ".*", true},
		{"3.4", "*", false},
	}

	for _, e := range examples {
		matched := DoesMatch(e.s, e.pattern)

		action := "should"
		if !e.matched {
			action = "should not"
		}

		if matched != e.matched {
			t.Errorf(
				"String '%s' %s match pattern '%s'",
				e.s,
				action,
				e.pattern,
			)
		}
	}
}

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
			t.Errorf(
				"Unexpected path '%s' generated from hostname '%s' (expected: '%s')",
				output,
				input,
				expected,
			)
		}
	}
}
