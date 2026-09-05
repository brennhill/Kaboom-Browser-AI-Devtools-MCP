// registry_test.go — Proves the instance registry is machine-wide, survives a
// moved state dir, expires by identity rather than pid, and refuses to pollute a
// developer's real registry from a test.

package instancereg_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/instancereg"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/procidentity"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/state"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/testsync"
)

func withRegistry(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv(instancereg.DirEnv, dir)
	return dir
}

// The registry is the ONE thing isolation must not isolate. If --parallel or
// --state-dir moved it, every instance would be invisible to every other and the
// machine-wide cap could never be enforced -- which is the original defect.
func TestDirIgnoresStateDirOverride(t *testing.T) {
	fixed := withRegistry(t)
	t.Setenv(state.StateDirEnv, filepath.Join(t.TempDir(), "isolated-run"))

	got, err := instancereg.Dir()
	if err != nil {
		t.Fatalf("Dir() error = %v", err)
	}
	if got != fixed {
		t.Fatalf("Dir() = %q, want %q (state dir override must not move the registry)", got, fixed)
	}
}

// Guards the 750MB defect found on this machine: ProjectDir mirrored each Go
// t.TempDir() CWD into the real ~/.kaboom, leaving 166,824 directories behind.
// A test that forgets to point the registry at a temp dir must fail loudly, not
// silently write into the developer's home.
func TestDirRefusesRealHomeUnderTest(t *testing.T) {
	t.Setenv(instancereg.DirEnv, "")
	if _, err := instancereg.Dir(); err == nil {
		t.Fatal("Dir() returned the real home registry from a test; it must refuse")
	}
}

func TestRegisterListDeregisterRoundTrip(t *testing.T) {
	withRegistry(t)

	handle, err := instancereg.Register(instancereg.Record{
		Role: instancereg.RoleDaemon, Ports: []int{7890, 7891}, Version: "0.9.0",
	})
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	records, err := instancereg.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("List() returned %d records, want 1", len(records))
	}
	got := records[0]
	if got.PID != os.Getpid() {
		t.Errorf("PID = %d, want %d", got.PID, os.Getpid())
	}
	if got.Role != instancereg.RoleDaemon {
		t.Errorf("Role = %q, want daemon", got.Role)
	}
	if got.Identity.Start == "" || got.Identity.Command == "" {
		t.Errorf("Register() did not stamp process identity: %+v", got.Identity)
	}
	if got.StartedAt == "" || got.HeartbeatAt == "" {
		t.Error("Register() did not stamp timestamps")
	}

	if err := handle.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	records, err = instancereg.List()
	if err != nil {
		t.Fatalf("List() after Close() error = %v", err)
	}
	if len(records) != 0 {
		t.Fatalf("Close() left %d records behind, want 0", len(records))
	}
}

func TestHeartbeatAdvancesOnlyHeartbeatAt(t *testing.T) {
	withRegistry(t)
	handle, err := instancereg.Register(instancereg.Record{Role: instancereg.RoleDaemon, Ports: []int{7890}})
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	t.Cleanup(func() { _ = handle.Close() })

	before, _ := instancereg.List()

	// HeartbeatAt is RFC3339Nano, so on a fine-grained clock one Heartbeat advances it
	// immediately. On a coarse clock (Windows ticks at ~15ms) two calls can land in the
	// same tick, so this retries until the value moves instead of sleeping a guess.
	var after []instancereg.Record
	testsync.Eventually(t, 5*time.Second, "Heartbeat() to advance HeartbeatAt", func() bool {
		if err := handle.Heartbeat(); err != nil {
			t.Fatalf("Heartbeat() error = %v", err)
		}
		after, _ = instancereg.List()
		return after[0].HeartbeatAt != before[0].HeartbeatAt
	})
	if after[0].StartedAt != before[0].StartedAt {
		t.Error("Heartbeat() moved StartedAt; it must record the original start")
	}
}

// A dead holder's record must not survive, and a RECYCLED pid must not keep it
// alive: pid-only liveness is what let 8-Aug locks claim Adobe processes.
func TestPruneRemovesDeadAndRecycledRecords(t *testing.T) {
	dir := withRegistry(t)
	self, ok := procidentity.Self()
	if !ok {
		t.Skip("no process identity available")
	}

	write := func(name string, rec instancereg.Record) {
		t.Helper()
		if err := instancereg.WriteRecordForTest(filepath.Join(dir, name), rec); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	now := time.Now()
	stamp := now.UTC().Format(time.RFC3339Nano)

	// Alive: this process, correct identity.
	write("live.json", instancereg.Record{
		PID: os.Getpid(), Role: instancereg.RoleDaemon, Identity: self,
		StartedAt: stamp, HeartbeatAt: stamp,
	})
	// Dead: a pid that cannot exist.
	write("dead.json", instancereg.Record{
		PID: 0x7FFFFFF0, Role: instancereg.RoleDaemon,
		Identity:  procidentity.Info{Start: "Thu Aug 27 18:58:54 2026", Command: "kaboom"},
		StartedAt: stamp, HeartbeatAt: stamp,
	})
	// Recycled: this pid IS running, but as a different process than recorded.
	write("recycled.json", instancereg.Record{
		PID: os.Getpid(), Role: instancereg.RoleDaemon,
		Identity:  procidentity.Info{Start: "Fri Aug  8 22:37:52 2026", Command: "kaboom-agentic-browser-090"},
		StartedAt: stamp, HeartbeatAt: stamp,
	})

	removed, err := instancereg.Prune(now, time.Minute)
	if err != nil {
		t.Fatalf("Prune() error = %v", err)
	}
	if removed != 2 {
		t.Fatalf("Prune() removed %d records, want 2 (dead + recycled)", removed)
	}
	records, _ := instancereg.List()
	if len(records) != 1 || records[0].PID != os.Getpid() {
		t.Fatalf("Prune() left %+v, want only the live record", records)
	}
}

// A live process whose heartbeat has stopped is WEDGED, not garbage: it still
// holds its ports, so Prune must leave it alone and let the reaper terminate it.
// Forgetting the record instead would let a replacement start and collide.
// Classifying it is instancegov.IsWedged's job, tested there.
func TestPruneKeepsALiveButWedgedRecord(t *testing.T) {
	dir := withRegistry(t)
	self, ok := procidentity.Self()
	if !ok {
		t.Skip("no process identity available")
	}
	now := time.Now()
	old := now.Add(-10 * time.Minute).UTC().Format(time.RFC3339Nano)
	if err := instancereg.WriteRecordForTest(filepath.Join(dir, "wedged.json"), instancereg.Record{
		PID: os.Getpid(), Role: instancereg.RoleDaemon, Identity: self,
		StartedAt: old, HeartbeatAt: old, Ports: []int{7890},
	}); err != nil {
		t.Fatal(err)
	}

	removed, err := instancereg.Prune(now, time.Minute)
	if err != nil {
		t.Fatalf("Prune() error = %v", err)
	}
	if removed != 0 {
		t.Fatalf("Prune() removed %d live-but-wedged records, want 0", removed)
	}
}

func TestListIgnoresUnparseableRecords(t *testing.T) {
	dir := withRegistry(t)
	if err := os.WriteFile(filepath.Join(dir, "junk.json"), []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	records, err := instancereg.List()
	if err != nil {
		t.Fatalf("List() error = %v, want a corrupt record to be skipped", err)
	}
	if len(records) != 0 {
		t.Fatalf("List() = %+v, want no records", records)
	}
}
