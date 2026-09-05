// runlog_test.go — Proves the run log is resumable and that a skip never counts
// as coverage.

package runlog

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/scripts/uat/human/inventory"
)

func tempLog(t *testing.T) *Log {
	t.Helper()
	log, err := OpenLog(filepath.Join(t.TempDir(), "run.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = log.Close() })
	return log
}

func recordFor(id string, chosen Verdict) Result {
	return Result{CaseID: id, Verdict: chosen, Note: "note", StartedAt: Timestamp(time.Unix(0, 0))}
}

func TestAnsweredCasesSurviveReopeningTheLog(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "run.jsonl")

	first, err := OpenLog(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := first.Append(recordFor("observe/screenshot", VerdictPass)); err != nil {
		t.Fatal(err)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}

	// A second sitting must not present a case the first one Answered — that is
	// the whole reason a 194-case run can be done over several days.
	second, err := OpenLog(path)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	if _, Answered := second.Answered("observe/screenshot"); !Answered {
		t.Fatal("the reopened log does not know the case was Answered, so the tester would be asked again")
	}
	// Control: a case nobody Answered is still open.
	if _, Answered := second.Answered("observe/page"); Answered {
		t.Error("an unanswered case came back Answered, which would let a sitting skip cases nobody looked at")
	}
}

func TestASkipIsNotAnAnswer(t *testing.T) {
	t.Parallel()
	log := tempLog(t)
	if err := log.Append(recordFor("interact/click", VerdictSkipped)); err != nil {
		t.Fatal(err)
	}

	// A skipped case must come back around. If SKIPPED counted as Answered, a
	// tester could skip all 194 cases and the coverage gate would read 100%.
	if _, Answered := log.Answered("interact/click"); Answered {
		t.Fatal("SKIPPED counted as an humanAnswer")
	}
	for _, Verdict := range []Verdict{VerdictPass, VerdictFail, VerdictBlocked} {
		if !Verdict.Answered() {
			t.Errorf("%s must settle a case", Verdict)
		}
	}
}

func TestALaterAnswerReplacesAnEarlierOne(t *testing.T) {
	t.Parallel()
	log := tempLog(t)
	if err := log.Append(recordFor("observe/logs", VerdictFail)); err != nil {
		t.Fatal(err)
	}
	if err := log.Append(recordFor("observe/logs", VerdictPass)); err != nil {
		t.Fatal(err)
	}

	// The log is append-only, so a re-run of a fixed case leaves both records.
	// The last one is the Verdict; reading the first would report a fix as broken
	// forever.
	Result, Answered := log.Answered("observe/logs")
	if !Answered || Result.Verdict != VerdictPass {
		t.Fatalf("Verdict = %q, want the later PASS", Result.Verdict)
	}
	if len(FailedCases(log)) != 0 {
		t.Errorf("the superseded FAIL is still being reported: %v", FailedCases(log))
	}
}

func TestACrashTruncatedRecordDoesNotLoseTheRunBeforeIt(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "run.jsonl")
	good, err := json.Marshal(recordFor("observe/page", VerdictPass))
	if err != nil {
		t.Fatal(err)
	}
	// The second line is what a process killed mid-write leaves behind.
	if err := os.WriteFile(path, append(append(good, '\n'), []byte(`{"case_id":"observe/net`)...), 0o644); err != nil {
		t.Fatal(err)
	}

	log, err := OpenLog(path)
	if err != nil {
		t.Fatalf("a half-written last line made the whole log unreadable: %v", err)
	}
	defer log.Close()
	if _, Answered := log.Answered("observe/page"); !Answered {
		t.Error("the complete record before the truncated one was lost")
	}
}

func TestUnansweredCasesAreCountedNotOmitted(t *testing.T) {
	t.Parallel()
	log := tempLog(t)
	if err := log.Append(recordFor("a/one", VerdictPass)); err != nil {
		t.Fatal(err)
	}
	if err := log.Append(recordFor("a/two", VerdictFail)); err != nil {
		t.Fatal(err)
	}
	cases := []inventory.Case{{ID: "a/one"}, {ID: "a/two"}, {ID: "a/three"}, {ID: "a/four"}}

	tally := Summarize(cases, log)
	if tally.Pass != 1 || tally.Fail != 1 {
		t.Errorf("tally = %+v, want one pass and one fail", tally)
	}
	// Two cases nobody looked at. Counting only what is in the log would report
	// this run as 50% pass instead of 25%.
	if tally.Unanswered != 2 {
		t.Errorf("unanswered = %d, want 2 — a partial run must not read as complete", tally.Unanswered)
	}
	if got := DescribeTally(tally, len(cases)); got == "" {
		t.Error("the tally renders empty")
	}
}
