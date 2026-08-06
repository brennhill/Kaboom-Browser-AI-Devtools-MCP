// terminal_availability_test.go -- /health must explain WHY the terminal is missing.
//
// A terminal-port bind failure is non-fatal: the daemon serves MCP normally and the
// terminal is simply absent. But setTerminalPort is only called on success, so
// /health omitted terminal_port entirely and said nothing else — the extension got
// "Failed to fetch" on port+1 and configure(what:'health') showed no reason at all.
// Worse, the daemon's stderr explanation goes to /dev/null for a bridge-spawned
// daemon (it is spawned with Stdout/Stderr = nil so it cannot die of SIGPIPE), so
// the ONE place a user or agent can learn what happened is this payload.

package status

import (
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/operationalapi"
)

func terminalHealthPayload(store *Store) map[string]any {
	handler := operationalapi.New(operationalapi.Options{
		TerminalStatus: func() operationalapi.TerminalStatus {
			status := store.Snapshot()
			return operationalapi.TerminalStatus{
				Available:      status.Available,
				Port:           status.Port,
				Error:          status.Error,
				BlockedByPID:   status.BlockedByPID,
				BlockedCommand: status.BlockedByCommand,
			}
		},
	})
	return handler.HealthPayload()
}

func TestTerminalStatus_ReportsBlockingProcess(t *testing.T) {
	store := New()

	store.SetUnavailable(7891, "listen tcp 127.0.0.1:7891: bind: address already in use",
		4242, "/opt/homebrew/opt/postgresql@16/bin/postgres -D /var/pg")

	st := store.Snapshot()
	if st.Available {
		t.Fatal("a failed bind must not report the terminal as available")
	}
	if st.Port != 7891 {
		t.Errorf("port = %d, want 7891 (the port we could not get)", st.Port)
	}
	if st.Error == "" {
		t.Error("the underlying bind error must be preserved, not discarded")
	}
	if st.BlockedByPID != 4242 {
		t.Errorf("blocked_by_pid = %d, want 4242", st.BlockedByPID)
	}
	// Naming the process is the whole point: "port busy" is not actionable,
	// "postgres is on 7891" is.
	if st.BlockedByCommand == "" {
		t.Error("the blocking process's command must be reported so the user knows what to close")
	}
}

func TestTerminalStatus_AvailableAfterSuccessfulBind(t *testing.T) {
	store := New()

	// A failure followed by a successful (re)bind must fully clear the diagnosis —
	// a stale "blocked by postgres" would be worse than saying nothing.
	store.SetUnavailable(7891, "bind: address already in use", 4242, "postgres")
	store.SetPort(7891)

	st := store.Snapshot()
	if !st.Available {
		t.Fatal("a successful bind must report the terminal as available")
	}
	if st.Error != "" || st.BlockedByPID != 0 || st.BlockedByCommand != "" {
		t.Fatalf("a successful bind must clear the previous failure, got %+v", st)
	}
	if st.Port != 7891 {
		t.Errorf("port = %d, want 7891", st.Port)
	}
}

func TestTerminalStatus_SupervisorDeathMarksUnavailable(t *testing.T) {
	store := New()
	store.SetPort(7891)

	// The supervisor reports death by setting the port to 0. That must flip
	// availability too, or /health would keep claiming a terminal that is gone.
	store.SetPort(0)

	if store.Snapshot().Available {
		t.Fatal("setTerminalPort(0) means the terminal server is gone; availability must follow")
	}
}

func TestHealthPayload_ExplainsAnUnavailableTerminal(t *testing.T) {
	store := New()
	store.SetUnavailable(7891, "bind: address already in use", 4242, "postgres -D /var/pg")

	payload := terminalHealthPayload(store)

	avail, ok := payload["terminal_available"].(bool)
	if !ok {
		t.Fatal("health must always carry terminal_available so a caller can branch on it")
	}
	if avail {
		t.Fatal("terminal_available must be false when the bind failed")
	}
	if payload["terminal_error"] == nil || payload["terminal_error"] == "" {
		t.Error("health must carry the reason the terminal is unavailable")
	}
	blocked, ok := payload["terminal_blocked_by"].(map[string]any)
	if !ok {
		t.Fatal("health must name the process holding the port")
	}
	if pid, _ := blocked["pid"].(int); pid != 4242 {
		t.Errorf("terminal_blocked_by.pid = %v, want 4242", blocked["pid"])
	}
	if cmd, _ := blocked["command"].(string); cmd == "" {
		t.Error("terminal_blocked_by.command must be present")
	}
}

func TestHealthPayload_HealthyTerminalCarriesNoDiagnosis(t *testing.T) {
	store := New()
	store.SetPort(7891)

	payload := terminalHealthPayload(store)
	if avail, _ := payload["terminal_available"].(bool); !avail {
		t.Fatal("terminal_available must be true after a successful bind")
	}
	// Noise on the happy path trains people to ignore the field.
	if _, present := payload["terminal_error"]; present {
		t.Error("a healthy terminal must not carry an error field")
	}
	if _, present := payload["terminal_blocked_by"]; present {
		t.Error("a healthy terminal must not claim to be blocked by anything")
	}
	if port, _ := payload["terminal_port"].(int); port != 7891 {
		t.Errorf("terminal_port = %v, want 7891", payload["terminal_port"])
	}
}
