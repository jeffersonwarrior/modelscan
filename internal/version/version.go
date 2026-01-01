package version

import (
	_ "embed"
	"strings"
)

//go:embed version.txt
var versionFile string

// Version returns the current modelscan version
func Version() string {
	return strings.TrimSpace(versionFile)
}
