package getenv

import (
	"os"
)

// String gets string value of a given environment variable, or falls back to default
func String(name, defaultValue string) string {
	value := os.Getenv(name)

	if value != "" {
		return value
	}

	return defaultValue
}
