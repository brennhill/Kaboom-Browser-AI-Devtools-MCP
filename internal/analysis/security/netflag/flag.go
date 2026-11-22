// flag.go — Defines canonical suspicious-network findings.
// Docs: docs/features/feature/security-hardening/index.md

package netflag

import "time"

// Flag describes one suspicious property of a captured network resource.
type Flag struct {
	Type      string    `json:"type"`
	Severity  string    `json:"severity"`
	Origin    string    `json:"origin"`
	Message   string    `json:"message"`
	Resource  string    `json:"resource"`
	PageURL   string    `json:"page_url"`
	Timestamp time.Time `json:"timestamp"`
}
