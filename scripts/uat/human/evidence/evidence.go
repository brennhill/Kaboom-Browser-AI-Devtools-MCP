// Purpose: The evidence bundle written beside every human UAT verdict.
// Why: A FAIL has to be actionable by someone who was not in the room.
// Docs: docs/features/feature/human-uat-rig/index.md

package evidence

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Probe is one thing captured beside a case.
type Probe struct {
	Name string
	Tool string
	Args map[string]any
}

// Probes are captured before and after the call under test.
//
// Console and network come from the same daemon the call went to, so they
// describe the same browser the tester is looking at. The screenshot is what
// makes "it looked wrong" reviewable a week later.
func Probes() []Probe {
	return []Probe{
		{Name: "screenshot", Tool: "observe", Args: map[string]any{"what": "screenshot"}},
		{Name: "console", Tool: "observe", Args: map[string]any{"what": "logs"}},
		{Name: "network", Tool: "observe", Args: map[string]any{"what": "network_waterfall"}},
	}
}

// File is one artifact in a bundle.
//
// The digest is what makes two runs comparable: a reader diffing yesterday's
// manifest against today's sees which artifact changed without opening any of
// them, and an artifact that was replaced between runs cannot look unchanged.
type File struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Bytes  int    `json:"bytes"`
	// Error is set when the probe could not run. The file then holds the reason
	// rather than the capture, and this says so — an empty bundle is ambiguous
	// between "capture failed" and "capture was off", and those lead to opposite
	// conclusions about a FAIL.
	Error string `json:"error,omitempty"`
}

// Manifest is the bundle's index: everything needed to reopen the case.
type Manifest struct {
	CaseID     string          `json:"case_id"`
	Question   string          `json:"question"`
	BuildSHA   string          `json:"build_sha"`
	FixtureSHA string          `json:"fixture_sha"`
	RunID      string          `json:"run_id"`
	Request    json.RawMessage `json:"request,omitempty"`
	Response   json.RawMessage `json:"response,omitempty"`
	CallError  string          `json:"call_error,omitempty"`
	Files      []File          `json:"files"`
	CapturedAt string          `json:"captured_at"`
}

// ManifestName is the file every bundle carries.
const ManifestName = "bundle.json"

// Bundle collects one case's artifacts under a directory.
type Bundle struct {
	dir   string
	files []File
}

// Open creates the bundle directory.
func Open(root, caseID string) (*Bundle, error) {
	dir := filepath.Join(root, SafeName(caseID))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create evidence directory %s: %w", dir, err)
	}
	return &Bundle{dir: dir}, nil
}

// Dir is where the bundle is being written.
func (b *Bundle) Dir() string { return b.dir }

// Write stores one artifact and records its digest.
func (b *Bundle) Write(name string, content []byte) File {
	path := filepath.Join(b.dir, name)
	record := File{Path: path, Bytes: len(content), SHA256: digest(content)}
	if err := os.WriteFile(path, content, 0o644); err != nil {
		record.Error = err.Error()
	}
	b.files = append(b.files, record)
	return record
}

// WriteFailure stores why a probe could not run.
func (b *Bundle) WriteFailure(name, reason string) File {
	path := filepath.Join(b.dir, name+".error")
	record := File{Path: path, Bytes: len(reason), SHA256: digest([]byte(reason)), Error: reason}
	if err := os.WriteFile(path, []byte(reason), 0o644); err != nil {
		record.Error = reason + "; and the reason could not be written: " + err.Error()
	}
	b.files = append(b.files, record)
	return record
}

// WriteScreenshot decodes a screenshot response into a viewable image when it
// carries one, and stores the response either way.
//
// The JSON alone is not evidence a person can act on: `data_url` is a megabyte
// of base64, and nobody opening a FAIL a week later is going to paste it into a
// decoder. When there is no image in the response — the extension was not
// connected, or the capture was queued — the JSON is all there is, and the
// bundle says so rather than writing a zero-byte png.
func (b *Bundle) WriteScreenshot(name string, response []byte) []File {
	written := []File{b.Write(name+".json", response)}
	image, ok := decodeScreenshot(response)
	if !ok {
		return written
	}
	return append(written, b.Write(name+".png", image))
}

// Paths lists every artifact written, for the run log.
func (b *Bundle) Paths() []string {
	paths := make([]string, 0, len(b.files))
	for _, file := range b.files {
		paths = append(paths, file.Path)
	}
	return paths
}

// Files is what the manifest records.
func (b *Bundle) Files() []File { return b.files }

// WriteManifest closes the bundle with its index.
func (b *Bundle) WriteManifest(manifest Manifest) error {
	manifest.Files = b.files
	encoded, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(b.dir, ManifestName), append(encoded, '\n'), 0o644)
}

// decodeScreenshot pulls the PNG or JPEG out of a screenshot response.
func decodeScreenshot(response []byte) ([]byte, bool) {
	var body struct {
		DataURL string `json:"data_url"`
	}
	if json.Unmarshal(response, &body) != nil || body.DataURL == "" {
		// The response may be the MCP envelope with the payload nested in text.
		if inner, ok := nestedText(response); ok {
			return decodeScreenshot(inner)
		}
		return nil, false
	}
	comma := strings.Index(body.DataURL, ",")
	if comma == -1 || !strings.Contains(body.DataURL[:comma], ";base64") {
		return nil, false
	}
	decoded, err := base64.StdEncoding.DecodeString(body.DataURL[comma+1:])
	if err != nil || len(decoded) == 0 {
		return nil, false
	}
	return decoded, true
}

// nestedText unwraps one level of the MCP content envelope.
func nestedText(response []byte) ([]byte, bool) {
	var envelope struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	if json.Unmarshal(response, &envelope) != nil || len(envelope.Content) == 0 {
		return nil, false
	}
	for _, part := range envelope.Content {
		if trimmed := strings.TrimSpace(part.Text); strings.HasPrefix(trimmed, "{") {
			return []byte(trimmed), true
		}
	}
	return nil, false
}

// SafeName makes a case id usable as a directory name.
func SafeName(id string) string {
	return strings.ReplaceAll(id, "/", "__")
}

func digest(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}
