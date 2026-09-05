// session.go — Drives one sitting: call the tool, capture evidence, record the answer.

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/scripts/uat/human/evidence"
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
	// fixtureSHA pins the fixture pages the tester was looking at. Without it a
	// FAIL cannot be reproduced: the same build against a changed fixture is a
	// different experiment.
	fixtureSHA string
	// bundles are this case's open evidence directories, closed with a manifest
	// once the answer is in.
	bundles []*evidence.Bundle
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
		s.closeBundles(c, outcome)
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
// Every case is judged from what a person sees, and a verdict recorded without
// evidence cannot be re-examined a week later. The bundle is written around each
// call and its probe failures are recorded, never swallowed: "no screenshot" and
// "the screenshot showed the wrong page" must not look alike in the run log.

// captureEvidence writes one phase's probes and returns the paths written.
func (s *session) captureEvidence(c inventory.Case, phase string) []string {
	if s.evidenceDir == "" {
		return nil
	}
	bundle, err := evidence.Open(s.evidenceDir, c.ID+"/"+phase)
	if err != nil {
		// Fail loud: the tester is about to judge a case whose evidence will not
		// exist, and only they can decide whether that is acceptable.
		fmt.Fprintf(os.Stderr, "human-uat: %v\n", err)
		return nil
	}
	for _, probe := range evidence.Probes() {
		s.captureProbe(bundle, probe)
	}
	s.bundles = append(s.bundles, bundle)
	return bundle.Paths()
}

// captureProbe stores one probe's answer, or the reason it had none.
func (s *session) captureProbe(bundle *evidence.Bundle, probe evidence.Probe) {
	response, err := s.mcpSession.call(probe.Tool, probe.Args)
	if err != nil {
		bundle.WriteFailure(probe.Name, err.Error())
		return
	}
	if probe.Name == "screenshot" {
		bundle.WriteScreenshot(probe.Name, response)
		return
	}
	bundle.Write(probe.Name+".json", response)
}

// closeBundles writes the manifest for every bundle this case produced.
//
// Written after the answer so the manifest carries the call under test and its
// response, which is what lets a reader reopen the case without the person who
// ran it.
func (s *session) closeBundles(c inventory.Case, outcome callOutcome) {
	for _, bundle := range s.bundles {
		if err := bundle.WriteManifest(evidence.Manifest{
			CaseID:     c.ID,
			Question:   c.Question,
			BuildSHA:   s.buildSHA,
			FixtureSHA: s.fixtureSHA,
			RunID:      s.runID,
			Request:    outcome.Request,
			Response:   outcome.Response,
			CallError:  outcome.Err,
			CapturedAt: runlog.Timestamp(s.clock()),
		}); err != nil {
			fmt.Fprintf(os.Stderr, "human-uat: could not write %s for %s: %v\n", evidence.ManifestName, c.ID, err)
		}
	}
	s.bundles = nil
}
