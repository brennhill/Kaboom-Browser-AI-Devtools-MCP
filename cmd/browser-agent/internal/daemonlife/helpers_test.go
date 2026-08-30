// helpers_test.go — shared fakes for daemonlife tests: a recording Logger and a
// Deps whose every seam is stubbed, so no test touches a real process or port.

package daemonlife

import (
	"os"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/statediag"
)

func TestSignalSourceLabelsShutdownSignals(t *testing.T) {
	for signal, want := range map[os.Signal]string{
		os.Interrupt: "Ctrl+C", syscall.SIGTERM: "SIGTERM", syscall.SIGHUP: "SIGHUP", syscall.Signal(99): "signal 99",
	} {
		if got := SignalSource(signal); !strings.Contains(got, want) {
			t.Errorf("SignalSource(%v) = %q, want %q", signal, got, want)
		}
	}
}

// loggedEvent is one captured lifecycle event.
type loggedEvent struct {
	Event  string
	Port   int
	Fields map[string]any
}

// recordingLogger captures lifecycle events in memory.
type recordingLogger struct {
	mu     sync.Mutex
	events []loggedEvent
}

func (l *recordingLogger) LogLifecycle(event string, port int, fields map[string]any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.events = append(l.events, loggedEvent{Event: event, Port: port, Fields: fields})
}

// find returns the first captured event with the given name, or nil.
func (l *recordingLogger) find(event string) *loggedEvent {
	l.mu.Lock()
	defer l.mu.Unlock()
	for i := range l.events {
		if l.events[i].Event == event {
			return &l.events[i]
		}
	}
	return nil
}

// newTestDeps returns a Deps with every seam stubbed plus the recording logger
// behind it. It lost eight process and port stubs when singleton admission moved
// to internal/instancegov: this package no longer signals, probes, or binds
// anything, so a fake that still offered those would advertise reach it lacks.
func newTestDeps(t *testing.T) (Deps, *recordingLogger) {
	t.Helper()
	log := &recordingLogger{}
	return Deps{
		Log:     log,
		Version: "0.0.0",
		Warnf:   func(string, ...any) {},
	}, log
}

// freezeClock pins daemonNow/daemonSleep for a test and restores them afterwards.
// Returns a pointer the test can move to advance the frozen clock.
func freezeClock(t *testing.T, at time.Time) *time.Time {
	t.Helper()
	oldNow, oldSleep := daemonNow, daemonSleep
	t.Cleanup(func() { daemonNow, daemonSleep = oldNow, oldSleep })
	now := at
	daemonNow = func() time.Time { return now }
	daemonSleep = func(time.Duration) {} // never really sleep in tests
	return &now
}

// stubInstallEpoch pins this package's install epoch for a test.
func stubInstallEpoch(t *testing.T, epoch int64) {
	t.Helper()
	old := daemonInstallEpoch
	t.Cleanup(func() { daemonInstallEpoch = old })
	daemonInstallEpoch = func(statediag.Reporter) int64 { return epoch }
}
