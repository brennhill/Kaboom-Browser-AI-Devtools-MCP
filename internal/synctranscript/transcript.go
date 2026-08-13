// transcript.go — Records and replays the daemon↔extension command exchange.
//
// PURPOSE: the connected UAT categories are the only tests that prove a browser
// feature still works, and they run nowhere automated because they need Chrome,
// the extension and a human. A transcript is one live run's command/result
// pairs, captured once and replayed by a fake extension, so those categories
// become a deterministic gate instead of a thing someone remembers to do.
//
// CONTRACT: replay must never invent an answer. A command with no recording is
// a reported miss, not an empty success — the failure mode this whole exercise
// exists to prevent is a test that passes because nothing happened.

package synctranscript

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
)

// volatileKeys differ between runs and carry no meaning for matching. Leaving
// them in would make every fingerprint unique to the run that recorded it.
func volatileKeys() map[string]bool {
	return map[string]bool{
		"tab_id":                true,
		"correlation_id":        true,
		"trace_id":              true,
		"request_id":            true,
		"session_id":            true,
		"ext_session_id":        true,
		"connection_generation": true,
		"timestamp":             true,
		"started_at":            true,
		"updated_at":            true,
		"id":                    true,
	}
}

// Command is one daemon instruction, reduced to the parts replay depends on.
type Command struct {
	Type   string          `json:"type"`
	Params json.RawMessage `json:"params,omitempty"`
}

// Result is the extension's terminal outcome for a command.
type Result struct {
	Status string          `json:"status"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  string          `json:"error,omitempty"`
}

// Record is one exchange as stored on disk.
type Record struct {
	Type        string          `json:"type"`
	Fingerprint string          `json:"fingerprint"`
	Params      json.RawMessage `json:"params,omitempty"`
	Status      string          `json:"status"`
	Result      json.RawMessage `json:"result,omitempty"`
	Error       string          `json:"error,omitempty"`
}

// Fingerprint identifies a command by what it asks for, ignoring the fields
// that change between runs.
func Fingerprint(kind string, params json.RawMessage) string {
	digest := sha256.New()
	digest.Write([]byte(kind))
	digest.Write([]byte{0})
	digest.Write([]byte(canonical(params)))
	return hex.EncodeToString(digest.Sum(nil))[:32]
}

// canonical renders params with volatile keys removed and object keys sorted,
// so two runs of the same request produce the same bytes.
func canonical(params json.RawMessage) string {
	if len(params) == 0 {
		return ""
	}
	var value any
	if err := json.Unmarshal(params, &value); err != nil {
		// EXPECTED_ABSENCE: params that are not JSON cannot be normalized, and
		// hashing them verbatim still distinguishes one from another.
		return string(params)
	}
	var builder strings.Builder
	writeCanonical(&builder, value)
	return builder.String()
}

func writeCanonical(builder *strings.Builder, value any) {
	switch typed := value.(type) {
	case map[string]any:
		keys := make([]string, 0, len(typed))
		volatile := volatileKeys()
		for key := range typed {
			if !volatile[key] {
				keys = append(keys, key)
			}
		}
		sort.Strings(keys)
		builder.WriteByte('{')
		for i, key := range keys {
			if i > 0 {
				builder.WriteByte(',')
			}
			builder.WriteString(key)
			builder.WriteByte(':')
			writeCanonical(builder, typed[key])
		}
		builder.WriteByte('}')
	case []any:
		builder.WriteByte('[')
		for i, element := range typed {
			if i > 0 {
				builder.WriteByte(',')
			}
			writeCanonical(builder, element)
		}
		builder.WriteByte(']')
	default:
		fmt.Fprintf(builder, "%v", typed)
	}
}

// Recorder accumulates exchanges during a live run.
type Recorder struct {
	records []Record
}

func NewRecorder() *Recorder { return &Recorder{} }

// Observe stores one command and the extension's answer to it.
func (r *Recorder) Observe(command Command, result Result) {
	r.records = append(r.records, Record{
		Type:        command.Type,
		Fingerprint: Fingerprint(command.Type, command.Params),
		Params:      command.Params,
		Status:      result.Status,
		Result:      result.Result,
		Error:       result.Error,
	})
}

func (r *Recorder) Records() []Record { return r.records }

// Transcript answers commands from a recording, in recorded order per command
// shape, and remembers what it could not answer.
type Transcript struct {
	queues map[string][]Record
	cursor map[string]int
	misses map[string]int
}

func NewTranscript(records []Record) *Transcript {
	transcript := &Transcript{
		queues: make(map[string][]Record),
		cursor: make(map[string]int),
		misses: make(map[string]int),
	}
	for _, record := range records {
		print := record.Fingerprint
		if print == "" {
			print = Fingerprint(record.Type, record.Params)
		}
		transcript.queues[print] = append(transcript.queues[print], record)
	}
	return transcript
}

// Match returns the next recorded answer for a command shape.
//
// Exhaustion is a miss, not a repeat of the last answer: a category that runs
// one more probe than was recorded is asking something the transcript does not
// know, and handing back a stale result would turn that into a pass.
func (t *Transcript) Match(command Command) (Record, bool) {
	print := Fingerprint(command.Type, command.Params)
	queue := t.queues[print]
	index := t.cursor[print]
	if index >= len(queue) {
		t.misses[command.Type]++
		return Record{}, false
	}
	t.cursor[print] = index + 1
	return queue[index], true
}

// Unused reports recordings no command asked for — the signal that a category
// stopped exercising something it used to.
func (t *Transcript) Unused() []Record {
	var unused []Record
	prints := make([]string, 0, len(t.queues))
	for print := range t.queues {
		prints = append(prints, print)
	}
	sort.Strings(prints)
	for _, print := range prints {
		unused = append(unused, t.queues[print][t.cursor[print]:]...)
	}
	return unused
}

// Misses reports commands asked for but never recorded, counted by type.
func (t *Transcript) Misses() map[string]int {
	copied := make(map[string]int, len(t.misses))
	for kind, count := range t.misses {
		copied[kind] = count
	}
	return copied
}

// Encode writes records as JSONL, one exchange per line.
func Encode(writer io.Writer, records []Record) error {
	encoder := json.NewEncoder(writer)
	for _, record := range records {
		if err := encoder.Encode(record); err != nil {
			return fmt.Errorf("encode transcript: %w", err)
		}
	}
	return nil
}

// Decode reads a JSONL transcript, rejecting anything it cannot fully parse.
//
// A malformed transcript must stop the run. Skipping bad lines would replay as
// a partial transcript, and a fake extension that answers nothing is
// indistinguishable from a browser that never responded.
func Decode(reader io.Reader) ([]Record, error) {
	var records []Record
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	line := 0
	for scanner.Scan() {
		line++
		text := strings.TrimSpace(scanner.Text())
		if text == "" {
			continue
		}
		var record Record
		if err := json.Unmarshal([]byte(text), &record); err != nil {
			return nil, fmt.Errorf("transcript line %d: %w", line, err)
		}
		if record.Type == "" {
			return nil, fmt.Errorf("transcript line %d: record has no command type", line)
		}
		records = append(records, record)
	}
	if err := scanner.Err(); err != nil {
		return nil, errors.New("read transcript: " + err.Error())
	}
	return records, nil
}
