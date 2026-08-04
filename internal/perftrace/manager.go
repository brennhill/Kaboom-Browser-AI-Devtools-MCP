// manager.go — Owns one bounded, append-only Chrome trace artifact lifecycle.

package perftrace

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var ErrTraceActive = errors.New("a performance trace is already active")

type activeTrace struct {
	id          string
	partialPath string
	finalPath   string
	file        *os.File
	nextSeq     int
	eventCount  int64
	bytes       int64
	firstEvent  bool
}

type StartedTrace struct {
	TraceID     string
	PartialPath string
}

type Manager struct {
	mu     sync.Mutex
	dir    string
	active *activeTrace
}

func NewManager(dir string) *Manager { return &Manager{dir: dir} }

func (m *Manager) Start(tabID int) (StartedTrace, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.active != nil {
		return StartedTrace{}, ErrTraceActive
	}
	if tabID <= 0 {
		return StartedTrace{}, errors.New("tab_id must be a positive integer")
	}
	if err := os.MkdirAll(m.dir, 0o700); err != nil {
		return StartedTrace{}, fmt.Errorf("create performance trace directory: %w", err)
	}
	id, err := randomID()
	if err != nil {
		return StartedTrace{}, err
	}
	base := fmt.Sprintf("cpu-%s-%s", time.Now().UTC().Format("20060102T150405Z"), id)
	partial := filepath.Join(m.dir, base+".json.partial")
	final := filepath.Join(m.dir, base+".json")
	f, err := os.OpenFile(partial, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return StartedTrace{}, fmt.Errorf("create performance trace artifact: %w", err)
	}
	if _, err := f.WriteString(`{"traceEvents":[`); err != nil {
		_ = f.Close()
		_ = os.Remove(partial)
		return StartedTrace{}, fmt.Errorf("initialize performance trace artifact: %w", err)
	}
	m.active = &activeTrace{id: id, partialPath: partial, finalPath: final, file: f, firstEvent: true, bytes: int64(len(`{"traceEvents":[`))}
	return StartedTrace{TraceID: id, PartialPath: partial}, nil
}

func (m *Manager) Append(req WirePerformanceTraceChunkRequest) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	trace, err := m.require(req.TraceID)
	if err != nil {
		return err
	}
	if req.Sequence != trace.nextSeq {
		return fmt.Errorf("trace chunk sequence %d is out of order; expected %d", req.Sequence, trace.nextSeq)
	}
	for _, event := range req.Events {
		if !json.Valid(event) {
			return errors.New("trace chunk contains invalid JSON event")
		}
	}
	for _, event := range req.Events {
		prefix := ""
		if !trace.firstEvent {
			prefix = ","
		}
		written, writeErr := trace.file.Write(append([]byte(prefix), event...))
		if writeErr != nil {
			return fmt.Errorf("append performance trace event: %w", writeErr)
		}
		trace.bytes += int64(written)
		trace.eventCount++
		trace.firstEvent = false
	}
	trace.nextSeq++
	return nil
}

func (m *Manager) Finish(req WirePerformanceTraceFinishRequest) (WirePerformanceTraceResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	trace, err := m.require(req.TraceID)
	if err != nil {
		return WirePerformanceTraceResult{}, err
	}
	if _, err := trace.file.WriteString(`]}`); err != nil {
		return WirePerformanceTraceResult{}, fmt.Errorf("finalize performance trace artifact: %w", err)
	}
	trace.bytes += 2
	if err := trace.file.Sync(); err != nil {
		return WirePerformanceTraceResult{}, fmt.Errorf("sync performance trace artifact: %w", err)
	}
	if err := trace.file.Close(); err != nil {
		return WirePerformanceTraceResult{}, fmt.Errorf("close performance trace artifact: %w", err)
	}
	if err := os.Rename(trace.partialPath, trace.finalPath); err != nil {
		return WirePerformanceTraceResult{}, fmt.Errorf("publish performance trace artifact: %w", err)
	}
	result := WirePerformanceTraceResult{
		TraceID: trace.id, ArtifactPath: trace.finalPath, EventCount: trace.eventCount,
		ChunkCount: trace.nextSeq, Bytes: trace.bytes, TabID: req.TabID, URL: req.URL,
		NavigationID: req.NavigationID, BuildSHA: req.BuildSHA,
	}
	m.active = nil
	return result, nil
}

func (m *Manager) Abort(traceID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	trace, err := m.require(traceID)
	if err != nil {
		return err
	}
	closeErr := trace.file.Close()
	removeErr := os.Remove(trace.partialPath)
	m.active = nil
	if closeErr != nil {
		return fmt.Errorf("close aborted performance trace: %w", closeErr)
	}
	if removeErr != nil && !os.IsNotExist(removeErr) {
		return fmt.Errorf("remove aborted performance trace: %w", removeErr)
	}
	return nil
}

func (m *Manager) require(traceID string) (*activeTrace, error) {
	if m.active == nil {
		return nil, errors.New("no performance trace is active")
	}
	if traceID == "" || traceID != m.active.id {
		return nil, errors.New("performance trace id does not match the active trace")
	}
	return m.active, nil
}

func randomID() (string, error) {
	var raw [12]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", fmt.Errorf("generate performance trace id: %w", err)
	}
	return hex.EncodeToString(raw[:]), nil
}
