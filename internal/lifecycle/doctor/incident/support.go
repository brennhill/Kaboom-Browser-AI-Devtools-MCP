// support.go — Builds privacy-bounded local Doctor support artifacts.
package incident

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

type SupportBundle struct {
	SchemaVersion int               `json:"schema_version"`
	Version       string            `json:"version"`
	Platform      string            `json:"platform"`
	Incidents     []SupportIncident `json:"incidents"`
}

type SupportIncident struct {
	Fingerprint string    `json:"fingerprint"`
	Code        Code      `json:"code"`
	Subsystem   Subsystem `json:"subsystem"`
	Stage       Stage     `json:"stage"`
	Severity    Severity  `json:"severity"`
	Retryable   bool      `json:"retryable"`
	State       State     `json:"state"`
	Attempts    uint      `json:"attempts"`
}

func BuildSupportBundle(version, platform string, views []DoctorView) SupportBundle {
	incidents := make([]SupportIncident, 0, len(views))
	for _, view := range views {
		incidents = append(incidents, SupportIncident{
			Fingerprint: view.Fingerprint, Code: view.Code, Subsystem: view.Subsystem,
			Stage: view.Stage, Severity: view.Severity, Retryable: view.Retryable,
			State: view.State, Attempts: view.Attempts,
		})
	}
	return SupportBundle{SchemaVersion: 1, Version: version, Platform: platform, Incidents: incidents}
}

func SupportBundleToken(bundle SupportBundle) (string, error) {
	encoded, err := SupportBundleBytes(bundle)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}

func SupportBundleBytes(bundle SupportBundle) ([]byte, error) {
	encoded, err := json.MarshalIndent(bundle, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(encoded, '\n'), nil
}
