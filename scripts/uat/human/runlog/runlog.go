// Purpose: The human UAT run log — one JSON record per case, appended as it is answered.
// Why: The runner writes it and the release gate reads it; both must agree on what an answer is.
// Docs: docs/features/feature/human-uat-rig/index.md
//
// Append-only and read back on start, so a 194-case run survives being done over
// several sittings and a crash costs at most the case in progress.

package runlog

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

// Verdict is the person's answer. There is no "probably".
type Verdict string

const (
	VerdictPass    Verdict = "PASS"
	VerdictFail    Verdict = "FAIL"
	VerdictBlocked Verdict = "BLOCKED"
	// VerdictSkipped is recorded when the tester defers a case. It is not a
	// result: the coverage ratchet counts it as unanswered, because a skipped
	// case that counted as covered would let the whole rig be skipped green.
	VerdictSkipped Verdict = "SKIPPED"
)

// Verdicts lists the answers in prompt order.
func Verdicts() []Verdict {
	return []Verdict{VerdictPass, VerdictFail, VerdictBlocked, VerdictSkipped}
}

// Answered reports whether a Verdict settles a case.
func (v Verdict) Answered() bool {
	return v == VerdictPass || v == VerdictFail || v == VerdictBlocked
}

// Result is one Answered case.
//
// Field order is the record's order on disk, and every run writes the same keys,
// so two runs of the same inventory diff line by line and a regression shows up
// as a Verdict flipping rather than as a reordered file.
type Result struct {
	CaseID     string          `json:"case_id"`
	Kind       string          `json:"kind"`
	Tool       string          `json:"tool,omitempty"`
	Mode       string          `json:"mode,omitempty"`
	Verdict    Verdict         `json:"Verdict"`
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

// Log appends results to a file and remembers what is already Answered.
type Log struct {
	path  string
	file  *os.File
	prior map[string]Result
}

// OpenLog opens (or creates) the run log and indexes what it already holds.
func OpenLog(path string) (*Log, error) {
	prior, err := readResults(path)
	if err != nil {
		return nil, err
	}
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, err
	}
	return &Log{path: path, file: file, prior: prior}, nil
}

// readResults reads every record in the log, last write winning per case.
//
// A corrupt trailing line is tolerated: it is the record a crash cut in half,
// and refusing to open the log because of it would throw away every answer
// before it.
func readResults(path string) (map[string]Result, error) {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]Result{}, nil
		}
		return nil, err
	}
	defer file.Close()

	byCase := map[string]Result{}
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var record Result
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

// Answered returns the recorded Verdict for a case, if it has one that counts.
func (l *Log) Answered(caseID string) (Result, bool) {
	record, ok := l.prior[caseID]
	if !ok || !record.Verdict.Answered() {
		return Result{}, false
	}
	return record, true
}

// Append writes one record and flushes it.
//
// Flushed per record rather than at the end: the value of the log is that a
// tester who stops halfway keeps every answer given so far.
func (l *Log) Append(record Result) error {
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

// Close releases the log file.
func (l *Log) Close() error { return l.file.Close() }

// Tally counts the log by Verdict.
type Tally struct {
	Pass, Fail, Blocked, Skipped, Unanswered int
}

// Summarize counts every case in the inventory against the log.
//
// Cases with no record at all are counted as unanswered rather than omitted: a
// run that Answered nine cases out of 194 must not report 100%.
func Summarize(cases []inventory.Case, log *Log) Tally {
	var Tally Tally
	for _, c := range cases {
		switch log.prior[c.ID].Verdict {
		case VerdictPass:
			Tally.Pass++
		case VerdictFail:
			Tally.Fail++
		case VerdictBlocked:
			Tally.Blocked++
		case VerdictSkipped:
			Tally.Skipped++
		default:
			Tally.Unanswered++
		}
	}
	return Tally
}

// FailedCases lists the ids a run Answered FAIL, in a stable order.
func FailedCases(log *Log) []string {
	var failed []string
	for id, record := range log.prior {
		if record.Verdict == VerdictFail {
			failed = append(failed, id)
		}
	}
	sort.Strings(failed)
	return failed
}

// Timestamp is the one time format the log uses.
func Timestamp(at time.Time) string { return at.UTC().Format(time.RFC3339) }

// DescribeTally renders a tally the way the release gate reports it.
func DescribeTally(counts Tally, total int) string {
	return fmt.Sprintf("PASS %d, FAIL %d, BLOCKED %d, SKIPPED %d, UNANSWERED %d of %d",
		counts.Pass, counts.Fail, counts.Blocked, counts.Skipped, counts.Unanswered, total)
}
