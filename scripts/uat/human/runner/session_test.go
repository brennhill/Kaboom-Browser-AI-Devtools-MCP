// session_test.go — Proves a sitting records what was actually run, resumes
// where it stopped, and never attributes one case's response to another.

package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/scripts/uat/human/inventory"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/scripts/uat/human/runlog"
)

// scriptedCaller answers calls from a table and records what it was asked.
type scriptedCaller struct {
	responses map[string]string
	failWith  map[string]error
	calls     []string
}

func (s *scriptedCaller) call(tool string, arguments map[string]any) (json.RawMessage, error) {
	key := tool + "/" + toString(arguments["what"])
	s.calls = append(s.calls, key)
	if err := s.failWith[key]; err != nil {
		return nil, err
	}
	if body, ok := s.responses[key]; ok {
		return json.RawMessage(body), nil
	}
	return json.RawMessage(`{}`), nil
}

func toString(v any) string {
	text, _ := v.(string)
	return text
}

// scriptedPrompter answers each case from a queue.
type scriptedPrompter struct {
	answers []humanAnswer
	seen    []callOutcome
}

func (p *scriptedPrompter) present(index, total int, c inventory.Case, outcome callOutcome) (humanAnswer, error) {
	p.seen = append(p.seen, outcome)
	if len(p.answers) == 0 {
		return humanAnswer{Quit: true}, nil
	}
	next := p.answers[0]
	p.answers = p.answers[1:]
	return next, nil
}

func modeCase(id, tool, mode string) inventory.Case {
	return inventory.Case{
		ID: id, Kind: inventory.KindMCPMode, Tool: tool, Mode: mode,
		Setup: "Open a page with something to find.", Question: "Is what came back what is on the screen?",
	}
}

func newSession(t *testing.T, mcpSession caller, prompt presenter) (*session, *runlog.Log) {
	t.Helper()
	log, err := runlog.OpenLog(filepath.Join(t.TempDir(), "run.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = log.Close() })
	return &session{log: log, mcpSession: mcpSession, prompt: prompt, runID: "run-1", buildSHA: "abc1234"}, log
}

func TestTheRecordHoldsTheCallThatWasActuallyMade(t *testing.T) {
	t.Parallel()
	mcpSession := &scriptedCaller{responses: map[string]string{"observe/screenshot": `{"data_url":"data:image/png;base64,AA"}`}}
	prompt := &scriptedPrompter{answers: []humanAnswer{{Verdict: runlog.VerdictPass}}}
	session, log := newSession(t, mcpSession, prompt)

	if err := session.presentAll([]inventory.Case{modeCase("observe/screenshot", "observe", "screenshot")}, false); err != nil {
		t.Fatal(err)
	}

	record, answered := log.Answered("observe/screenshot")
	if !answered {
		t.Fatal("nothing was recorded")
	}
	// Without the request, a runlog.Verdict cannot be reproduced: nobody can tell which
	// arguments were judged.
	if !strings.Contains(string(record.Request), `"what":"screenshot"`) {
		t.Errorf("request = %s, want the call under test", record.Request)
	}
	if !strings.Contains(string(record.Response), "data:image/png") {
		t.Errorf("response = %s, want what the server returned", record.Response)
	}
	if record.BuildSHA != "abc1234" || record.RunID != "run-1" {
		t.Errorf("record does not say which build was judged: %+v", record)
	}
}

func TestAFailingCallIsRecordedAndTheRunContinues(t *testing.T) {
	t.Parallel()
	mcpSession := &scriptedCaller{failWith: map[string]error{"interact/click": errors.New("no tracked tab")}}
	prompt := &scriptedPrompter{answers: []humanAnswer{{Verdict: runlog.VerdictFail, Note: "no tab"}, {Verdict: runlog.VerdictPass}}}
	session, log := newSession(t, mcpSession, prompt)

	cases := []inventory.Case{modeCase("interact/click", "interact", "click"), modeCase("observe/page", "observe", "page")}
	if err := session.presentAll(cases, false); err != nil {
		t.Fatal(err)
	}

	failed, _ := log.Answered("interact/click")
	if !strings.Contains(failed.CallError, "no tracked tab") {
		t.Errorf("call_error = %q, want the server's reason", failed.CallError)
	}
	// One tool erroring must not cost the 193 cases behind it.
	if _, answered := log.Answered("observe/page"); !answered {
		t.Error("the run stopped at the first failing call")
	}
}

func TestQuittingLeavesTheCurrentCaseUnanswered(t *testing.T) {
	t.Parallel()
	mcpSession := &scriptedCaller{}
	prompt := &scriptedPrompter{answers: []humanAnswer{{Verdict: runlog.VerdictPass}, {Quit: true}}}
	session, log := newSession(t, mcpSession, prompt)

	cases := []inventory.Case{modeCase("a/one", "observe", "page"), modeCase("a/two", "observe", "logs"), modeCase("a/three", "observe", "errors")}
	if err := session.presentAll(cases, false); err != nil {
		t.Fatal(err)
	}

	if _, answered := log.Answered("a/one"); !answered {
		t.Error("the answered case was lost when the tester quit")
	}
	for _, id := range []string{"a/two", "a/three"} {
		if _, answered := log.Answered(id); answered {
			t.Errorf("%s was recorded although the tester quit before answering it", id)
		}
	}
}

func TestAnsweredCasesAreNotPresentedAgainUnlessAsked(t *testing.T) {
	t.Parallel()
	mcpSession := &scriptedCaller{}
	prompt := &scriptedPrompter{answers: []humanAnswer{{Verdict: runlog.VerdictPass}}}
	session, log := newSession(t, mcpSession, prompt)
	cases := []inventory.Case{modeCase("a/one", "observe", "page")}

	if err := session.presentAll(cases, false); err != nil {
		t.Fatal(err)
	}
	presentedFirst := len(prompt.seen)
	if err := session.presentAll(cases, false); err != nil {
		t.Fatal(err)
	}
	if len(prompt.seen) != presentedFirst {
		t.Error("an answered case was presented again, so a resumed run would re-ask everything")
	}

	// Control: --redo must still be able to re-present it, or a fixed case could
	// never be re-judged.
	prompt.answers = []humanAnswer{{Verdict: runlog.VerdictPass}}
	if err := session.presentAll(cases, true); err != nil {
		t.Fatal(err)
	}
	if len(prompt.seen) == presentedFirst {
		t.Error("--redo did not re-present the case")
	}
	if _, answered := log.Answered("a/one"); !answered {
		t.Error("the case lost its humanAnswer")
	}
}

func TestEvidenceIsCapturedAroundTheCallAndFailuresAreWritten(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	mcpSession := &scriptedCaller{
		responses: map[string]string{"observe/logs": `{"logs":[]}`},
		failWith:  map[string]error{"observe/network_waterfall": errors.New("extension not connected")},
	}
	prompt := &scriptedPrompter{answers: []humanAnswer{{Verdict: runlog.VerdictPass}}}
	session, log := newSession(t, mcpSession, prompt)
	session.evidenceDir = filepath.Join(dir, "evidence")

	if err := session.presentAll([]inventory.Case{modeCase("observe/page", "observe", "page")}, false); err != nil {
		t.Fatal(err)
	}

	record, _ := log.Answered("observe/page")
	if len(record.Evidence) != 6 {
		t.Fatalf("evidence = %v, want three probes before and three after", record.Evidence)
	}
	for _, path := range record.Evidence {
		if _, err := os.Stat(path); err != nil {
			t.Errorf("the record names %s but nothing is there: %v", path, err)
		}
	}
	// A probe that could not run must leave a file saying so. An empty evidence
	// directory reads as "capture was off", which is the opposite conclusion.
	failure := filepath.Join(session.evidenceDir, "observe__page", "before", "network.error")
	content, err := os.ReadFile(failure)
	if err != nil {
		t.Fatalf("the failed probe left nothing behind: %v", err)
	}
	if !strings.Contains(string(content), "extension not connected") {
		t.Errorf("the .error file does not say why: %q", content)
	}
}

func TestASurfaceCaseMakesNoToolCallButStillCapturesTheScreen(t *testing.T) {
	t.Parallel()
	mcpSession := &scriptedCaller{}
	prompt := &scriptedPrompter{answers: []humanAnswer{{Verdict: runlog.VerdictPass}}}
	session, log := newSession(t, mcpSession, prompt)
	session.evidenceDir = filepath.Join(t.TempDir(), "evidence")

	surface := inventory.Case{
		ID: "popup/pilot_toggle", Kind: inventory.KindSurface,
		Setup: "Open the popup with a tab tracked.", Question: "Does the pilot switch move when you click it?",
	}
	if err := session.presentAll([]inventory.Case{surface}, false); err != nil {
		t.Fatal(err)
	}

	record, _ := log.Answered("popup/pilot_toggle")
	if len(record.Request) != 0 {
		t.Errorf("a surface case invented a tool call: %s", record.Request)
	}
	if len(record.Evidence) != 3 {
		t.Errorf("evidence = %v, want the three probes; a surface verdict is unreviewable without a screenshot", record.Evidence)
	}
	for _, call := range mcpSession.calls {
		if strings.HasPrefix(call, "popup") {
			t.Errorf("the runner called a tool for a surface case: %s", call)
		}
	}
}
