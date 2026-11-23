// fault.go — Canonical deterministic fault fixtures for persisted-state boundaries.
package statefault

import (
	"context"
	"errors"
	"fmt"
	"math"
)

// Kind identifies one stable persisted-state failure transition.
type Kind string

const (
	Read          Kind = "read"
	Write         Kind = "write"
	Sync          Kind = "sync"
	Rename        Kind = "rename"
	DirectorySync Kind = "directory_sync"
	Quota         Kind = "quota"
	Corruption    Kind = "corruption"
	PartialWrite  Kind = "partial_write"
	Cancellation  Kind = "cancellation"
	Restart       Kind = "restart"
)

var canonicalKinds = [...]Kind{Read, Write, Sync, Rename, DirectorySync, Quota, Corruption, PartialWrite, Cancellation, Restart}

// ErrInjected classifies a deterministic test fault without retaining private state.
var ErrInjected = errors.New("persisted_state_fault_injected")

// Scenario is an immutable fault fixture. It deliberately does not retain the
// private sentinel supplied by a state-owner test.
type Scenario struct {
	kind Kind
}

// New creates a deterministic scenario and discards the sentinel immediately.
func New(kind Kind, privateSentinel string) Scenario {
	_ = privateSentinel
	return Scenario{kind: kind}
}

// Kinds returns an independent stable copy of the complete fault vocabulary.
func Kinds() []Kind {
	return append([]Kind(nil), canonicalKinds[:]...)
}

// Valid reports whether kind belongs to the canonical vocabulary.
func Valid(kind Kind) bool {
	for _, candidate := range canonicalKinds {
		if candidate == kind {
			return true
		}
	}
	return false
}

// Error returns a stable, redacted injected error.
func (scenario Scenario) Error() error {
	if !Valid(scenario.kind) {
		return fmt.Errorf("%w:unknown", ErrInjected)
	}
	return fmt.Errorf("%w:%s", ErrInjected, scenario.kind)
}

// Payload returns deterministic bytes for corruption and partial-write tests,
// or an independent copy of valid for operation-only faults.
func (scenario Scenario) Payload(valid []byte) []byte {
	switch scenario.kind {
	case Corruption:
		return []byte(`{"schema_version":`)
	case PartialWrite:
		return append([]byte(nil), valid[:len(valid)/2]...)
	default:
		return append([]byte(nil), valid...)
	}
}

// Context returns an already-cancelled child only for Cancellation.
func (scenario Scenario) Context(parent context.Context) context.Context {
	if scenario.kind != Cancellation {
		return parent
	}
	ctx, cancel := context.WithCancel(parent)
	cancel()
	return ctx
}

// NextGeneration models a restart without allowing generation wraparound.
func (scenario Scenario) NextGeneration(current uint64) uint64 {
	if scenario.kind != Restart || current == math.MaxUint64 {
		return current
	}
	return current + 1
}
