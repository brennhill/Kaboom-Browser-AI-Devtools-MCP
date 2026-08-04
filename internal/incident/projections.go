// projections.go — Derives Doctor and analytics views from canonical incidents.
package incident

import (
	"sort"
	"time"
)

// DoctorView combines registry-owned presentation with local-only incident
// identity and evidence. It is never an outbound telemetry payload.
type DoctorView struct {
	Code          Code
	Subsystem     Subsystem
	Stage         Stage
	Severity      Severity
	Retryable     bool
	CorrelationID string
	Generation    uint64
	State         State
	Attempts      uint
	DetectedAt    time.Time
	UpdatedAt     time.Time
	ResolvedAt    time.Time
	History       []Transition
	Detail        string
	Fix           string
	LocalDetail   string
}

func (s *Store) Doctor(key string) (DoctorView, bool) {
	s.mu.RLock()
	current, ok := s.items[key]
	s.mu.RUnlock()
	if !ok {
		return DoctorView{}, false
	}
	definition, ok := Lookup(current.Code)
	if !ok {
		return DoctorView{}, false
	}
	return doctorProjection(current, definition), true
}

func (s *Store) DoctorSnapshot() []DoctorView {
	s.mu.RLock()
	type keyedView struct {
		key  string
		view DoctorView
	}
	keyed := make([]keyedView, 0, len(s.items))
	for key, current := range s.items {
		definition, ok := Lookup(current.Code)
		if !ok {
			continue
		}
		keyed = append(keyed, keyedView{key: key, view: doctorProjection(current, definition)})
	}
	s.mu.RUnlock()
	sort.Slice(keyed, func(i, j int) bool { return keyed[i].key < keyed[j].key })
	views := make([]DoctorView, len(keyed))
	for index := range keyed {
		views[index] = keyed[index].view
	}
	return views
}

func doctorProjection(current Incident, definition Definition) DoctorView {
	return DoctorView{
		Code: current.Code, Subsystem: definition.Subsystem, Stage: definition.Stage,
		Severity: definition.Severity, Retryable: definition.Retryable,
		CorrelationID: current.CorrelationID, Generation: current.Generation,
		State: current.State, Attempts: current.Attempts,
		DetectedAt: current.DetectedAt, UpdatedAt: current.UpdatedAt, ResolvedAt: current.ResolvedAt,
		History: append([]Transition(nil), current.History...),
		Detail:  definition.DoctorDetail, Fix: definition.DoctorFix,
		LocalDetail: current.LocalEvidence.Detail,
	}
}
