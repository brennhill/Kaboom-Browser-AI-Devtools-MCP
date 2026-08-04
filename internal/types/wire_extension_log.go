// wire_extension_log.go — Defines the canonical extension diagnostic wire entry.

package types

import (
	"encoding/json"
	"time"
)

// ExtensionLog is one redacted local extension diagnostic sent over /sync.
type ExtensionLog struct {
	Timestamp time.Time       `json:"timestamp"`
	Level     string          `json:"level"`
	Message   string          `json:"message"`
	Source    string          `json:"source"`
	Category  string          `json:"category,omitempty"`
	Data      json.RawMessage `json:"data,omitempty"`
}
