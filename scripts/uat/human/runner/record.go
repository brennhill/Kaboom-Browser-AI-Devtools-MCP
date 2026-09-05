// record.go — The run log: one JSON record per case, appended as it is answered.
//
// appendResult-only and read back on start, so a 194-case run survives being done over
// several sittings and a crash costs at most the case in progress.

package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/scripts/uat/human/inventory"
)

// verdict is the person's answer. There is no "probably".
type verdict string

const (
	verdictPass    verdict = "PASS"
	verdictFail    verdict = "FAIL"
	verdictBlocked verdict = "BLOCKED"
	// verdictSkipped is recorded when the tester defers a case. It is not a
	// result: the coverage ratchet counts it as unanswered, because a skipped
	// case that counted as covered would let the whole rig be skipped green.
	verdictSkipped verdict = "SKIPPED"
)

// verdicts lists the answers in prompt order.
func verdicts() []verdict {
	return []verdict{verdictPass, verdictFail, verdictBlocked, verdictSkipped}
}

// answered reports whether a verdict settles a case.
func (v verdict) answered() bool {
	return v == verdictPass || v == verdictFail || v == verdictBlocked
}

// caseRecord is one answered case.
//
// Field order is the record's order on disk, and every run writes the same keys,
// so two runs of the same inventory diff line by line and a regression shows up
// as a verdict flipping rather than as a reordered file.
type caseRecord struct {
	CaseID     string          `json:"case_id"`
	Kind       string          `json:"kind"`
	Tool       string          `json:"tool,omitempty"`
	Mode       string          `json:"mode,omitempty"`
	Verdict    verdict         `json:"verdict"`
	Note       string          `json:"note"`
	Question   string          `json:"question"`
	Request    json.RawMessage `json:"request,omitempty"`
	Response   json.RawMessage `json:"response,omitempty"`
	CallError  string          `json:"call_error,omitempty"`
	Evidence   []string        `json:"evidence,omitempty"`
	StartedAt  string          `json:"started_at"`
	AnsweredAt string          `json:"answered_at"`
	BuildSHA   string          `json:"build_sha"`
	RunID      string          `json:"run_id"`
}

// runLog appends results to a file and remembers what is already answered.
type runLog struct {
	path  string
	file  *os.File
	prior map[string]caseRecord
}

// openLog opens (or creates) the run log and indexes what it already holds.
func openLog(path string) (*runLog, error) {
	prior, err := readResults(path)
	if err != nil {
		return nil, err
	}
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, err
	}
	return &runLog{path: path, file: file, prior: prior}, nil
}

// readResults reads every record in the log, last write winning per case.
//
// A corrupt trailing line is tolerated: it is the record a crash cut in half,
// and refusing to open the log because of it would throw away every answer
// before it.
func readResults(path string) (map[string]caseRecord, error) {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]caseRecord{}, nil
		}
		return nil, err
	}
	defer file.Close()

	byCase := map[string]caseRecord{}
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var record caseRecord
		if json.Unmarshal([]byte(line), &record) != nil || record.CaseID == "" {
			continue
		}
		byCase[record.CaseID] = record
	}
	if err := scanner.Err(); err != nil && err != io.EOF {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return byCase, nil
}

// answered returns the recorded verdict for a case, if it has one that counts.
func (l *runLog) answered(caseID string) (caseRecord, bool) {
	record, ok := l.prior[caseID]
	if !ok || !record.Verdict.answered() {
		return caseRecord{}, false
	}
	return record, true
}

// appendResult writes one record and flushes it.
//
// Flushed per record rather than at the end: the value of the log is that a
// tester who stops halfway keeps every answer given so far.
func (l *runLog) appendResult(record caseRecord) error {
	encoded, err := json.Marshal(record)
	if err != nil {
		return err
	}
	if _, err := l.file.Write(append(encoded, '\n')); err != nil {
		return err
	}
	if err := l.file.Sync(); err != nil {
		return fmt.Errorf("flush %s: %w", l.path, err)
	}
	l.prior[record.CaseID] = record
	return nil
}

// shutdown releases the log file.
func (l *runLog) shutdown() error { return l.file.Close() }

// tally counts the log by verdict.
type tally struct {
	Pass, Fail, Blocked, Skipped, Unanswered int
}

// summarize counts every case in the inventory against the log.
//
// Cases with no record at all are counted as unanswered rather than omitted: a
// run that answered nine cases out of 194 must not report 100%.
func summarize(cases []inventory.Case, log *runLog) tally {
	var tally tally
	for _, c := range cases {
		switch log.prior[c.ID].Verdict {
		case verdictPass:
			tally.Pass++
		case verdictFail:
			tally.Fail++
		case verdictBlocked:
			tally.Blocked++
		case verdictSkipped:
			tally.Skipped++
		default:
			tally.Unanswered++
		}
	}
	return tally
}

// failedCases lists the ids a run answered FAIL, in a stable order.
func failedCases(log *runLog) []string {
	var failed []string
	for id, record := range log.prior {
		if record.Verdict == verdictFail {
			failed = append(failed, id)
		}
	}
	sort.Strings(failed)
	return failed
}

// timestamp is the one time format the log uses.
func timestamp(at time.Time) string { return at.UTC().Format(time.RFC3339) }
