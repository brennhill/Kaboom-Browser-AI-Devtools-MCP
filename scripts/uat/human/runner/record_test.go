// record_test.go — Proves the run log is resumable and that a skip never counts
// as coverage.

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/scripts/uat/human/inventory"
)

func tempLog(t *testing.T) *runLog {
	t.Helper()
	log, err := openLog(filepath.Join(t.TempDir(), "run.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = log.shutdown() })
	return log
}

func recordFor(id string, chosen verdict) caseRecord {
	return caseRecord{CaseID: id, Verdict: chosen, Note: "note", StartedAt: timestamp(time.Unix(0, 0))}
}

func TestAnsweredCasesSurviveReopeningTheLog(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "run.jsonl")

	first, err := openLog(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := first.appendResult(recordFor("observe/screenshot", verdictPass)); err != nil {
		t.Fatal(err)
	}
	if err := first.shutdown(); err != nil {
		t.Fatal(err)
	}

	// A second sitting must not present a case the first one answered — that is
	// the whole reason a 194-case run can be done over several days.
	second, err := openLog(path)
	if err != nil {
		t.Fatal(err)
	}
	defer second.shutdown()
	if _, answered := second.answered("observe/screenshot"); !answered {
		t.Fatal("the reopened log does not know the case was answered, so the tester would be asked again")
	}
	// Control: a case nobody answered is still open.
	if _, answered := second.answered("observe/page"); answered {
		t.Error("an unanswered case came back answered, which would let a sitting skip cases nobody looked at")
	}
}

func TestASkipIsNotAnAnswer(t *testing.T) {
	t.Parallel()
	log := tempLog(t)
	if err := log.appendResult(recordFor("interact/click", verdictSkipped)); err != nil {
		t.Fatal(err)
	}

	// A skipped case must come back around. If SKIPPED counted as answered, a
	// tester could skip all 194 cases and the coverage gate would read 100%.
	if _, answered := log.answered("interact/click"); answered {
		t.Fatal("SKIPPED counted as an humanAnswer")
	}
	for _, verdict := range []verdict{verdictPass, verdictFail, verdictBlocked} {
		if !verdict.answered() {
			t.Errorf("%s must settle a case", verdict)
		}
	}
}

func TestALaterAnswerReplacesAnEarlierOne(t *testing.T) {
	t.Parallel()
	log := tempLog(t)
	if err := log.appendResult(recordFor("observe/logs", verdictFail)); err != nil {
		t.Fatal(err)
	}
	if err := log.appendResult(recordFor("observe/logs", verdictPass)); err != nil {
		t.Fatal(err)
	}

	// The log is append-only, so a re-run of a fixed case leaves both records.
	// The last one is the verdict; reading the first would report a fix as broken
	// forever.
	caseRecord, answered := log.answered("observe/logs")
	if !answered || caseRecord.Verdict != verdictPass {
		t.Fatalf("verdict = %q, want the later PASS", caseRecord.Verdict)
	}
	if len(failedCases(log)) != 0 {
		t.Errorf("the superseded FAIL is still being reported: %v", failedCases(log))
	}
}

func TestACrashTruncatedRecordDoesNotLoseTheRunBeforeIt(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "run.jsonl")
	good, err := json.Marshal(recordFor("observe/page", verdictPass))
	if err != nil {
		t.Fatal(err)
	}
	// The second line is what a process killed mid-write leaves behind.
	if err := os.WriteFile(path, append(append(good, '\n'), []byte(`{"case_id":"observe/net`)...), 0o644); err != nil {
		t.Fatal(err)
	}

	log, err := openLog(path)
	if err != nil {
		t.Fatalf("a half-written last line made the whole log unreadable: %v", err)
	}
	defer log.shutdown()
	if _, answered := log.answered("observe/page"); !answered {
		t.Error("the complete record before the truncated one was lost")
	}
}

func TestUnansweredCasesAreCountedNotOmitted(t *testing.T) {
	t.Parallel()
	log := tempLog(t)
	if err := log.appendResult(recordFor("a/one", verdictPass)); err != nil {
		t.Fatal(err)
	}
	if err := log.appendResult(recordFor("a/two", verdictFail)); err != nil {
		t.Fatal(err)
	}
	cases := []inventory.Case{{ID: "a/one"}, {ID: "a/two"}, {ID: "a/three"}, {ID: "a/four"}}

	tally := summarize(cases, log)
	if tally.Pass != 1 || tally.Fail != 1 {
		t.Errorf("tally = %+v, want one pass and one fail", tally)
	}
	// Two cases nobody looked at. Counting only what is in the log would report
	// this run as 50% pass instead of 25%.
	if tally.Unanswered != 2 {
		t.Errorf("unanswered = %d, want 2 — a partial run must not read as complete", tally.Unanswered)
	}
	if got := describeTally(tally, len(cases)); got == "" {
		t.Error("the tally renders empty")
	}
}
