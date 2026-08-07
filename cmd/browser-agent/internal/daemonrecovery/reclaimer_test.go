// reclaimer_test.go — Tests safe port ownership detection and reclaim escalation.

package daemonrecovery

import (
	"errors"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/incident"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/statediag"
)

type recordedEvent struct {
	name   string
	port   int
	fields map[string]any
}

func TestLifecycleDepsAreCompleteByConstruction(t *testing.T) {
	reclaimer := New(Config{
		Version: "test", Recovery: statediag.NewCollector(), Incidents: incident.NewStore(4),
		LogLifecycle: func(string, int, map[string]any) {}, Diagnosticf: func(string, ...any) {},
	})
	deps := reclaimer.LifecycleDeps()
	value := reflect.ValueOf(deps)
	for index := 0; index < value.NumField(); index++ {
		field := value.Field(index)
		name := value.Type().Field(index).Name
		switch field.Kind() {
		case reflect.Func, reflect.Interface, reflect.Pointer:
			if field.IsNil() {
				t.Errorf("LifecycleDeps().%s is nil", name)
			}
		case reflect.String:
			if field.String() == "" {
				t.Errorf("LifecycleDeps().%s is empty", name)
			}
		}
	}
}

func testReclaimer(events *[]recordedEvent) *Reclaimer {
	r := New(Config{
		Version: "test",
		LogLifecycle: func(name string, port int, fields map[string]any) {
			*events = append(*events, recordedEvent{name: name, port: port, fields: fields})
		},
		Diagnosticf: func(string, ...any) {},
	})
	r.host.processCommand = func(int) string { return "/usr/local/bin/kaboom-agentic-browser --daemon" }
	return r
}

func TestReclaimPortTerminatesOwnedDaemonAndLogsOutcome(t *testing.T) {
	var events []recordedEvent
	r := testReclaimer(&events)
	self := os.Getpid()
	var graceful, forced []int
	r.host.findProcessOnPort = func(int) ([]int, error) { return []int{self, 4242}, nil }
	r.host.terminatePID = func(pid int, force bool) {
		if force {
			forced = append(forced, pid)
		} else {
			graceful = append(graceful, pid)
		}
	}
	r.host.waitForPortRelease = func(int, time.Duration) bool { return true }
	r.host.isServerRunning = func(int) bool { return false }

	if !r.ReclaimPort(7891, "terminal") {
		t.Fatal("owned daemon port was not reported free")
	}
	if len(graceful) != 1 || graceful[0] != 4242 || len(forced) != 0 {
		t.Fatalf("graceful=%v forced=%v", graceful, forced)
	}
	if got := events[len(events)-1]; got.name != "port_reclaimed" || got.port != 7891 || got.fields["purpose"] != "terminal" {
		t.Fatalf("final event = %#v", got)
	}
}

func TestReclaimPortEscalatesWhenGracefulTerminationDoesNotReleasePort(t *testing.T) {
	var events []recordedEvent
	r := testReclaimer(&events)
	var graceful, forced []int
	r.host.findProcessOnPort = func(int) ([]int, error) { return []int{4242}, nil }
	r.host.terminatePID = func(pid int, force bool) {
		if force {
			forced = append(forced, pid)
		} else {
			graceful = append(graceful, pid)
		}
	}
	r.host.waitForPortRelease = func(int, time.Duration) bool { return false }
	r.host.isServerRunning = func(int) bool { return true }

	if r.ReclaimPort(7890, "main") {
		t.Fatal("occupied port was reported free")
	}
	if len(graceful) != 1 || graceful[0] != 4242 || len(forced) != 1 || forced[0] != 4242 {
		t.Fatalf("graceful=%v forced=%v", graceful, forced)
	}
}

func TestReclaimPortNeverTerminatesForeignOrUnknownProcesses(t *testing.T) {
	for _, command := range []string{"", "/opt/homebrew/bin/postgres -D /var/db"} {
		t.Run(command, func(t *testing.T) {
			var events []recordedEvent
			r := testReclaimer(&events)
			r.host.findProcessOnPort = func(int) ([]int, error) { return []int{4242}, nil }
			r.host.processCommand = func(int) string { return command }
			terminated := false
			r.host.terminatePID = func(int, bool) { terminated = true }
			r.host.isServerRunning = func(int) bool { return true }

			if r.ReclaimPort(7891, "terminal") {
				t.Fatal("foreign-owned port was reported free")
			}
			if terminated {
				t.Fatal("foreign or unknown process was terminated")
			}
			if len(events) != 1 || events[0].name != "port_reclaim_skipped_foreign" {
				t.Fatalf("events = %#v", events)
			}
		})
	}
}

func TestIdentifyPortHolderFiltersInvalidSelfAndFailedLookups(t *testing.T) {
	var events []recordedEvent
	r := testReclaimer(&events)
	r.host.processCommand = func(int) string { return "foreign --pid=4242" }
	r.host.findProcessOnPort = func(int) ([]int, error) { return []int{-1, os.Getpid(), 4242}, nil }
	if pid, command := r.IdentifyPortHolder(7890); pid != 4242 || command != "foreign --pid=4242" {
		t.Fatalf("holder = %d %q", pid, command)
	}
	r.host.findProcessOnPort = func(int) ([]int, error) { return nil, errors.New("lookup failed") }
	if pid, command := r.IdentifyPortHolder(7890); pid != 0 || command != "" {
		t.Fatalf("failed lookup = %d %q", pid, command)
	}
}

func TestProcessLooksLikeOurDaemonUsesExecutableOnly(t *testing.T) {
	for _, command := range []string{
		"/usr/local/bin/kaboom-agentic-browser --daemon",
		"/tmp/go-build/browser-agent.test -test.run=X",
		"/opt/bin/browser-agent.exe --daemon",
	} {
		if !processLooksLikeOurDaemon(command, "/usr/local/bin/kaboom-agentic-browser") {
			t.Errorf("own daemon not recognized: %q", command)
		}
	}
	for _, command := range []string{"", "node server.js --port 7890", "python -m http.server 7890"} {
		if processLooksLikeOurDaemon(command, "/usr/local/bin/kaboom-agentic-browser") {
			t.Errorf("foreign process claimed: %q", command)
		}
	}
}
