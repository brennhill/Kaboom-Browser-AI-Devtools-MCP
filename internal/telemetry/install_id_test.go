// install_id_test.go — Tests for random install ID generation and persistence.
// Tests in this package must NOT use t.Parallel() due to shared package-level state.

package telemetry

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/incident"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/statefault"
)

func TestInstallIdentityIncidentTelemetryCannotDeadlockWarmup(t *testing.T) {
	resetInstallIDState()
	overrideKaboomDir(t.TempDir())
	t.Cleanup(resetKaboomDir)
	installIdentityFiles = faultIdentityFilesystem{readErr: fs.ErrPermission}
	t.Cleanup(func() { installIdentityFiles = localIdentityFilesystem{} })
	diagnostics := incident.NewStore(10, QueueReliability)
	done := make(chan struct{})
	go func() {
		Warm(diagnostics)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("install identity warmup deadlocked while publishing its incident")
	}
	views := diagnostics.DoctorSnapshot()
	if len(views) != 1 || views[0].Code != incident.CodeStateRecoveryFailed || views[0].CorrelationID != "install_identity" {
		t.Fatalf("warmup diagnostics = %#v, want retained install identity incident", views)
	}
}

var hexPattern = regexp.MustCompile(`^[0-9a-f]{12}$`)

type faultIdentityFilesystem struct {
	readErr  error
	writeErr error
}

type markerFaultFilesystem struct {
	localIdentityFilesystem
	writeErr error
}

func (f markerFaultFilesystem) WriteFile(string, []byte) error { return f.writeErr }

func (f faultIdentityFilesystem) ReadFile(string) ([]byte, error) {
	if f.readErr != nil {
		return nil, f.readErr
	}
	return nil, fs.ErrNotExist
}
func (f faultIdentityFilesystem) CreateExclusive(string, string, []byte) error { return f.writeErr }
func (f faultIdentityFilesystem) Replace(string, string, []byte) error         { return f.writeErr }
func (f faultIdentityFilesystem) WriteFile(string, []byte) error               { return f.writeErr }

func TestInstallIdentityCanonicalFaultsSuppressUnstableTelemetry(t *testing.T) {
	private := "private-install-state"
	for _, kind := range []statefault.Kind{statefault.Read, statefault.Write, statefault.Quota, statefault.Cancellation} {
		t.Run(string(kind), func(t *testing.T) {
			resetInstallIDState()
			overrideKaboomDir(t.TempDir())
			t.Cleanup(resetKaboomDir)
			diagnostics := incident.NewStore(10)
			stateRecovery = diagnostics
			scenario := statefault.New(kind, private)
			if kind == statefault.Read {
				installIdentityFiles = faultIdentityFilesystem{readErr: scenario.Error()}
			} else {
				installIdentityFiles = faultIdentityFilesystem{writeErr: scenario.Error()}
			}
			t.Cleanup(func() { installIdentityFiles = localIdentityFilesystem{} })

			if id := GetInstallID(); id != "" {
				t.Fatalf("GetInstallID() = %q, want telemetry suppression", id)
			}
			got := diagnostics.DoctorSnapshot()
			if len(got) != 1 || got[0].Code != incident.CodeStateRecoveryFailed {
				t.Fatalf("diagnostics = %#v, want redacted identity incident", got)
			}
			if strings.Contains(got[0].LocalDetail, private) {
				t.Fatal("diagnostic leaked private state")
			}
		})
	}
}

func TestInstallIdentityCorruptionRecoveryRemainsStableAcrossRestart(t *testing.T) {
	for _, kind := range []statefault.Kind{statefault.Corruption, statefault.PartialWrite} {
		t.Run(string(kind), func(t *testing.T) {
			dir := t.TempDir()
			overrideKaboomDir(dir)
			t.Cleanup(resetKaboomDir)
			scenario := statefault.New(kind, "private-install-state")
			if err := os.WriteFile(filepath.Join(dir, "install_id"), scenario.Payload([]byte("aabbccddeeff")), 0o600); err != nil {
				t.Fatal(err)
			}

			resetInstallIDState()
			first := GetInstallID()
			if !hexPattern.MatchString(first) {
				t.Fatalf("replacement ID = %q", first)
			}
			resetInstallIDState()
			if second := GetInstallID(); second != first {
				t.Fatalf("identity rotated after restart: first=%q second=%q", first, second)
			}
		})
	}
}

func TestFirstToolCallMarkerWriteFaultSuppressesEvent(t *testing.T) {
	dir := t.TempDir()
	overrideKaboomDir(dir)
	t.Cleanup(resetKaboomDir)
	resetInstallIDState()
	resetFirstToolCallState()
	diagnostics := incident.NewStore(10)
	stateRecovery = diagnostics
	if id := GetInstallID(); id == "" {
		t.Fatal("failed to establish install identity")
	}
	scenario := statefault.New(statefault.Quota, "private-marker-state")
	installIdentityFiles = markerFaultFilesystem{writeErr: scenario.Error()}

	if markFirstToolCallEmittedForInstall() {
		t.Fatal("first-tool-call telemetry must be suppressed when its durable marker fails")
	}
	if got := diagnostics.DoctorSnapshot(); len(got) != 1 || got[0].Code != incident.CodeStateRecoveryFailed {
		t.Fatalf("diagnostics = %#v, want marker persistence incident", got)
	}
}

func TestGetInstallID_GeneratesOnFirstCall(t *testing.T) {
	dir := t.TempDir()
	resetInstallIDState()
	overrideKaboomDir(dir)
	defer resetKaboomDir()

	id := GetInstallID()
	if !hexPattern.MatchString(id) {
		t.Fatalf("GetInstallID() = %q, want 12-char hex string", id)
	}
}

func TestGetInstallID_PersistsAcrossCalls(t *testing.T) {
	dir := t.TempDir()
	resetInstallIDState()
	overrideKaboomDir(dir)
	defer resetKaboomDir()

	id1 := GetInstallID()
	id2 := GetInstallID()
	if id1 != id2 {
		t.Fatalf("GetInstallID() returned different values: %q vs %q", id1, id2)
	}
}

func TestGetInstallID_ReadsFromFile(t *testing.T) {
	dir := t.TempDir()
	resetInstallIDState()
	overrideKaboomDir(dir)
	defer resetKaboomDir()

	// Pre-write a known ID file.
	knownID := "aabbccddeeff"
	if err := os.WriteFile(filepath.Join(dir, "install_id"), []byte(knownID), 0600); err != nil {
		t.Fatalf("failed to write test install_id: %v", err)
	}

	id := GetInstallID()
	if id != knownID {
		t.Fatalf("GetInstallID() = %q, want %q (from file)", id, knownID)
	}
}

func TestGetInstallID_CreatesDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", ".strum")
	resetInstallIDState()
	overrideKaboomDir(dir)
	defer resetKaboomDir()

	id := GetInstallID()
	if !hexPattern.MatchString(id) {
		t.Fatalf("GetInstallID() = %q, want 12-char hex string", id)
	}

	// Verify directory was created.
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("strum dir not created: %v", err)
	}
	if !info.IsDir() {
		t.Fatalf("strum dir path is not a directory")
	}

	// Verify file was written.
	data, err := os.ReadFile(filepath.Join(dir, "install_id"))
	if err != nil {
		t.Fatalf("install_id file not written: %v", err)
	}
	if string(data) != id {
		t.Fatalf("file content = %q, want %q", string(data), id)
	}
}

// #7: Install ID file with trailing newline should be trimmed.
func TestGetInstallID_TrimsWhitespace(t *testing.T) {
	dir := t.TempDir()
	resetInstallIDState()
	overrideKaboomDir(dir)
	defer resetKaboomDir()

	// Write ID with trailing newline (common from echo "id" > file).
	if err := os.WriteFile(filepath.Join(dir, "install_id"), []byte("aabbccddeeff\n"), 0600); err != nil {
		t.Fatalf("failed to write test install_id: %v", err)
	}

	id := GetInstallID()
	if id != "aabbccddeeff" {
		t.Fatalf("GetInstallID() = %q, want %q (should trim whitespace)", id, "aabbccddeeff")
	}
}

// #7: Install ID with spaces and carriage return should be trimmed.
func TestGetInstallID_TrimsCarriageReturn(t *testing.T) {
	dir := t.TempDir()
	resetInstallIDState()
	overrideKaboomDir(dir)
	defer resetKaboomDir()

	if err := os.WriteFile(filepath.Join(dir, "install_id"), []byte("  aabbccddeeff\r\n"), 0600); err != nil {
		t.Fatalf("failed to write test install_id: %v", err)
	}

	id := GetInstallID()
	if id != "aabbccddeeff" {
		t.Fatalf("GetInstallID() = %q, want %q", id, "aabbccddeeff")
	}
}

// Installation identity is machine-install scoped, not runtime-state scoped.
// UAT and project-isolated daemons routinely override these state roots; letting
// those overrides rotate iid inflates active-install analytics.
func TestDefaultKaboomDir_IgnoresRuntimeStateOverrides(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("KABOOM_STATE_DIR", filepath.Join(t.TempDir(), "isolated"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(t.TempDir(), "xdg"))
	t.Cleanup(resetKaboomDir)

	resetInstallIDState()
	resetKaboomDir()

	want := filepath.Join(home, ".kaboom")
	if kaboomDir != want {
		t.Fatalf("kaboomDir = %q, want installation root %q", kaboomDir, want)
	}

	id := GetInstallID()
	data, err := os.ReadFile(filepath.Join(want, "install_id"))
	if err != nil {
		t.Fatalf("install_id not written under installation root: %v", err)
	}
	if string(data) != id {
		t.Fatalf("install_id file content = %q, want %q", string(data), id)
	}
}

// Concurrent daemon starts must converge on the one ID that wins file
// creation. A normal overwriting write lets every process cache a different ID.
func TestLoadOrGenerateInstallID_ConcurrentCreatorsConverge(t *testing.T) {
	dir := t.TempDir()
	overrideKaboomDir(dir)
	t.Cleanup(resetKaboomDir)

	const creators = 32
	start := make(chan struct{})
	results := make(chan string, creators)
	var wg sync.WaitGroup
	wg.Add(creators)
	for range creators {
		go func() {
			defer wg.Done()
			<-start
			results <- loadOrGenerateInstallID()
		}()
	}
	close(start)
	wg.Wait()
	close(results)

	var first string
	for id := range results {
		if first == "" {
			first = id
		}
		if id != first {
			t.Fatalf("concurrent creators returned %q and %q", first, id)
		}
	}
}

func TestGetInstallID_ReadFailure(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("chmod 000 not effective on Windows")
	}

	dir := t.TempDir()
	resetInstallIDState()
	overrideKaboomDir(dir)
	defer resetKaboomDir()
	diagnostics := incident.NewStore(10)
	stateRecovery = diagnostics

	// Create a directory where the install_id file would be, making ReadFile fail.
	idPath := filepath.Join(dir, "install_id")
	if err := os.Mkdir(idPath, 0000); err != nil {
		t.Fatalf("failed to create blocking dir: %v", err)
	}
	defer os.Chmod(idPath, 0700) // cleanup

	id := GetInstallID()
	if id != "" {
		t.Fatalf("GetInstallID() = %q, want telemetry-suppressing empty ID on read failure", id)
	}
	got := diagnostics.DoctorSnapshot()
	if len(got) != 1 || got[0].Code != incident.CodeStateRecoveryFailed || got[0].Fix == "" {
		t.Fatalf("diagnostics = %#v, want actionable install identity warning", got)
	}
}

func TestGetInstallIDReplacesMalformedIdentityOnce(t *testing.T) {
	dir := t.TempDir()
	resetInstallIDState()
	overrideKaboomDir(dir)
	defer resetKaboomDir()
	diagnostics := incident.NewStore(10)
	stateRecovery = diagnostics

	idPath := filepath.Join(dir, "install_id")
	if err := os.WriteFile(idPath, []byte("not-a-stable-id"), 0o600); err != nil {
		t.Fatal(err)
	}
	first := GetInstallID()
	if !hexPattern.MatchString(first) {
		t.Fatalf("replacement ID = %q, want stable hex ID", first)
	}
	resetInstallIDState()
	stateRecovery = diagnostics
	second := GetInstallID()
	if second != first {
		t.Fatalf("replacement rotated across restart: first=%q second=%q", first, second)
	}
	got := diagnostics.DoctorSnapshot()
	if len(got) != 1 || got[0].Code != incident.CodeStateRecoveryFailed {
		t.Fatalf("diagnostics = %#v, want one identity recovery", got)
	}
}

func TestLocalIdentityFilesystemReportsBlockedParentDirectories(t *testing.T) {
	blocked := filepath.Join(t.TempDir(), "blocked")
	if err := os.WriteFile(blocked, []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	files := localIdentityFilesystem{}
	path := filepath.Join(blocked, "install_id")
	if err := files.CreateExclusive(blocked, path, []byte("aabbccddeeff")); err == nil {
		t.Fatal("CreateExclusive accepted blocked parent")
	}
	if err := files.Replace(blocked, path, []byte("aabbccddeeff")); err == nil {
		t.Fatal("Replace accepted blocked parent")
	}
	if err := files.WriteFile(path, []byte("aabbccddeeff")); err == nil {
		t.Fatal("WriteFile accepted blocked parent")
	}
	if validInstallID("zzzzzzzzzzzz") || validInstallID("short") {
		t.Fatal("invalid install identity was accepted")
	}
	previous := stateRecovery
	defer func() { stateRecovery = previous }()
	stateRecovery = nil
	reportInstallIDRecovery("expected absence")
}

func TestGetSessionID_Returns16CharHex(t *testing.T) {
	resetSessionState()
	TouchSession() // mint a session
	id := GetSessionID()
	if !regexp.MustCompile(`^[0-9a-f]{16}$`).MatchString(id) {
		t.Errorf("GetSessionID() = %q, want 16-char hex", id)
	}
}

func TestGetSessionID_StableWithinTimeout(t *testing.T) {
	resetSessionState()
	TouchSession()
	id1 := GetSessionID()
	id2 := GetSessionID()
	if id1 != id2 {
		t.Errorf("GetSessionID() returned different IDs within same session: %q vs %q", id1, id2)
	}
}

func TestTouchSession_RotatesAfterInactivity(t *testing.T) {
	resetSessionState()
	TouchSession()
	id1 := GetSessionID()

	// Simulate inactivity beyond timeout.
	session.mu.Lock()
	session.lastSeen = time.Now().Add(-sessionTimeout - time.Second)
	session.mu.Unlock()

	// TouchSession detects expiry and mints a new session.
	TouchSession()
	id2 := GetSessionID()
	if id1 == id2 {
		t.Error("session should have rotated after inactivity timeout")
	}
}

func TestGetSessionID_DoesNotExtendSession(t *testing.T) {
	// GetSessionID must be read-only — it must NOT refresh lastSeen.
	// Only TouchSession (and Increment) should extend the session.
	resetSessionState()
	TouchSession()

	// Record lastSeen before calling GetSessionID.
	session.mu.Lock()
	before := session.lastSeen
	session.mu.Unlock()

	// Use a sentinel value so any accidental refresh is observable without
	// relying on wall-clock separation.
	before = time.Unix(123, 456)
	session.mu.Lock()
	session.lastSeen = before
	session.mu.Unlock()

	_ = GetSessionID()

	session.mu.Lock()
	after := session.lastSeen
	session.mu.Unlock()

	if !after.Equal(before) {
		t.Errorf("GetSessionID modified lastSeen: before=%v, after=%v", before, after)
	}
}

func TestTouchSession_RefreshesLastSeen(t *testing.T) {
	resetSessionState()
	TouchSession()
	id1 := GetSessionID()

	// Backdate to near-expiry.
	session.mu.Lock()
	session.lastSeen = time.Now().Add(-sessionTimeout + time.Second)
	session.mu.Unlock()

	// Touch should refresh without rotating.
	TouchSession()

	id2 := GetSessionID()
	if id1 != id2 {
		t.Error("TouchSession should have prevented rotation")
	}
}

func TestTouchSession_BeforeFirstGetSessionID(t *testing.T) {
	resetSessionState()

	// TouchSession on empty session should mint a new session.
	TouchSession()

	id := GetSessionID()
	if !regexp.MustCompile(`^[0-9a-f]{16}$`).MatchString(id) {
		t.Errorf("GetSessionID() after TouchSession on empty = %q, want 16-char hex", id)
	}
}

func TestGetSessionID_ConcurrentAccess(t *testing.T) {
	resetSessionState()
	TouchSession()

	var wg sync.WaitGroup
	ids := make([]string, 100)
	for i := range ids {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			ids[idx] = GetSessionID()
		}(i)
	}
	wg.Wait()

	// All should return the same ID (session is active).
	for i, id := range ids {
		if id != ids[0] {
			t.Errorf("goroutine %d got %q, goroutine 0 got %q — expected same session", i, id, ids[0])
			break
		}
	}
}

func TestUsageTracker_Increment_TouchesSession(t *testing.T) {
	resetSessionState()
	TouchSession()
	id1 := GetSessionID()

	counter := NewUsageTracker()

	// Backdate to near-expiry.
	session.mu.Lock()
	session.lastSeen = time.Now().Add(-sessionTimeout + time.Second)
	session.mu.Unlock()

	// Increment should touch session, refreshing lastSeen.
	counter.RecordToolCall("observe:errors", 0, false)

	// Verify lastSeen was refreshed (not still near-expiry).
	session.mu.Lock()
	ls := session.lastSeen
	session.mu.Unlock()

	if time.Since(ls) > 5*time.Second {
		t.Errorf("Increment did not touch session: lastSeen is %v ago", time.Since(ls))
	}

	// Session should not have rotated.
	id2 := GetSessionID()
	if id1 != id2 {
		t.Error("Increment should have touched session, preventing rotation")
	}
}
