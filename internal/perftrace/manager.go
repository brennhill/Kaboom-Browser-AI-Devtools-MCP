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
var ErrTraceSizeLimit = errors.New("performance trace exceeded the total byte limit")
var ErrTraceDurationLimit = errors.New("performance trace exceeded the duration limit")

const (
	defaultMaxTraceBytes    int64 = 512 << 20
	defaultMaxTraceDuration       = 5 * time.Minute
)

type activeTrace struct {
	id          string
	partialPath string
	finalPath   string
	file        *os.File
	nextSeq     int
	eventCount  int64
	bytes       int64
	firstEvent  bool
	startedAt   time.Time
}

type StartedTrace struct {
	TraceID     string
	PartialPath string
}

type Manager struct {
	mu                sync.Mutex
	dir               string
	active            *activeTrace
	maxBytes          int64
	maxDuration       time.Duration
	now               func() time.Time
	startupRecovered  bool
	startupCleanupErr error
}

func NewManager(dir string) *Manager {
	m := newManagerWithLimits(dir, defaultMaxTraceBytes, defaultMaxTraceDuration, time.Now)
	m.startupRecovered, m.startupCleanupErr = cleanupPartialArtifacts(dir)
	return m
}

func newManagerWithLimits(dir string, maxBytes int64, maxDuration time.Duration, now func() time.Time) *Manager {
	return &Manager{dir: dir, maxBytes: maxBytes, maxDuration: maxDuration, now: now}
}

func (m *Manager) Start(tabID int) (StartedTrace, error) {
	started, _, err := m.StartReplacing(tabID, false)
	return started, err
}

// StartReplacing atomically removes an active partial artifact when a newly
// started MV3 worker explicitly reports that it has no matching controller.
func (m *Manager) StartReplacing(tabID int, replaceActive bool) (StartedTrace, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if tabID <= 0 {
		return StartedTrace{}, false, errors.New("tab_id must be a positive integer")
	}
	recovered := false
	if m.startupCleanupErr != nil {
		return StartedTrace{}, false, fmt.Errorf("clean stale performance traces: %w", m.startupCleanupErr)
	}
	recovered = m.startupRecovered
	m.startupRecovered = false
	if m.active != nil {
		if !replaceActive {
			return StartedTrace{}, false, ErrTraceActive
		}
		if err := m.abortLocked(); err != nil {
			return StartedTrace{}, false, fmt.Errorf("recover active performance trace: %w", err)
		}
		recovered = true
	}
	if err := os.MkdirAll(m.dir, 0o700); err != nil {
		return StartedTrace{}, false, fmt.Errorf("create performance trace directory: %w", err)
	}
	id, err := randomID()
	if err != nil {
		return StartedTrace{}, false, err
	}
	base := fmt.Sprintf("cpu-%s-%s", time.Now().UTC().Format("20060102T150405Z"), id)
	partial := filepath.Join(m.dir, base+".json.partial")
	final := filepath.Join(m.dir, base+".json")
	f, err := os.OpenFile(partial, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return StartedTrace{}, false, fmt.Errorf("create performance trace artifact: %w", err)
	}
	if _, err := f.WriteString(`{"traceEvents":[`); err != nil {
		_ = f.Close()
		_ = os.Remove(partial)
		return StartedTrace{}, false, fmt.Errorf("initialize performance trace artifact: %w", err)
	}
	m.active = &activeTrace{id: id, partialPath: partial, finalPath: final, file: f, firstEvent: true, bytes: int64(len(`{"traceEvents":[`)), startedAt: m.now()}
	return StartedTrace{TraceID: id, PartialPath: partial}, recovered, nil
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
	if m.now().Sub(trace.startedAt) > m.maxDuration {
		return ErrTraceDurationLimit
	}
	additionalBytes := int64(0)
	for _, event := range req.Events {
		if !json.Valid(event) {
			return errors.New("trace chunk contains invalid JSON event")
		}
		additionalBytes += int64(len(event) + 1)
	}
	if trace.bytes+additionalBytes+2 > m.maxBytes {
		return ErrTraceSizeLimit
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

func cleanupPartialArtifacts(dir string) (bool, error) {
	paths, err := filepath.Glob(filepath.Join(dir, "*.json.partial"))
	if err != nil {
		return false, err
	}
	for _, path := range paths {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return false, err
		}
	}
	return len(paths) > 0, nil
}

func (m *Manager) Finish(req WirePerformanceTraceFinishRequest) (WirePerformanceTraceResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	trace, err := m.require(req.TraceID)
	if err != nil {
		return WirePerformanceTraceResult{}, err
	}
	if m.now().Sub(trace.startedAt) > m.maxDuration {
		return WirePerformanceTraceResult{}, ErrTraceDurationLimit
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
	_, err := m.require(traceID)
	if err != nil {
		return err
	}
	return m.abortLocked()
}

func (m *Manager) abortLocked() error {
	trace := m.active
	if trace == nil {
		return errors.New("no performance trace is active")
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
