// bridge_fingerprint.go -- Computes an immutable binary fingerprint (SHA-256, Go build ID, path) for bridge launch diagnostics.
// Why: Enables version-mismatch detection between bridge and daemon when troubleshooting upgrade issues.
// Docs: docs/features/feature/bridge-restart/index.md

package bridge

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
)

const goBuildIDPrefix = "\xff Go build ID: \""

var getBridgeExecutablePath = os.Executable

// LaunchFingerprint returns immutable diagnostics that identify the exact
// binary image used for this bridge process.
func (r *Runner) LaunchFingerprint() map[string]any {
	fingerprint := map[string]any{
		"binary_version":  r.identity.Version,
		"binary_path":     "",
		"binary_build_id": "unknown",
		"binary_sha256":   "unknown",
	}

	exePath, err := getBridgeExecutablePath()
	if err != nil {
		fingerprint["binary_path_error"] = err.Error()
		return fingerprint
	}
	fingerprint["binary_path"] = exePath

	if buildID, buildErr := readGoBuildID(exePath); buildErr == nil {
		fingerprint["binary_build_id"] = buildID
	} else {
		fingerprint["binary_build_id_error"] = buildErr.Error()
	}

	if sha, shaErr := fileSHA256(exePath); shaErr == nil {
		fingerprint["binary_sha256"] = sha
	} else {
		fingerprint["binary_sha256_error"] = shaErr.Error()
	}

	return fingerprint
}

func readGoBuildID(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	buildID := extractGoBuildID(data)
	if buildID == "" {
		return "", errors.New("go build id not found")
	}
	return buildID, nil
}

func extractGoBuildID(data []byte) string {
	idx := bytes.Index(data, []byte(goBuildIDPrefix))
	if idx < 0 {
		return ""
	}
	start := idx + len(goBuildIDPrefix)
	end := bytes.IndexByte(data[start:], '"')
	if end < 0 {
		return ""
	}
	return string(data[start : start+end])
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path) // #nosec G304 -- path comes from os.Executable
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
