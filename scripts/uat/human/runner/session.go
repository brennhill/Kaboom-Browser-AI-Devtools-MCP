// session.go — Drives one sitting: call the tool, capture evidence, record the answer.

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/scripts/uat/human/inventory"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/scripts/uat/human/runlog"
)

// callOutcome is everything the tool produced for one case.
type callOutcome struct {
	Request  json.RawMessage
	Response json.RawMessage
	Err      string
	Evidence []string
}

// caller is the part of the MCP client a session needs, so a session can be
// driven in tests without a server process.
type caller interface {
	call(tool string, arguments map[string]any) (json.RawMessage, error)
}

// presenter is the part of the prompter a session needs.
type presenter interface {
	present(index, total int, c inventory.Case, outcome callOutcome) (humanAnswer, error)
}

type session struct {
	log        *runlog.Log
	mcpSession caller
	prompt     presenter
	runID      string
	buildSHA   string
	// evidenceDir is where captures are written. Empty means evidence is off,
	// which is what --no-evidence and every test that is not about evidence use.
	evidenceDir string
	// now is swappable so a test can assert the timestamps a record carries.
	now func() time.Time
}

func (s *session) clock() time.Time {
	if s.now == nil {
		return time.Now()
	}
	return s.now()
}

// presentAll runs every selected case in inventory order.
//
// Answers are written as they are given, not at the end: a tester who stops
// after 40 cases keeps 40 answers.
func (s *session) presentAll(cases []inventory.Case, redo bool) error {
	total := len(cases)
	for i, c := range cases {
		if _, answered := s.log.Answered(c.ID); answered && !redo {
			continue
		}
		startedAt := s.clock()
		outcome := s.runCase(c)
		answer, err := s.prompt.present(i+1, total, c, outcome)
		if err != nil {
			return err
		}
		if answer.Quit {
			return nil
		}
		if err := s.record(c, outcome, answer, startedAt); err != nil {
			return err
		}
	}
	return nil
}

// runCase makes the call under test. A surface case has no call: the person is
// looking at the browser, and the record holds only their answer.
func (s *session) runCase(c inventory.Case) callOutcome {
	if c.Kind != inventory.KindMCPMode {
		// A surface case has no call — the person is looking at the browser — but
		// it is the kind of case where a screenshot matters most, because there is
		// no tool response to re-read afterwards.
		return callOutcome{Evidence: s.captureEvidence(c, "surface")}
	}
	arguments := c.CallArguments()
	outcome := callOutcome{Request: marshalRequest(c.Tool, arguments)}
	outcome.Evidence = append(outcome.Evidence, s.captureEvidence(c, "before")...)
	response, err := s.mcpSession.call(c.Tool, arguments)
	if err != nil {
		// Recorded, not fatal: a tool that errors is a legitimate thing to judge,
		// and stopping the run would cost every unanswered case behind it.
		outcome.Err = err.Error()
	}
	outcome.Response = response
	outcome.Evidence = append(outcome.Evidence, s.captureEvidence(c, "after")...)
	return outcome
}

// record writes one result.
func (s *session) record(c inventory.Case, outcome callOutcome, answer humanAnswer, startedAt time.Time) error {
	return s.log.Append(runlog.Result{
		CaseID:     c.ID,
		Kind:       c.Kind,
		Tool:       c.Tool,
		Mode:       c.Mode,
		Verdict:    answer.Verdict,
		Note:       answer.Note,
		Question:   c.Question,
		Request:    outcome.Request,
		Response:   outcome.Response,
		CallError:  outcome.Err,
		Evidence:   outcome.Evidence,
		StartedAt:  runlog.Timestamp(startedAt),
		AnsweredAt: runlog.Timestamp(s.clock()),
		BuildSHA:   s.buildSHA,
		RunID:      s.runID,
	})
}

// ── Evidence ────────────────────────────────────────────────────────────────
//
// Every case is judged from what a person sees, and a runlog.Verdict recorded without
// evidence cannot be re-examined a week later. The captures below are attempted
// around each call and their failures are recorded, never swallowed: "no
// screenshot" and "screenshot showed the wrong page" must not look alike in the
// run log.

// evidenceProbe is one thing captured beside a case.
type evidenceProbe struct {
	name string
	tool string
	args map[string]any
}

// evidenceProbes are captured before and after the call under test.
//
// Console and network come from the same daemon the tool call went to, so they
// describe the same browser the tester is looking at.
func evidenceProbes() []evidenceProbe {
	return []evidenceProbe{
		{name: "screenshot", tool: "observe", args: map[string]any{"what": "screenshot"}},
		{name: "console", tool: "observe", args: map[string]any{"what": "logs"}},
		{name: "network", tool: "observe", args: map[string]any{"what": "network_waterfall"}},
	}
}

// captureEvidence writes one phase's probes and returns the paths written.
//
// A probe that fails writes a .error file rather than nothing: an empty evidence
// directory is ambiguous between "capture failed" and "capture was off", and the
// two lead to opposite conclusions about a FAIL.
func (s *session) captureEvidence(c inventory.Case, phase string) []string {
	if s.evidenceDir == "" {
		return nil
	}
	dir := filepath.Join(s.evidenceDir, safeName(c.ID), phase)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return []string{fmt.Sprintf("%s: could not create evidence directory: %v", dir, err)}
	}
	var written []string
	for _, probe := range evidenceProbes() {
		written = append(written, s.captureProbe(dir, probe))
	}
	return written
}

func (s *session) captureProbe(dir string, probe evidenceProbe) string {
	response, err := s.mcpSession.call(probe.tool, probe.args)
	if err != nil {
		path := filepath.Join(dir, probe.name+".error")
		writeEvidenceFile(path, []byte(err.Error()))
		return path
	}
	path := filepath.Join(dir, probe.name+".json")
	writeEvidenceFile(path, response)
	return path
}

// writeEvidenceFile records a write failure in the file's place.
//
// Evidence is a side product; losing one file must not end a sitting. What it
// must not do is disappear quietly, so the failure is printed where the tester
// is already looking.
func writeEvidenceFile(path string, content []byte) {
	if err := os.WriteFile(path, content, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "human-uat: could not write %s: %v\n", path, err)
	}
}

// safeName makes a case id usable as a directory name.
func safeName(id string) string {
	return strings.ReplaceAll(id, "/", "__")
}
