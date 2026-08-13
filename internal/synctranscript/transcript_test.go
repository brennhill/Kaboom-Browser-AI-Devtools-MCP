// transcript_test.go — Pins recording and matching of extension command exchanges.

package synctranscript

import (
	"encoding/json"
	"strings"
	"testing"
)

func cmd(kind, params string) Command {
	return Command{Type: kind, Params: json.RawMessage(params)}
}

// A fingerprint must survive the fields that differ on every run, or a replay
// never matches anything it recorded and the suite silently degrades into
// "no recorded answer" for every command.
func TestFingerprintIgnoresVolatileFields(t *testing.T) {
	first := Fingerprint("analyze", json.RawMessage(`{"what":"computed_styles","selector":".card","tab_id":41}`))
	second := Fingerprint("analyze", json.RawMessage(`{"what":"computed_styles","selector":".card","tab_id":99}`))
	if first != second {
		t.Errorf("tab_id changed the fingerprint: %s vs %s", first, second)
	}
}

func TestFingerprintIgnoresEveryVolatileKey(t *testing.T) {
	base := `{"what":"dom","selector":"body"}`
	noisy := `{"what":"dom","selector":"body","tab_id":7,"correlation_id":"c-1","trace_id":"t-1","request_id":"r","timestamp":"2026-01-01T00:00:00Z","connection_generation":4}`
	if Fingerprint("analyze", json.RawMessage(base)) != Fingerprint("analyze", json.RawMessage(noisy)) {
		t.Error("a volatile key leaked into the fingerprint")
	}
}

// Key order is not meaningful in JSON, and the extension does not promise one.
func TestFingerprintIsIndependentOfKeyOrder(t *testing.T) {
	a := Fingerprint("analyze", json.RawMessage(`{"what":"dom","selector":"body"}`))
	b := Fingerprint("analyze", json.RawMessage(`{"selector":"body","what":"dom"}`))
	if a != b {
		t.Errorf("key order changed the fingerprint: %s vs %s", a, b)
	}
}

// The whole point is discrimination: two genuinely different commands must not
// collide, or a replay answers one query with another's result.
func TestFingerprintDistinguishesMeaningfulDifferences(t *testing.T) {
	distinct := []struct {
		kind   string
		params string
	}{
		{"analyze", `{"what":"dom","selector":"body"}`},
		{"analyze", `{"what":"dom","selector":".card"}`},
		{"analyze", `{"what":"computed_styles","selector":"body"}`},
		{"interact", `{"what":"dom","selector":"body"}`},
	}
	seen := make(map[string]string)
	for _, entry := range distinct {
		print := Fingerprint(entry.kind, json.RawMessage(entry.params))
		if previous, collided := seen[print]; collided {
			t.Errorf("%s %s collided with %s", entry.kind, entry.params, previous)
		}
		seen[print] = entry.kind + " " + entry.params
	}
}

func TestFingerprintHandlesNestedVolatileFields(t *testing.T) {
	a := Fingerprint("interact", json.RawMessage(`{"steps":[{"action":"click","tab_id":1}]}`))
	b := Fingerprint("interact", json.RawMessage(`{"steps":[{"action":"click","tab_id":2}]}`))
	if a != b {
		t.Error("a volatile key nested in an array element leaked into the fingerprint")
	}
}

func TestFingerprintToleratesUnparseableParams(t *testing.T) {
	if Fingerprint("analyze", json.RawMessage(`not json`)) == "" {
		t.Error("unparseable params produced no fingerprint")
	}
}

func TestRecordAndMatchRoundTrip(t *testing.T) {
	recorder := NewRecorder()
	recorder.Observe(cmd("analyze", `{"what":"dom","selector":"body"}`), Result{
		Status: "complete",
		Result: json.RawMessage(`{"elements":3}`),
	})

	transcript := NewTranscript(recorder.Records())
	got, matched := transcript.Match(cmd("analyze", `{"what":"dom","selector":"body","tab_id":88}`))
	if !matched {
		t.Fatal("a recorded command did not match on replay")
	}
	if string(got.Result) != `{"elements":3}` {
		t.Errorf("Result = %s", got.Result)
	}
}

// Without this, an unrecorded command silently returns a zero result and the
// category under replay reports a clean pass for work that never happened.
func TestMatchReportsAnUnrecordedCommand(t *testing.T) {
	transcript := NewTranscript(nil)
	if _, matched := transcript.Match(cmd("analyze", `{"what":"dom"}`)); matched {
		t.Error("an unrecorded command matched")
	}
}

// Repeated identical commands are normal — a category may probe the same
// selector before and after an action — and each must get its own recorded
// answer in order.
func TestMatchReplaysRepeatedCommandsInRecordedOrder(t *testing.T) {
	recorder := NewRecorder()
	same := cmd("analyze", `{"what":"dom","selector":".card"}`)
	recorder.Observe(same, Result{Status: "complete", Result: json.RawMessage(`{"n":1}`)})
	recorder.Observe(same, Result{Status: "complete", Result: json.RawMessage(`{"n":2}`)})

	transcript := NewTranscript(recorder.Records())
	first, _ := transcript.Match(same)
	second, _ := transcript.Match(same)
	if string(first.Result) != `{"n":1}` || string(second.Result) != `{"n":2}` {
		t.Errorf("replay order = %s then %s, want n:1 then n:2", first.Result, second.Result)
	}
}

// A category that runs one more probe than was recorded must not silently
// receive the last answer again — that turns a missing recording into a pass.
func TestMatchDoesNotReuseTheLastAnswerOnceExhausted(t *testing.T) {
	recorder := NewRecorder()
	same := cmd("analyze", `{"what":"dom"}`)
	recorder.Observe(same, Result{Status: "complete", Result: json.RawMessage(`{"n":1}`)})

	transcript := NewTranscript(recorder.Records())
	transcript.Match(same)
	if _, matched := transcript.Match(same); matched {
		t.Error("an exhausted recording answered a second time")
	}
}

func TestMatchPreservesAnErrorOutcome(t *testing.T) {
	recorder := NewRecorder()
	failing := cmd("analyze", `{"what":"computed_styles"}`)
	recorder.Observe(failing, Result{Status: "error", Error: "no active tab"})

	transcript := NewTranscript(recorder.Records())
	got, matched := transcript.Match(failing)
	if !matched {
		t.Fatal("a recorded failure did not match")
	}
	if got.Status != "error" || got.Error != "no active tab" {
		t.Errorf("got = %+v, want the recorded failure preserved", got)
	}
}

func TestEncodeDecodeRoundTrip(t *testing.T) {
	recorder := NewRecorder()
	recorder.Observe(cmd("analyze", `{"what":"dom"}`), Result{Status: "complete", Result: json.RawMessage(`{"ok":true}`)})
	recorder.Observe(cmd("interact", `{"action":"click"}`), Result{Status: "error", Error: "detached"})

	var buffer strings.Builder
	if err := Encode(&buffer, recorder.Records()); err != nil {
		t.Fatal(err)
	}
	decoded, err := Decode(strings.NewReader(buffer.String()))
	if err != nil {
		t.Fatal(err)
	}
	if len(decoded) != 2 {
		t.Fatalf("decoded %d record(s), want 2", len(decoded))
	}
	if decoded[0].Type != "analyze" || decoded[1].Error != "detached" {
		t.Errorf("decoded = %+v", decoded)
	}
}

// A transcript is a test input. A malformed one must stop the run rather than
// replay as an empty transcript, which would answer nothing and look like a
// browser that never responded.
func TestDecodeRejectsAMalformedTranscript(t *testing.T) {
	if _, err := Decode(strings.NewReader("{not json}\n")); err == nil {
		t.Error("a malformed transcript decoded without error")
	}
}

func TestDecodeSkipsBlankLines(t *testing.T) {
	body := "{\"type\":\"analyze\",\"status\":\"complete\"}\n\n   \n{\"type\":\"observe\",\"status\":\"complete\"}\n"
	records, err := Decode(strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 2 {
		t.Fatalf("decoded %d record(s), want 2", len(records))
	}
}

func TestDecodeRejectsARecordWithNoType(t *testing.T) {
	if _, err := Decode(strings.NewReader(`{"status":"complete"}` + "\n")); err == nil {
		t.Error("a record with no command type was accepted")
	}
}

// Coverage is the transcript's own health metric: a replay that answered half
// the commands is not a passing run, and the count is what makes that visible.
func TestUnmatchedRecordsWhatWasNeverAsked(t *testing.T) {
	recorder := NewRecorder()
	recorder.Observe(cmd("analyze", `{"what":"dom"}`), Result{Status: "complete"})
	recorder.Observe(cmd("observe", `{"what":"logs"}`), Result{Status: "complete"})

	transcript := NewTranscript(recorder.Records())
	transcript.Match(cmd("analyze", `{"what":"dom"}`))

	unused := transcript.Unused()
	if len(unused) != 1 || unused[0].Type != "observe" {
		t.Errorf("Unused() = %+v, want the observe record", unused)
	}
}

func TestMissesRecordsWhatWasAskedButNeverRecorded(t *testing.T) {
	transcript := NewTranscript(nil)
	transcript.Match(cmd("analyze", `{"what":"dom"}`))
	transcript.Match(cmd("analyze", `{"what":"dom"}`))
	transcript.Match(cmd("observe", `{"what":"logs"}`))

	misses := transcript.Misses()
	if len(misses) != 2 {
		t.Fatalf("Misses() = %+v, want two distinct command shapes", misses)
	}
	if misses["analyze"] != 2 || misses["observe"] != 1 {
		t.Errorf("Misses() = %v, want analyze twice and observe once", misses)
	}
}
