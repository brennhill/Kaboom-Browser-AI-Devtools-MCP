// fingerprint.go — Computes immutable binary identity for bridge launch diagnostics.

package fingerprint

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
)

const goBuildIDPrefix = "\xff Go build ID: \""

// Capture identifies the exact executable image used to start a bridge.
func Capture(version string, executable func() (string, error)) map[string]any {
	result := map[string]any{
		"binary_version": version,
		"binary_path":    "", "binary_build_id": "unknown", "binary_sha256": "unknown",
	}
	path, err := executable()
	if err != nil {
		result["binary_path_error"] = err.Error()
		return result
	}
	result["binary_path"] = path
	if buildID, buildErr := readGoBuildID(path); buildErr == nil {
		result["binary_build_id"] = buildID
	} else {
		result["binary_build_id_error"] = buildErr.Error()
	}
	if sha, shaErr := fileSHA256(path); shaErr == nil {
		result["binary_sha256"] = sha
	} else {
		result["binary_sha256_error"] = shaErr.Error()
	}
	return result
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
	index := bytes.Index(data, []byte(goBuildIDPrefix))
	if index < 0 {
		return ""
	}
	start := index + len(goBuildIDPrefix)
	end := bytes.IndexByte(data[start:], '"')
	if end < 0 {
		return ""
	}
	return string(data[start : start+end])
}

func fileSHA256(path string) (string, error) {
	file, err := os.Open(path) // #nosec G304 -- path comes from os.Executable.
	if err != nil {
		return "", err
	}
	defer func() { _ = file.Close() }()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}
