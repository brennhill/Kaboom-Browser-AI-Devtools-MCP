// hookdiag_test.go — Holds the diagnostic line to one record per call.

package hookdiag

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// capture records one record without touching the process's stderr.
func capture(t *testing.T, code string) string {
	t.Helper()
	var buf bytes.Buffer
	EmitTo(&buf, code)
	return buf.String()
}

func TestEmitWritesOneParseableRecord(t *testing.T) {
	t.Parallel()
	line := capture(t, "session_cleanup_remove_failed")

	var record map[string]string
	if err := json.Unmarshal([]byte(strings.TrimSpace(line)), &record); err != nil {
		t.Fatalf("the diagnostic is not JSON, so a reader tailing stderr drops it: %v (%q)", err, line)
	}
	if record["kaboom_hook_diagnostic"] != "session_cleanup_remove_failed" {
		t.Errorf("code = %q, want the one passed in", record["kaboom_hook_diagnostic"])
	}
	if strings.Count(line, "\n") != 1 || !strings.HasSuffix(line, "\n") {
		t.Errorf("one call must write exactly one terminated line, got %q", line)
	}
}

func TestACodeCarryingAQuoteStaysOneRecord(t *testing.T) {
	t.Parallel()
	// A code assembled from an error string can contain a quote or a newline.
	// Interpolated raw, it would end the JSON string early and turn one failure
	// into two records — the second of them unparseable.
	line := capture(t, "read_failed: \"/tmp/a\"\nsecond")

	if strings.Count(line, "\n") != 1 {
		t.Fatalf("the code's newline split the record: %q", line)
	}
	var record map[string]string
	if err := json.Unmarshal([]byte(strings.TrimSpace(line)), &record); err != nil {
		t.Fatalf("the record is unparseable: %v (%q)", err, line)
	}
	if !strings.Contains(record["kaboom_hook_diagnostic"], "second") {
		t.Errorf("the code was truncated at the embedded quote: %q", record["kaboom_hook_diagnostic"])
	}
}
