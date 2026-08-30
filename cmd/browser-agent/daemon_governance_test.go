// daemon_governance_test.go — Proves a daemon is only ever considered idle when
// every kind of work is absent, and that any single unknown keeps it alive.

package main

import (
	"testing"
)

func TestBusyProbeReportsBusyForEachKindOfWork(t *testing.T) {
	cases := []struct {
		name    string
		inputs  busyInputs
		wantBusy bool
	}{
		{
			name: "no work at all is idle",
			inputs: busyInputs{
				Clients: func() int { return 0 }, ExtensionConnected: func() bool { return false },
				ActiveRecording: func() bool { return false }, TerminalSessions: func() int { return 0 },
			},
			wantBusy: false,
		},
		{
			name: "a connected MCP client is work",
			inputs: busyInputs{
				Clients: func() int { return 1 }, ExtensionConnected: func() bool { return false },
				ActiveRecording: func() bool { return false }, TerminalSessions: func() int { return 0 },
			},
			wantBusy: true,
		},
		{
			name: "an attached browser extension is work",
			inputs: busyInputs{
				Clients: func() int { return 0 }, ExtensionConnected: func() bool { return true },
				ActiveRecording: func() bool { return false }, TerminalSessions: func() int { return 0 },
			},
			wantBusy: true,
		},
		{
			name: "an in-progress recording must never be interrupted",
			inputs: busyInputs{
				Clients: func() int { return 0 }, ExtensionConnected: func() bool { return false },
				ActiveRecording: func() bool { return true }, TerminalSessions: func() int { return 0 },
			},
			wantBusy: true,
		},
		{
			name: "a live terminal session is work",
			inputs: busyInputs{
				Clients: func() int { return 0 }, ExtensionConnected: func() bool { return false },
				ActiveRecording: func() bool { return false }, TerminalSessions: func() int { return 2 },
			},
			wantBusy: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			busy, reason := busyProbe(tc.inputs)()
			if busy != tc.wantBusy {
				t.Fatalf("busyProbe() = %v (%s), want %v", busy, reason, tc.wantBusy)
			}
			if busy && reason == "" {
				t.Error("busyProbe() reported busy with no reason")
			}
		})
	}
}

// Every probe is optional at the call site, and a missing probe means "cannot
// tell". Treating an unknown as idle would let a daemon shut down mid-recording.
func TestBusyProbeTreatsMissingSignalsAsBusy(t *testing.T) {
	cases := map[string]busyInputs{
		"no client probe":     {ExtensionConnected: func() bool { return false }, ActiveRecording: func() bool { return false }, TerminalSessions: func() int { return 0 }},
		"no extension probe":  {Clients: func() int { return 0 }, ActiveRecording: func() bool { return false }, TerminalSessions: func() int { return 0 }},
		"no recording probe":  {Clients: func() int { return 0 }, ExtensionConnected: func() bool { return false }, TerminalSessions: func() int { return 0 }},
		"no terminal probe":   {Clients: func() int { return 0 }, ExtensionConnected: func() bool { return false }, ActiveRecording: func() bool { return false }},
		"no probes at all":    {},
	}
	for name, inputs := range cases {
		t.Run(name, func(t *testing.T) {
			if busy, _ := busyProbe(inputs)(); !busy {
				t.Fatal("busyProbe() reported idle while a signal was unavailable")
			}
		})
	}
}

// A production daemon must have no hard lifetime bound: a developer's daemon that
// is in use should live as long as they need it.
func TestIdleConfigBoundsOnlyParallelDaemons(t *testing.T) {
	production := idleConfigFor(false, nil)
	if production.MaxLifetime != 0 {
		t.Errorf("production MaxLifetime = %v, want 0 (unbounded)", production.MaxLifetime)
	}
	if production.IdleAfter <= 0 {
		t.Error("production daemon has no idle window; it would never be reclaimed")
	}

	parallel := idleConfigFor(true, nil)
	if parallel.MaxLifetime <= 0 {
		t.Error("parallel daemon has no maximum lifetime; a dead test run would leak it")
	}
	if parallel.IdleAfter >= production.IdleAfter {
		t.Errorf("parallel IdleAfter = %v, want shorter than production's %v",
			parallel.IdleAfter, production.IdleAfter)
	}
}
