// helpers_test.go — shared fakes for daemonlife tests: a recording Logger and a
// Deps whose every seam is stubbed, so no test touches a real process or port.

package daemonlife

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/statediag"
)

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

// newTestDeps returns a Deps with every seam stubbed to an inert default plus the
// recording logger behind it. Tests override only the seams they care about.
func newTestDeps(t *testing.T) (Deps, *recordingLogger) {
	t.Helper()
	log := &recordingLogger{}
	return Deps{
		Log:                log,
		Version:            "0.0.0",
		Warnf:              func(string, ...any) {},
		IsProcessAlive:     func(int) bool { return true },
		IsServerRunning:    func(int) bool { return false },
		TryShutdown:        func(int) bool { return false },
		WaitForPortRelease: func(int, time.Duration) bool { return true },
		TerminatePID:       func(int, bool) {},
		FetchHealth: func(context.Context, int, time.Duration) (bool, string, bool) {
			return false, "", false
		},
		ReadPIDFile:   func(int) int { return 0 },
		RemovePIDFile: func(int) {},
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
