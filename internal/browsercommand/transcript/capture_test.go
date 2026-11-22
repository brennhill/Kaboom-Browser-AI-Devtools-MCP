// capture_test.go — Pins pairing of issued commands with their outcomes.

package synctranscript

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestFileRecorderPairsACommandWithItsOutcome(t *testing.T) {
	recorder := NewFileRecorder(filepath.Join(t.TempDir(), "t.jsonl"))
	recorder.Issued("cmd-1", "analyze", json.RawMessage(`{"what":"dom"}`))
	recorder.Completed("cmd-1", "complete", json.RawMessage(`{"elements":2}`), "")

	if recorder.Written() != 1 {
		t.Fatalf("Written() = %d, want 1", recorder.Written())
	}
	records := readBack(t, recorder)
	if records[0].Type != "analyze" || string(records[0].Result) != `{"elements":2}` {
		t.Errorf("record = %+v", records[0])
	}
	if records[0].Fingerprint == "" {
		t.Error("record has no fingerprint, so replay could never match it")
	}
}

// readBack closes the recorder and decodes what it actually wrote, so the
// assertions run against the file a replay would load rather than in-memory
// state the file might not reflect.
func readBack(t *testing.T, recorder *FileRecorder) []Record {
	t.Helper()
	if err := recorder.Close(); err != nil {
		t.Fatalf("Close() = %v", err)
	}
	file, err := os.Open(recorder.path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = file.Close() }()
	records, err := Decode(file)
	if err != nil {
		t.Fatal(err)
	}
	return records
}

// Commands and their results arrive in different requests, so the recorder has
// to hold the command open across an arbitrary gap.
func TestFileRecorderPairsAcrossInterleavedCommands(t *testing.T) {
	recorder := NewFileRecorder(filepath.Join(t.TempDir(), "t.jsonl"))
	recorder.Issued("a", "analyze", json.RawMessage(`{"what":"dom"}`))
	recorder.Issued("b", "observe", json.RawMessage(`{"what":"logs"}`))
	recorder.Completed("b", "complete", json.RawMessage(`{"logs":[]}`), "")
	recorder.Completed("a", "complete", json.RawMessage(`{"elements":1}`), "")

	records := readBack(t, recorder)
	if len(records) != 2 {
		t.Fatalf("records = %+v, want two", records)
	}
	if records[0].Type != "observe" || records[1].Type != "analyze" {
		t.Errorf("records = %+v, want completion order", records)
	}
}

// A result for a command issued before recording began has no command to pair
// with; storing it would produce an entry replay can never match.
func TestFileRecorderIgnoresAnUnpairedResult(t *testing.T) {
	recorder := NewFileRecorder(filepath.Join(t.TempDir(), "t.jsonl"))
	recorder.Completed("unknown", "complete", json.RawMessage(`{}`), "")
	if recorder.Written() != 0 {
		t.Errorf("Written() = %d, want 0", recorder.Written())
	}
}

func TestFileRecorderIgnoresCommandsWithNoID(t *testing.T) {
	recorder := NewFileRecorder(filepath.Join(t.TempDir(), "t.jsonl"))
	recorder.Issued("", "analyze", json.RawMessage(`{}`))
	recorder.Completed("", "complete", json.RawMessage(`{}`), "")
	if recorder.Written() != 0 {
		t.Errorf("Written() = %d, want 0", recorder.Written())
	}
}

// A command that never got an answer must not be written as an answer of
// nothing — that is the empty-success failure mode, preserved into the fixture.
func TestFileRecorderReportsUnansweredCommandsInsteadOfWritingThem(t *testing.T) {
	recorder := NewFileRecorder(filepath.Join(t.TempDir(), "t.jsonl"))
	recorder.Issued("hung", "analyze", json.RawMessage(`{"what":"dom"}`))

	if recorder.Written() != 0 {
		t.Errorf("an unanswered command was recorded (%d written)", recorder.Written())
	}
	unanswered := recorder.Unanswered()
	if len(unanswered) != 1 || unanswered[0] != "hung" {
		t.Errorf("Unanswered() = %v, want [hung]", unanswered)
	}
}

func TestFileRecorderPreservesAFailure(t *testing.T) {
	recorder := NewFileRecorder(filepath.Join(t.TempDir(), "t.jsonl"))
	recorder.Issued("cmd-1", "analyze", json.RawMessage(`{"what":"computed_styles"}`))
	recorder.Completed("cmd-1", "error", nil, "no active tab")

	records := readBack(t, recorder)
	if len(records) != 1 || records[0].Status != "error" || records[0].Error != "no active tab" {
		t.Errorf("records = %+v, want the failure preserved", records)
	}
}

func TestRecordingIsReplayableFromDisk(t *testing.T) {
	path := filepath.Join(t.TempDir(), "t.jsonl")
	recorder := NewFileRecorder(path)
	recorder.Issued("cmd-1", "analyze", json.RawMessage(`{"what":"dom","selector":"body"}`))
	recorder.Completed("cmd-1", "complete", json.RawMessage(`{"elements":2}`), "")

	transcript := NewTranscript(readBack(t, recorder))
	if _, matched := transcript.Match(Command{Type: "analyze", Params: json.RawMessage(`{"what":"dom","selector":"body","tab_id":9}`)}); !matched {
		t.Error("the flushed transcript did not replay the command it recorded")
	}
}

// An empty transcript would replay as a fake extension that answers nothing,
// which is indistinguishable from a browser that never responded. Creating no
// file at all keeps that ambiguity out of CI: the replay binary refuses to
// start without a transcript.
func TestNoFileIsCreatedWhenNothingCompleted(t *testing.T) {
	path := filepath.Join(t.TempDir(), "t.jsonl")
	recorder := NewFileRecorder(path)
	recorder.Issued("hung", "analyze", json.RawMessage(`{}`))

	if err := recorder.Close(); err != nil {
		t.Fatalf("Close() = %v", err)
	}
	if _, statErr := os.Stat(path); statErr == nil {
		t.Error("a file was created for a run with no completed exchange")
	}
}

// A write failure must be visible. A recorder that silently wrote nothing
// produces a transcript that replays as a browser answering nothing.
func TestWriteFailureIsRetainedAndReported(t *testing.T) {
	unwritable := filepath.Join(t.TempDir(), "no-such-dir", "t.jsonl")
	recorder := NewFileRecorder(unwritable)
	recorder.Issued("cmd-1", "analyze", json.RawMessage(`{}`))
	recorder.Completed("cmd-1", "complete", json.RawMessage(`{}`), "")

	if recorder.Err() == nil {
		t.Error("a failed transcript write was not reported")
	}
	if recorder.Written() != 0 {
		t.Errorf("Written() = %d, want 0 after a failed write", recorder.Written())
	}
}
