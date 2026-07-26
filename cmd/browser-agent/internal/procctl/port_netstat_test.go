// Purpose: Tests for platform error diagnostics with netstat.
// Docs: docs/features/feature/mcp-persistent-server/index.md

// platform_errors_netstat_test.go — Tests for parseNetstatPIDs function.
package procctl

import (
	"testing"
)

// ============================================
// parseNetstatPIDs
// ============================================

func TestParseNetstatPIDs_SingleListeningEntry(t *testing.T) {
	t.Parallel()
	output := "  TCP    0.0.0.0:7890           0.0.0.0:0              LISTENING       1234\n"
	pids := parseNetstatPIDs(output, 7890)
	if len(pids) != 1 {
		t.Fatalf("expected 1 PID, got %d", len(pids))
	}
	if pids[0] != 1234 {
		t.Errorf("expected PID 1234, got %d", pids[0])
	}
}

func TestParseNetstatPIDs_MultipleListeningEntries(t *testing.T) {
	t.Parallel()
	output := `  TCP    0.0.0.0:7890           0.0.0.0:0              LISTENING       1234
  TCP    [::]:7890              [::]:0                 LISTENING       5678
`
	pids := parseNetstatPIDs(output, 7890)
	if len(pids) != 2 {
		t.Fatalf("expected 2 PIDs, got %d", len(pids))
	}
	if pids[0] != 1234 {
		t.Errorf("expected first PID 1234, got %d", pids[0])
	}
	if pids[1] != 5678 {
		t.Errorf("expected second PID 5678, got %d", pids[1])
	}
}

func TestParseNetstatPIDs_IgnoresNonListening(t *testing.T) {
	t.Parallel()
	output := `  TCP    0.0.0.0:7890           0.0.0.0:0              LISTENING       1234
  TCP    127.0.0.1:7890        192.168.1.1:54321      ESTABLISHED     5678
  TCP    0.0.0.0:7890          0.0.0.0:0              TIME_WAIT       9999
`
	pids := parseNetstatPIDs(output, 7890)
	if len(pids) != 1 {
		t.Fatalf("expected 1 PID (only LISTENING), got %d", len(pids))
	}
	if pids[0] != 1234 {
		t.Errorf("expected PID 1234, got %d", pids[0])
	}
}

func TestParseNetstatPIDs_IgnoresDifferentPort(t *testing.T) {
	t.Parallel()
	output := `  TCP    0.0.0.0:8080           0.0.0.0:0              LISTENING       1111
  TCP    0.0.0.0:7890           0.0.0.0:0              LISTENING       2222
`
	pids := parseNetstatPIDs(output, 7890)
	if len(pids) != 1 {
		t.Fatalf("expected 1 PID for port 7890, got %d", len(pids))
	}
	if pids[0] != 2222 {
		t.Errorf("expected PID 2222, got %d", pids[0])
	}
}

func TestParseNetstatPIDs_EmptyOutput(t *testing.T) {
	t.Parallel()
	pids := parseNetstatPIDs("", 7890)
	if len(pids) != 0 {
		t.Errorf("expected 0 PIDs for empty output, got %d", len(pids))
	}
}

func TestParseNetstatPIDs_NoMatchingPort(t *testing.T) {
	t.Parallel()
	output := "  TCP    0.0.0.0:8080           0.0.0.0:0              LISTENING       1234\n"
	pids := parseNetstatPIDs(output, 7890)
	if len(pids) != 0 {
		t.Errorf("expected 0 PIDs, got %d", len(pids))
	}
}

func TestParseNetstatPIDs_CaseInsensitiveListening(t *testing.T) {
	t.Parallel()
	output := "  TCP    0.0.0.0:7890           0.0.0.0:0              listening       1234\n"
	pids := parseNetstatPIDs(output, 7890)
	if len(pids) != 1 {
		t.Fatalf("expected 1 PID (case insensitive LISTENING), got %d", len(pids))
	}
	if pids[0] != 1234 {
		t.Errorf("expected PID 1234, got %d", pids[0])
	}
}

func TestParseNetstatPIDs_TooFewFields(t *testing.T) {
	t.Parallel()
	// Line with port and LISTENING but fewer than 5 fields
	output := "  TCP    0.0.0.0:7890   LISTENING\n"
	pids := parseNetstatPIDs(output, 7890)
	if len(pids) != 0 {
		t.Errorf("expected 0 PIDs for line with too few fields, got %d", len(pids))
	}
}

func TestParseNetstatPIDs_NonNumericPID(t *testing.T) {
	t.Parallel()
	output := "  TCP    0.0.0.0:7890           0.0.0.0:0              LISTENING       abc\n"
	pids := parseNetstatPIDs(output, 7890)
	if len(pids) != 0 {
		t.Errorf("expected 0 PIDs for non-numeric PID, got %d", len(pids))
	}
}

func TestParseNetstatPIDs_ZeroPID(t *testing.T) {
	t.Parallel()
	// PID 0 should be excluded (pid > 0 check)
	output := "  TCP    0.0.0.0:7890           0.0.0.0:0              LISTENING       0\n"
	pids := parseNetstatPIDs(output, 7890)
	if len(pids) != 0 {
		t.Errorf("expected 0 PIDs for PID=0, got %d", len(pids))
	}
}

func TestParseNetstatPIDs_WhitespaceLines(t *testing.T) {
	t.Parallel()
	output := "\n  \n  TCP    0.0.0.0:7890           0.0.0.0:0              LISTENING       1234\n\n  \n"
	pids := parseNetstatPIDs(output, 7890)
	if len(pids) != 1 {
		t.Fatalf("expected 1 PID ignoring blank lines, got %d", len(pids))
	}
	if pids[0] != 1234 {
		t.Errorf("expected PID 1234, got %d", pids[0])
	}
}

func TestParseNetstatPIDs_HeaderLines(t *testing.T) {
	t.Parallel()
	output := `Active Connections

  Proto  Local Address          Foreign Address        State           PID
  TCP    0.0.0.0:7890           0.0.0.0:0              LISTENING       4567
`
	pids := parseNetstatPIDs(output, 7890)
	if len(pids) != 1 {
		t.Fatalf("expected 1 PID, got %d", len(pids))
	}
	if pids[0] != 4567 {
		t.Errorf("expected PID 4567, got %d", pids[0])
	}
}

func TestParseNetstatPIDs_NegativePID(t *testing.T) {
	t.Parallel()
	// Negative numbers won't parse to positive int via Atoi returning >0
	output := "  TCP    0.0.0.0:7890           0.0.0.0:0              LISTENING       -1\n"
	pids := parseNetstatPIDs(output, 7890)
	if len(pids) != 0 {
		t.Errorf("expected 0 PIDs for negative PID, got %d", len(pids))
	}
}
