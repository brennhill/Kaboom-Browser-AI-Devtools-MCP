// judge_test.go — Proves a release cannot inherit somebody else's verdicts.

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/scripts/uat/human/evidence"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/scripts/uat/human/inventory"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/scripts/uat/human/runlog"
)

const thisBuild = "abc1234"

func twoCases() []inventory.Case {
	return []inventory.Case{
		{ID: "observe/page", Kind: inventory.KindMCPMode, Tool: "observe", Mode: "page"},
		{ID: "observe/logs", Kind: inventory.KindMCPMode, Tool: "observe", Mode: "logs"},
	}
}

// runWith writes a log holding the given answers.
func runWith(t *testing.T, records ...runlog.Result) *runlog.Log {
	t.Helper()
	log, err := runlog.OpenLog(filepath.Join(t.TempDir(), "run.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = log.Close() })
	for _, record := range records {
		if err := log.Append(record); err != nil {
			t.Fatal(err)
		}
	}
	return log
}

func answeredOn(id string, verdict runlog.Verdict, build string) runlog.Result {
	return runlog.Result{
		CaseID: id, Verdict: verdict, BuildSHA: build, Note: "seen",
		AnsweredAt: runlog.Timestamp(time.Unix(0, 0)),
	}
}

func TestACompleteRunOnThisBuildPasses(t *testing.T) {
	t.Parallel()
	log := runWith(t,
		answeredOn("observe/page", runlog.VerdictPass, thisBuild),
		answeredOn("observe/logs", runlog.VerdictPass, thisBuild))

	verdict := judge(twoCases(), log, map[string]waiver{}, thisBuild)
	if !verdict.Passed {
		t.Fatalf("a complete clean run was refused:\n%s", verdict.report())
	}
}

func TestAVerdictFromAnotherBuildDoesNotCount(t *testing.T) {
	t.Parallel()
	// This is the failure the gate exists for: "we ran it last week" is how a
	// release ships a regression somebody had already seen and fixed elsewhere.
	log := runWith(t,
		answeredOn("observe/page", runlog.VerdictPass, thisBuild),
		answeredOn("observe/logs", runlog.VerdictPass, "0000000"))

	verdict := judge(twoCases(), log, map[string]waiver{}, thisBuild)
	if verdict.Passed {
		t.Fatal("a run against a different binary cleared this build")
	}
	if len(verdict.StaleBuild) != 1 || !strings.Contains(verdict.StaleBuild[0], "0000000") {
		t.Errorf("stale = %v; the report must name the build that was judged", verdict.StaleBuild)
	}
}

func TestAnUnansweredCaseBlocksTheRelease(t *testing.T) {
	t.Parallel()
	log := runWith(t, answeredOn("observe/page", runlog.VerdictPass, thisBuild))

	verdict := judge(twoCases(), log, map[string]waiver{}, thisBuild)
	if verdict.Passed {
		t.Fatal("a half-finished run cleared the release")
	}
	if len(verdict.Unanswered) != 1 || verdict.Unanswered[0] != "observe/logs" {
		t.Errorf("unanswered = %v, want the case nobody judged", verdict.Unanswered)
	}
}

func TestASingleFailBlocksTheRelease(t *testing.T) {
	t.Parallel()
	log := runWith(t,
		answeredOn("observe/page", runlog.VerdictPass, thisBuild),
		answeredOn("observe/logs", runlog.VerdictFail, thisBuild))

	verdict := judge(twoCases(), log, map[string]waiver{}, thisBuild)
	if verdict.Passed {
		t.Fatal("a release went out with a FAIL on the record")
	}
	if !strings.Contains(verdict.report(), "FAILED") {
		t.Errorf("the report does not name the failure:\n%s", verdict.report())
	}
}

func TestAWaiverClearsACaseOnlyWhenItAcceptsSomething(t *testing.T) {
	t.Parallel()
	log := runWith(t, answeredOn("observe/page", runlog.VerdictPass, thisBuild))

	empty := map[string]waiver{"observe/logs": {CaseID: "observe/logs", Reason: "later", Owner: "someone"}}
	if judge(twoCases(), log, empty, thisBuild).Passed {
		t.Error("a waiver saying \"later\" cleared a case nobody judged")
	}
	anonymous := map[string]waiver{"observe/logs": {
		CaseID: "observe/logs",
		Reason: "the console fixture needs a browser we do not have in this environment",
	}}
	if judge(twoCases(), log, anonymous, thisBuild).Passed {
		t.Error("a waiver with nobody's name on it cleared a case")
	}

	// Control: a waiver that names an owner and a real risk does clear it, or
	// there would be no way to ship with a known, accepted gap.
	real := map[string]waiver{"observe/logs": {
		CaseID: "observe/logs",
		Reason: "the console fixture needs a browser we do not have in this environment",
		Owner:  "brennhill",
	}}
	verdict := judge(twoCases(), log, real, thisBuild)
	if !verdict.Passed {
		t.Fatalf("a properly recorded waiver did not clear the case:\n%s", verdict.report())
	}
	if len(verdict.Waived) != 1 || !strings.Contains(verdict.Waived[0], "brennhill") {
		t.Errorf("waived = %v; what shipped with an accepted risk must be visible", verdict.Waived)
	}
}

func TestAWaiverForACaseThatPassesIsRefused(t *testing.T) {
	t.Parallel()
	// waiverFile nobody prunes accumulate, and one left on a passing case silently
	// covers it if it later starts failing.
	log := runWith(t,
		answeredOn("observe/page", runlog.VerdictPass, thisBuild),
		answeredOn("observe/logs", runlog.VerdictPass, thisBuild))
	stale := map[string]waiver{"observe/logs": {
		CaseID: "observe/logs",
		Reason: "the console fixture needs a browser we do not have in this environment",
		Owner:  "brennhill",
	}}

	verdict := judge(twoCases(), log, stale, thisBuild)
	if verdict.Passed {
		t.Fatal("a waiver covering a passing case was accepted")
	}
	if len(verdict.StaleWaivers) != 1 {
		t.Errorf("stale waivers = %v, want the one that is no longer needed", verdict.StaleWaivers)
	}
}

func TestBlockedIsNotAPass(t *testing.T) {
	t.Parallel()
	log := runWith(t,
		answeredOn("observe/page", runlog.VerdictPass, thisBuild),
		answeredOn("observe/logs", runlog.VerdictBlocked, thisBuild))

	verdict := judge(twoCases(), log, map[string]waiver{}, thisBuild)
	if verdict.Passed {
		t.Fatal("a case nobody could judge was treated as judged")
	}
	if len(verdict.Blocked) != 1 {
		t.Errorf("blocked = %v", verdict.Blocked)
	}
}

func TestTheReportNamesTheBuildAndTheCounts(t *testing.T) {
	t.Parallel()
	log := runWith(t, answeredOn("observe/page", runlog.VerdictPass, thisBuild))
	report := judge(twoCases(), log, map[string]waiver{}, thisBuild).report()

	// Whoever reads a blocked release has to be able to act on it without
	// opening the log by hand.
	for _, want := range []string{thisBuild, "PASS 1", "NEVER ANSWERED", "make uat-human"} {
		if !strings.Contains(report, want) {
			t.Errorf("the report does not mention %q:\n%s", want, report)
		}
	}
}

func TestAFailWithNoEvidenceIsNamedAsUnreproducible(t *testing.T) {
	t.Parallel()
	// A red case nobody can reopen has to be fixed from the tester's memory,
	// which is the difference between a defect and an anecdote.
	failed := answeredOn("observe/logs", runlog.VerdictFail, thisBuild)
	log := runWith(t, answeredOn("observe/page", runlog.VerdictPass, thisBuild), failed)

	verdict := judge(twoCases(), log, map[string]waiver{}, thisBuild)
	if len(verdict.UnreproducibleFailures) != 1 {
		t.Fatalf("unreproducible = %v, want the FAIL that captured nothing", verdict.UnreproducibleFailures)
	}
	if !strings.Contains(verdict.report(), "FAILURES NOBODY ELSE CAN REPRODUCE") {
		t.Errorf("the report does not surface it:\n%s", verdict.report())
	}
}

func TestAFailWithABundleIsNotFlaggedAsUnreproducible(t *testing.T) {
	t.Parallel()
	// Control: with evidence and a manifest on disk the FAIL is actionable, and
	// flagging it anyway would make the signal meaningless.
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "console.json"), []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, evidence.ManifestName), []byte(`{"case_id":"observe/logs"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	withBundle := answeredOn("observe/logs", runlog.VerdictFail, thisBuild)
	withBundle.Evidence = []string{filepath.Join(dir, "console.json")}
	log := runWith(t, answeredOn("observe/page", runlog.VerdictPass, thisBuild), withBundle)

	if flagged := judge(twoCases(), log, map[string]waiver{}, thisBuild).UnreproducibleFailures; len(flagged) != 0 {
		t.Errorf("a FAIL with a complete bundle was flagged: %v", flagged)
	}
}

func TestAFailWhoseEvidenceWasDeletedIsFlagged(t *testing.T) {
	t.Parallel()
	// The bundle is checked at release time rather than at capture time because
	// evidence can be captured and then deleted, and the release is the moment
	// that matters.
	gone := answeredOn("observe/logs", runlog.VerdictFail, thisBuild)
	gone.Evidence = []string{filepath.Join(t.TempDir(), "console.json")}
	log := runWith(t, answeredOn("observe/page", runlog.VerdictPass, thisBuild), gone)

	flagged := judge(twoCases(), log, map[string]waiver{}, thisBuild).UnreproducibleFailures
	if len(flagged) != 1 || !strings.Contains(flagged[0], "gone") {
		t.Errorf("unreproducible = %v, want the FAIL whose evidence is missing", flagged)
	}
}
