// store.go — Persists the bounded redacted Doctor incident timeline.

package statediag

import (
	"encoding/json"
	"errors"
	"os"
	"sync"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/statefile"
)

const timelineStoreVersion = 1

type timelineDocument struct {
	Version          int          `json:"version"`
	DroppedRecovered int          `json:"dropped_recovered"`
	Diagnostics      []Diagnostic `json:"diagnostics"`
}

// PersistentCollector composes the in-memory incident owner with one durable store.
type PersistentCollector struct {
	mu                sync.Mutex
	collector         *Collector
	path              string
	writeFile         func(string, []byte, os.FileMode) error
	persistenceFailed bool
}

// NewPersistentCollector restores a previously persisted bounded incident timeline.
func NewPersistentCollector(path string) (*PersistentCollector, error) {
	owner := &PersistentCollector{collector: NewCollector(), path: path, writeFile: statefile.Write}
	data, err := os.ReadFile(path) // #nosec G304 -- path comes from the canonical state root owner.
	if errors.Is(err, os.ErrNotExist) {
		return owner, nil
	}
	if err != nil {
		return owner, err
	}
	var document timelineDocument
	if json.Unmarshal(data, &document) != nil || document.Version != timelineStoreVersion ||
		len(document.Diagnostics) > maxRecoveredDiagnostics+1000 {
		return owner, errors.New("doctor_timeline_state_invalid")
	}
	for _, diagnostic := range document.Diagnostics {
		if diagnostic.Name == "" || (diagnostic.Lifecycle != LifecycleActive && diagnostic.Lifecycle != LifecycleRecovered) ||
			len(diagnostic.History) > maxHistoryTransitions {
			return owner, errors.New("doctor_timeline_state_invalid")
		}
		diagnostic.Name = compactSafe(diagnostic.Name)
		diagnostic.CorrelationID = compactSafe(diagnostic.CorrelationID)
		diagnostic.Detail = compactSafe(diagnostic.Detail)
		diagnostic.Fix = compactSafe(diagnostic.Fix)
		owner.collector.items[diagnostic.Name] = diagnostic
	}
	owner.collector.droppedRecovered = document.DroppedRecovered
	return owner, nil
}

func (p *PersistentCollector) Report(diagnostic Diagnostic) {
	if p == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.collector.Report(diagnostic)
	p.persistOrReport()
}

func (p *PersistentCollector) Resolve(name string) {
	if p == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.collector.Resolve(name)
	p.persistOrReport()
}

func (p *PersistentCollector) Snapshot() []Diagnostic {
	if p == nil {
		return nil
	}
	return p.collector.Snapshot()
}

func (p *PersistentCollector) Stats() CollectorStats {
	if p == nil {
		return CollectorStats{}
	}
	return p.collector.Stats()
}

func (p *PersistentCollector) persistOrReport() {
	err := p.writeSnapshot()
	if err != nil {
		if !p.persistenceFailed {
			p.collector.Report(Diagnostic{
				Name: "doctor_timeline_persistence", Detail: "Doctor incident history could not be persisted.",
				Fix: "Check the Kaboom state directory permissions, then rerun Doctor.",
			})
		}
		p.persistenceFailed = true
		return
	}
	if p.persistenceFailed {
		p.collector.Resolve("doctor_timeline_persistence")
		if p.writeSnapshot() == nil {
			p.persistenceFailed = false
		}
	}
}

func (p *PersistentCollector) writeSnapshot() error {
	document := timelineDocument{
		Version: timelineStoreVersion, DroppedRecovered: p.collector.Stats().DroppedRecovered,
		Diagnostics: p.collector.Snapshot(),
	}
	data, err := json.Marshal(document)
	if err != nil {
		return err
	}
	return p.writeFile(p.path, data, 0o600)
}
