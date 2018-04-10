package fix

import (
	"os/user"
	"strings"
)

// Path resolves "~" intro the real home path
func Path(path string) string {
	u, _ := user.Current()

	return strings.Replace(path, "~", u.HomeDir, 1)
}
