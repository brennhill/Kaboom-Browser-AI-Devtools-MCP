// evidence_test.go — Proves a bundle is actionable by someone who was not in the
// room, and that two runs of the same case can be told apart.

package evidence

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// pngBytes is a one-pixel PNG, enough to prove the decode path wrote an image.
func pngBytes() []byte {
	decoded, _ := base64.StdEncoding.DecodeString(
		"iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg==")
	return decoded
}

func openBundle(t *testing.T) *Bundle {
	t.Helper()
	bundle, err := Open(t.TempDir(), "observe/screenshot")
	if err != nil {
		t.Fatal(err)
	}
	return bundle
}

func TestAScreenshotIsWrittenAsAnImageSomebodyCanOpen(t *testing.T) {
	t.Parallel()
	bundle := openBundle(t)
	response, err := json.Marshal(map[string]string{
		"data_url": "data:image/png;base64," + base64.StdEncoding.EncodeToString(pngBytes()),
	})
	if err != nil {
		t.Fatal(err)
	}

	files := bundle.WriteScreenshot("before", response)
	if len(files) != 2 {
		t.Fatalf("files = %d, want the json and the decoded image", len(files))
	}
	image, err := os.ReadFile(filepath.Join(bundle.Dir(), "before.png"))
	if err != nil {
		t.Fatalf("no image was written; a FAIL a week later is a megabyte of base64 nobody will decode: %v", err)
	}
	if string(image[:4]) != "\x89PNG" {
		t.Errorf("the written image is not a PNG: %q", image[:4])
	}
}

func TestAScreenshotNestedInTheMCPEnvelopeIsStillDecoded(t *testing.T) {
	t.Parallel()
	bundle := openBundle(t)
	// This is the shape the server actually returns: the payload is JSON inside
	// the content envelope's text field.
	inner, err := json.Marshal(map[string]string{
		"data_url": "data:image/png;base64," + base64.StdEncoding.EncodeToString(pngBytes()),
	})
	if err != nil {
		t.Fatal(err)
	}
	envelope, err := json.Marshal(map[string]any{
		"content": []map[string]string{{"type": "text", "text": string(inner)}},
	})
	if err != nil {
		t.Fatal(err)
	}

	if files := bundle.WriteScreenshot("after", envelope); len(files) != 2 {
		t.Fatalf("files = %d; the envelope was not unwrapped, so no image reaches the bundle", len(files))
	}
}

func TestAResponseWithNoImageWritesNoEmptyPNG(t *testing.T) {
	t.Parallel()
	bundle := openBundle(t)
	// The extension was not connected, so the capture was queued and there is no
	// image. A zero-byte png would look like a screenshot of a blank page.
	files := bundle.WriteScreenshot("before", []byte(`{"correlation_id":"abc","status":"queued"}`))
	if len(files) != 1 {
		t.Fatalf("files = %d, want only the json", len(files))
	}
	if _, err := os.Stat(filepath.Join(bundle.Dir(), "before.png")); err == nil {
		t.Error("an image file was created for a response carrying no image")
	}
}

func TestAFailedProbeLeavesItsReasonBehind(t *testing.T) {
	t.Parallel()
	bundle := openBundle(t)
	file := bundle.WriteFailure("network", "extension not connected")

	content, err := os.ReadFile(file.Path)
	if err != nil {
		t.Fatalf("nothing was written for a failed probe: %v", err)
	}
	if !strings.Contains(string(content), "extension not connected") {
		t.Errorf("the file does not say why: %q", content)
	}
	if file.Error == "" {
		t.Error("the manifest entry does not record that this probe failed, so an empty capture reads as a successful one")
	}
}

func TestTwoRunsOfTheSameCaseAreTellableApart(t *testing.T) {
	t.Parallel()
	first := openBundle(t)
	second := openBundle(t)

	sameAsBefore := first.Write("console.json", []byte(`{"logs":[]}`))
	changed := second.Write("console.json", []byte(`{"logs":[{"level":"error"}]}`))
	if sameAsBefore.SHA256 == changed.SHA256 {
		t.Fatal("two different captures hashed the same, so a run diff would show nothing changed")
	}

	// Control: identical content must hash identically, or every artifact would
	// look changed on every run and the diff would be useless.
	repeat := second.Write("console-again.json", []byte(`{"logs":[]}`))
	if repeat.SHA256 != sameAsBefore.SHA256 {
		t.Error("identical captures hashed differently")
	}
}

func TestTheManifestCarriesEverythingNeededToReopenTheCase(t *testing.T) {
	t.Parallel()
	bundle := openBundle(t)
	bundle.Write("console.json", []byte(`{"logs":[]}`))
	err := bundle.WriteManifest(Manifest{
		CaseID:     "observe/screenshot",
		Question:   "Does the image show what is on screen right now?",
		BuildSHA:   "abc1234",
		FixtureSHA: "def5678",
		RunID:      "run-1",
		Request:    json.RawMessage(`{"name":"observe","arguments":{"what":"screenshot"}}`),
		Response:   json.RawMessage(`{"ok":true}`),
		CapturedAt: "2026-09-05T00:00:00Z",
	})
	if err != nil {
		t.Fatal(err)
	}

	raw, err := os.ReadFile(filepath.Join(bundle.Dir(), ManifestName))
	if err != nil {
		t.Fatal(err)
	}
	var manifest Manifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatal(err)
	}
	// Each of these is something a reader would otherwise have to ask the person
	// who ran it: what was asked, what was sent, what came back, which build.
	if manifest.Question == "" || manifest.BuildSHA == "" || len(manifest.Request) == 0 || len(manifest.Response) == 0 {
		t.Errorf("the manifest cannot stand alone: %+v", manifest)
	}
	if len(manifest.Files) != 1 || manifest.Files[0].SHA256 == "" {
		t.Errorf("files = %+v, want the artifact and its digest", manifest.Files)
	}
}

func TestACaseIDBecomesOneDirectory(t *testing.T) {
	t.Parallel()
	// A slash would make observe/screenshot two nested directories, and the
	// manifest paths would not match what the run log recorded.
	if name := SafeName("observe/screenshot"); strings.Contains(name, "/") {
		t.Errorf("SafeName(%q) = %q", "observe/screenshot", name)
	}
	if SafeName("popup/pilot_toggle") == SafeName("popup/connection_status") {
		t.Error("two cases collapsed onto one directory, so one would overwrite the other's evidence")
	}
}
