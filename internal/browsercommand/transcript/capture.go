// capture.go — Pairs issued commands with their outcomes and appends a transcript.
//
// PURPOSE: the daemon sees a command go out and its result come back in two
// different requests. This holds the open commands until their outcome arrives,
// so a transcript records what the extension actually answered rather than what
// was asked.
//
// CONTRACT: records are appended as they complete, not flushed at exit. The UAT
// harness stops daemons with a signal, so anything held for shutdown is lost.
// Writing is diagnostic scaffolding and must never fail a sync — errors are
// retained and reported by Err rather than propagated into the request path.

package synctranscript

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"sync"
)

// FileRecorder pairs exchanges and appends each completed one to a JSONL file.
type FileRecorder struct {
	mu       sync.Mutex
	open     map[string]Command
	path     string
	file     *os.File
	written  int
	firstErr error
}

// NewFileRecorder returns a recorder that appends to path. The file is created
// on the first completed exchange, so a run that records nothing leaves no
// empty transcript to be mistaken for a valid one.
func NewFileRecorder(path string) *FileRecorder {
	return &FileRecorder{open: make(map[string]Command), path: path}
}

// Issued notes a command handed to the extension.
func (r *FileRecorder) Issued(id, kind string, params json.RawMessage) {
	if id == "" {
		// EXPECTED_ABSENCE: a command with no id cannot be paired with its
		// result, and recording it unpaired would produce an entry replay can
		// never match.
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.open[id] = Command{Type: kind, Params: append(json.RawMessage(nil), params...)}
}

// Completed pairs an outcome with the command that produced it and appends it.
func (r *FileRecorder) Completed(id, status string, result json.RawMessage, failure string) {
	if id == "" {
		// EXPECTED_ABSENCE: as above — an unidentifiable result cannot be paired.
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	command, known := r.open[id]
	if !known {
		// EXPECTED_ABSENCE: results arrive for commands issued before recording
		// started, which is normal when a daemon is recorded mid-session.
		return
	}
	delete(r.open, id)
	r.append(Record{
		Type:        command.Type,
		Fingerprint: Fingerprint(command.Type, command.Params),
		Params:      command.Params,
		Status:      status,
		Result:      append(json.RawMessage(nil), result...),
		Error:       failure,
	})
}

// append writes one record, retaining the first failure for Err.
func (r *FileRecorder) append(record Record) {
	if r.file == nil {
		file, err := os.Create(r.path)
		if err != nil {
			r.retain(fmt.Errorf("create transcript %s: %w", r.path, err))
			return
		}
		r.file = file
	}
	if err := Encode(r.file, []Record{record}); err != nil {
		r.retain(err)
		return
	}
	r.written++
}

func (r *FileRecorder) retain(err error) {
	if r.firstErr == nil {
		r.firstErr = err
	}
}

// Written reports how many exchanges reached the file.
func (r *FileRecorder) Written() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.written
}

// Err reports the first write failure, if any. A recorder that silently wrote
// nothing would produce an empty transcript and a replay that answers nothing.
func (r *FileRecorder) Err() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.firstErr
}

// Unanswered lists commands issued but never completed — a browser that hung,
// or a run stopped mid-command. They are reported rather than written, because
// a transcript entry with no outcome would replay as an answer of nothing.
func (r *FileRecorder) Unanswered() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	open := make([]string, 0, len(r.open))
	for id := range r.open {
		open = append(open, id)
	}
	sort.Strings(open)
	return open
}

// Close releases the file handle.
func (r *FileRecorder) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.file == nil {
		// EXPECTED_ABSENCE: a recorder that saw no completed exchange never
		// opened a file, and closing nothing is not an error.
		return r.firstErr
	}
	err := r.file.Close()
	r.file = nil
	if r.firstErr != nil {
		return r.firstErr
	}
	return err
}
