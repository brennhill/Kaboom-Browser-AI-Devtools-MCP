// probe.go — Parses the canonical daemon health identity and version contract.
// Docs: docs/features/feature/lazy-server-start/index.md

package healthprobe

import (
	"encoding/json"
	"strings"
)

type metadata struct {
	Version string `json:"version"`
	Name    string `json:"name"`
}

// Evaluate parses a health payload and checks its canonical identity and version.
func Evaluate(body []byte, expectedName, expectedVersion string) (compatible bool, version, serviceName string, valid bool) {
	var value metadata
	if json.Unmarshal(body, &value) != nil {
		return false, "", "", false
	}
	version = strings.TrimSpace(value.Version)
	serviceName = strings.TrimSpace(value.Name)
	normalize := func(version string) string {
		return strings.TrimPrefix(strings.TrimSpace(version), "v")
	}
	compatible = serviceName == expectedName && version != "" && normalize(version) == normalize(expectedVersion)
	return compatible, version, serviceName, true
}
