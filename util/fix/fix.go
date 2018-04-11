package fix

import (
	"os"
	"strings"
)

// Path resolves "~" intro the real home path
func Path(path string) string {
	return strings.Replace(path, "~", os.Getenv("HOME"), 1)
}
