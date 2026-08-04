// projections.go — Derives Doctor and analytics views from canonical incidents.
package incident

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
	Detail        string
	Fix           string
	LocalDetail   string
	LocalFix      string
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
	return DoctorView{
		Code: current.Code, Subsystem: definition.Subsystem, Stage: definition.Stage,
		Severity: definition.Severity, Retryable: definition.Retryable,
		CorrelationID: current.CorrelationID, Generation: current.Generation,
		State: current.State, Attempts: current.Attempts,
		Detail: definition.DoctorDetail, Fix: definition.DoctorFix,
		LocalDetail: current.LocalEvidence.Detail, LocalFix: current.LocalEvidence.Fix,
	}, true
}
