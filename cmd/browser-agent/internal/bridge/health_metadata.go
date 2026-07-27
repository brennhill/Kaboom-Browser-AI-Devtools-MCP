// health_metadata.go — Parses daemon health identity and compares normalized versions.

package bridge

import (
	"encoding/json"
	"strings"
)

type healthMetadata struct {
	Version     string `json:"version"`
	Service     string `json:"service"`
	ServiceName string `json:"service-name"`
	Name        string `json:"name"`
}

func decodeHealthMetadata(body []byte) (healthMetadata, bool) {
	var metadata healthMetadata
	if json.Unmarshal(body, &metadata) != nil {
		return healthMetadata{}, false
	}
	return metadata, true
}

func (m healthMetadata) resolvedServiceName() string {
	for _, name := range []string{m.ServiceName, m.Service, m.Name} {
		if trimmed := strings.TrimSpace(name); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func versionsMatch(left, right string) bool {
	normalize := func(version string) string {
		return strings.TrimPrefix(strings.TrimSpace(version), "v")
	}
	return normalize(left) == normalize(right)
}
