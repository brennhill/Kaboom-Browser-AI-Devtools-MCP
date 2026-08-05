// manager_test.go — Tests for PTY session manager lifecycle, auth tokens, and concurrent access.

package pty

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestPackageFileBoundary(t *testing.T) {
	t.Parallel()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read PTY package: %v", err)
	}
	count := 0
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".go" {
			count++
		}
	}
	if count > 10 {
		t.Fatalf("PTY package has %d Go files; maximum is 10", count)
	}
}

func TestManager_StartAndGet(t *testing.T) {
	m := NewManager()
	defer m.StopAll()

	result, err := m.Start(StartConfig{
		Cmd:  "/bin/sh",
		Args: []string{"-c", "exec cat"},
	})
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	if result.SessionID != "default" {
		t.Fatalf("expected session ID 'default', got: %s", result.SessionID)
	}
	if result.Token == "" {
		t.Fatal("expected non-empty token")
	}
	if result.Pid <= 0 {
		t.Fatalf("expected positive PID, got: %d", result.Pid)
	}

	// Get by token.
	sess, err := m.GetByToken(result.Token)
	if err != nil {
		t.Fatalf("get by token: %v", err)
	}
	if sess.ID != "default" {
		t.Fatalf("expected session ID 'default', got: %s", sess.ID)
	}

	// Get by ID.
	sess2, err := m.Get("default")
	if err != nil {
		t.Fatalf("get by id: %v", err)
	}
	if sess2 != sess {
		t.Fatal("expected same session instance")
	}
}

func TestManager_DuplicateSessionID(t *testing.T) {
	m := NewManager()
	defer m.StopAll()

	_, err := m.Start(StartConfig{
		ID:   "test",
		Cmd:  "/bin/sh",
		Args: []string{"-c", "exec cat"},
	})
	if err != nil {
		t.Fatalf("first start: %v", err)
	}

	_, err = m.Start(StartConfig{
		ID:   "test",
		Cmd:  "/bin/sh",
		Args: []string{"-c", "exec cat"},
	})
	if !errors.Is(err, ErrSessionExists) {
		t.Fatalf("expected ErrSessionExists, got: %v", err)
	}
}

func TestManager_InvalidToken(t *testing.T) {
	m := NewManager()

	_, err := m.GetByToken("bogus")
	if !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("expected ErrInvalidToken, got: %v", err)
	}
}

func TestManager_StopSession(t *testing.T) {
	m := NewManager()

	result, err := m.Start(StartConfig{
		Cmd:  "/bin/sh",
		Args: []string{"-c", "exec cat"},
	})
	if err != nil {
		t.Fatalf("start: %v", err)
	}

	if err := m.Stop("default"); err != nil {
		t.Fatalf("stop: %v", err)
	}

	// Token should be invalidated.
	_, err = m.GetByToken(result.Token)
	if !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("expected ErrInvalidToken after stop, got: %v", err)
	}

	// Session should be gone.
	_, err = m.Get("default")
	if !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("expected ErrSessionNotFound after stop, got: %v", err)
	}

	// Stop nonexistent.
	err = m.Stop("nonexistent")
	if !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("expected ErrSessionNotFound, got: %v", err)
	}
}

func TestManager_StopAll(t *testing.T) {
	m := NewManager()

	for _, id := range []string{"a", "b", "c"} {
		_, err := m.Start(StartConfig{
			ID:   id,
			Cmd:  "/bin/sh",
			Args: []string{"-c", "exec cat"},
		})
		if err != nil {
			t.Fatalf("start %s: %v", id, err)
		}
	}

	if m.Count() != 3 {
		t.Fatalf("expected 3 sessions, got: %d", m.Count())
	}

	m.StopAll()

	if m.Count() != 0 {
		t.Fatalf("expected 0 sessions after StopAll, got: %d", m.Count())
	}
}

func TestManager_List(t *testing.T) {
	m := NewManager()
	defer m.StopAll()

	ids := []string{"alpha", "beta"}
	for _, id := range ids {
		_, err := m.Start(StartConfig{
			ID:   id,
			Cmd:  "/bin/sh",
			Args: []string{"-c", "exec cat"},
		})
		if err != nil {
			t.Fatalf("start %s: %v", id, err)
		}
	}

	listed := m.List()
	if len(listed) != 2 {
		t.Fatalf("expected 2, got: %d", len(listed))
	}

	// Check both IDs are present (order is map-dependent).
	found := make(map[string]bool)
	for _, id := range listed {
		found[id] = true
	}
	for _, id := range ids {
		if !found[id] {
			t.Fatalf("missing session ID: %s", id)
		}
	}
}

func TestManager_ConcurrentAccess(t *testing.T) {
	m := NewManager()
	defer m.StopAll()

	// Start a session.
	result, err := m.Start(StartConfig{
		Cmd:  "/bin/sh",
		Args: []string{"-c", "exec cat"},
	})
	if err != nil {
		t.Fatalf("start: %v", err)
	}

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = m.GetByToken(result.Token)
			_ = m.List()
			_ = m.Count()
		}()
	}
	wg.Wait()
}

// --- Session-per-repo-per-agent (improvement 7) ---

func TestManager_GetByRepoAgent(t *testing.T) {
	m := NewManager()
	defer m.StopAll()

	_, err := m.Start(StartConfig{
		ID:        "claude-myrepo",
		Cmd:       "/bin/sh",
		Args:      []string{"-c", "exec cat"},
		RepoPath:  "/home/user/myrepo",
		AgentType: "claude",
	})
	if err != nil {
		t.Fatalf("start claude: %v", err)
	}

	_, err = m.Start(StartConfig{
		ID:        "codex-myrepo",
		Cmd:       "/bin/sh",
		Args:      []string{"-c", "exec cat"},
		RepoPath:  "/home/user/myrepo",
		AgentType: "codex",
	})
	if err != nil {
		t.Fatalf("start codex: %v", err)
	}

	// Get Claude session for myrepo.
	sess, err := m.GetByRepoAgent("/home/user/myrepo", "claude")
	if err != nil {
		t.Fatalf("get claude: %v", err)
	}
	if sess.ID != "claude-myrepo" {
		t.Fatalf("expected claude-myrepo, got %s", sess.ID)
	}

	// Get Codex session for myrepo.
	sess, err = m.GetByRepoAgent("/home/user/myrepo", "codex")
	if err != nil {
		t.Fatalf("get codex: %v", err)
	}
	if sess.ID != "codex-myrepo" {
		t.Fatalf("expected codex-myrepo, got %s", sess.ID)
	}

	// Nonexistent combination.
	_, err = m.GetByRepoAgent("/home/user/other", "claude")
	if !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("expected ErrSessionNotFound, got: %v", err)
	}
}

func TestManager_RepoIndex_CleanedOnStop(t *testing.T) {
	m := NewManager()
	defer m.StopAll()

	_, err := m.Start(StartConfig{
		ID:        "test-sess",
		Cmd:       "/bin/sh",
		Args:      []string{"-c", "exec cat"},
		RepoPath:  "/repo",
		AgentType: "claude",
	})
	if err != nil {
		t.Fatalf("start: %v", err)
	}

	if err := m.Stop("test-sess"); err != nil {
		t.Fatalf("stop: %v", err)
	}

	_, err = m.GetByRepoAgent("/repo", "claude")
	if !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("expected ErrSessionNotFound after stop, got: %v", err)
	}
}

func TestManager_RepoIndex_CleanedOnStopAll(t *testing.T) {
	m := NewManager()

	_, err := m.Start(StartConfig{
		ID:        "test-sess",
		Cmd:       "/bin/sh",
		Args:      []string{"-c", "exec cat"},
		RepoPath:  "/repo",
		AgentType: "claude",
	})
	if err != nil {
		t.Fatalf("start: %v", err)
	}

	m.StopAll()

	_, err = m.GetByRepoAgent("/repo", "claude")
	if !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("expected ErrSessionNotFound after StopAll, got: %v", err)
	}
}

func TestManager_MaxSessionsLimit(t *testing.T) {
	m := NewManager()
	defer m.StopAll()

	// Fill up to the limit.
	for i := 0; i < maxSessions; i++ {
		_, err := m.Start(StartConfig{
			ID:   fmt.Sprintf("sess-%d", i),
			Cmd:  "/bin/sh",
			Args: []string{"-c", "exec cat"},
		})
		if err != nil {
			t.Fatalf("start session %d: %v", i, err)
		}
	}

	if m.Count() != maxSessions {
		t.Fatalf("expected %d sessions, got %d", maxSessions, m.Count())
	}

	// Next session should be rejected.
	_, err := m.Start(StartConfig{
		ID:   "overflow",
		Cmd:  "/bin/sh",
		Args: []string{"-c", "exec cat"},
	})
	if !errors.Is(err, ErrMaxSessions) {
		t.Fatalf("expected ErrMaxSessions, got: %v", err)
	}

	// After stopping one, a new session should be allowed.
	if err := m.Stop("sess-0"); err != nil {
		t.Fatalf("stop: %v", err)
	}
	_, err = m.Start(StartConfig{
		ID:   "replacement",
		Cmd:  "/bin/sh",
		Args: []string{"-c", "exec cat"},
	})
	if err != nil {
		t.Fatalf("start replacement after stop: %v", err)
	}
}

func TestManager_NoRepoIndex_WithoutRepoPath(t *testing.T) {
	m := NewManager()
	defer m.StopAll()

	_, err := m.Start(StartConfig{
		Cmd:  "/bin/sh",
		Args: []string{"-c", "exec cat"},
	})
	if err != nil {
		t.Fatalf("start: %v", err)
	}

	// Without RepoPath, GetByRepoAgent should not find it.
	_, err = m.GetByRepoAgent("", "")
	if !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("expected ErrSessionNotFound, got: %v", err)
	}
}

func fakeSpawn(cfg SpawnConfig) (*Session, error) {
	reaped := make(chan struct{})
	close(reaped)
	return &Session{ID: cfg.ID, cmd: &exec.Cmd{}, done: make(chan struct{}), reaped: reaped}, nil
}

func newFakeManager() *Manager {
	m := NewManager()
	m.spawn = fakeSpawn
	return m
}

// A second Start of an ID whose session is DEAD self-heals: it evicts the corpse
// and spawns fresh with a new token, rather than returning ErrSessionExists (which
// used to strand the terminal on a dead session forever). Fakes are never IsAlive,
// so they stand in for a child that has exited on its own.
func TestManager_StartWithFakeSpawn_SelfHealsDeadSession(t *testing.T) {
	m := newFakeManager()

	res1, err := m.Start(StartConfig{ID: "s1"})
	if err != nil {
		t.Fatalf("first Start: unexpected error %v", err)
	}
	if res1.Replaced {
		t.Fatal("first Start should not report Replaced")
	}

	res2, err := m.Start(StartConfig{ID: "s1"})
	if err != nil {
		t.Fatalf("second Start of a dead session should self-heal, got %v", err)
	}
	if !res2.Replaced {
		t.Fatal("second Start should report Replaced=true")
	}
	if res2.Token == res1.Token {
		t.Fatal("self-heal must mint a fresh token")
	}
	if _, err := m.GetByToken(res1.Token); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("old token must be invalidated after self-heal, got %v", err)
	}
	if _, err := m.GetByToken(res2.Token); err != nil {
		t.Fatalf("new token must resolve, got %v", err)
	}
	if got := m.Count(); got != 1 {
		t.Fatalf("Count()=%d, want 1 (corpse evicted, not accumulated)", got)
	}
}

func TestManager_StartWithFakeSpawn_MaxSessions(t *testing.T) {
	m := newFakeManager()

	for i := 0; i < maxSessions; i++ {
		if _, err := m.Start(StartConfig{ID: string(rune('a' + i))}); err != nil {
			t.Fatalf("Start #%d: unexpected error %v", i, err)
		}
	}
	_, err := m.Start(StartConfig{ID: "overflow"})
	if !errors.Is(err, ErrMaxSessions) {
		t.Fatalf("Start past the limit: got %v, want ErrMaxSessions", err)
	}
}

func TestManager_StartWithFakeSpawn_TokenAndRepoIndex(t *testing.T) {
	m := newFakeManager()

	res, err := m.Start(StartConfig{ID: "s1", RepoPath: "/repo", AgentType: "claude"})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Token -> session.
	sess, err := m.GetByToken(res.Token)
	if err != nil || sess.ID != "s1" {
		t.Fatalf("GetByToken(%q) = (%v, %v), want session s1", res.Token, sess, err)
	}
	if got := m.GetTokenForSession("s1"); got != res.Token {
		t.Fatalf("GetTokenForSession = %q, want %q", got, res.Token)
	}

	// (repo, agent) -> session.
	byRepo, err := m.GetByRepoAgent("/repo", "claude")
	if err != nil || byRepo.ID != "s1" {
		t.Fatalf("GetByRepoAgent = (%v, %v), want session s1", byRepo, err)
	}

	// A bad token is rejected.
	if _, err := m.GetByToken("nope"); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("GetByToken(bad) = %v, want ErrInvalidToken", err)
	}
}

// Start must not hold m.mu while Close()-ing the session it evicts. Session.Close
// blocks for up to ~4s (SIGTERM → 2s → SIGKILL → 2s) when the child is wedged, and
// holding the manager lock across that freezes Get/GetByToken/List — i.e. every
// terminal route — for the whole teardown (finding S3).
//
// The first fake session's `reaped` channel stays open until the test closes it, so
// its Close() parks inside the reap wait. The assertion is that the self-heal Start
// publishes the new token (i.e. released m.mu) long before that wait can finish.
func TestManager_Start_DoesNotHoldLockDuringEvictedClose(t *testing.T) {
	m := NewManager()
	release := make(chan struct{})
	spawns := 0
	m.spawn = func(cfg SpawnConfig) (*Session, error) {
		spawns++
		reaped := release // first session: Close parks until the test releases it
		if spawns > 1 {
			ch := make(chan struct{})
			close(ch)
			reaped = ch
		}
		return &Session{ID: cfg.ID, cmd: &exec.Cmd{}, done: make(chan struct{}), reaped: reaped}, nil
	}
	var releaseOnce sync.Once
	unblockClose := func() { releaseOnce.Do(func() { close(release) }) }
	defer unblockClose() // also runs on t.Fatal, so the parked Close never leaks

	first, err := m.Start(StartConfig{ID: "s1"})
	if err != nil {
		t.Fatalf("first Start: %v", err)
	}

	startDone := make(chan struct{})
	go func() {
		defer close(startDone)
		_, _ = m.Start(StartConfig{ID: "s1"}) // self-heal: evicts + closes the first session
	}()

	// The evicted session closes done at the start of Close. By this point the
	// replacement must already be registered and the manager lock released.
	<-first.Session.done
	readResult := make(chan string, 1)
	go func() {
		readResult <- m.GetTokenForSession("s1")
	}()

	select {
	case token := <-readResult:
		if token == "" || token == first.Token {
			t.Fatalf("replacement token = %q, want a new published token", token)
		}
	case <-time.After(time.Second):
		t.Fatal("Start held m.mu across the evicted session's Close: manager reads blocked > 1s")
	}

	unblockClose()
	<-startDone
}

// Start must hand back the session it just spawned. Callers used to re-fetch it
// with `sess, _ := mgr.Get(result.SessionID)` — swallowing the error and then
// dereferencing the result — which races a concurrent Stop: the session is gone
// from the map by the time the lookup runs, and the caller nil-derefs (finding
// S4). The value exists inside Start; there is no reason to look it up again.
func TestManager_StartReturnsTheSpawnedSession(t *testing.T) {
	m := newFakeManager()

	res, err := m.Start(StartConfig{ID: "s1"})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if res.Session == nil {
		t.Fatal("StartResult.Session is nil — callers are forced back into a racy Get(result.SessionID)")
	}
	registered, err := m.Get("s1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if res.Session != registered {
		t.Fatal("StartResult.Session must be the session registered under the ID")
	}

	// A concurrent Stop is exactly the race: the map no longer has the session, but
	// the caller's handle from Start is still valid.
	// (Stop's error is the fake session's nil-ptmx Close, not a bookkeeping failure.)
	_ = m.Stop("s1")
	if _, err := m.Get("s1"); err == nil {
		t.Fatal("Get after Stop should fail — that is the window the caller used to deref through")
	}
	if res.Session.ID != "s1" {
		t.Fatalf("the handle from Start must survive a Stop, got %q", res.Session.ID)
	}
}

func TestManager_StartWithFakeSpawn_SpawnErrorPropagates(t *testing.T) {
	m := NewManager()
	sentinel := errors.New("spawn boom")
	m.spawn = func(SpawnConfig) (*Session, error) { return nil, sentinel }

	_, err := m.Start(StartConfig{ID: "s1"})
	if !errors.Is(err, sentinel) {
		t.Fatalf("Start with failing spawn: got %v, want the spawn error", err)
	}
	// A failed spawn must leave no bookkeeping behind.
	if _, err := m.GetByRepoAgent("", ""); err == nil {
		t.Error("a failed spawn must not register a session")
	}
}

func TestManager_Start_LiveSessionReturnsExists(t *testing.T) {
	m := NewManager()
	if _, err := m.Start(StartConfig{ID: "live", Cmd: "/bin/sh", Args: []string{"-c", "exec cat"}}); err != nil {
		t.Fatalf("first Start: %v", err)
	}
	defer m.Stop("live")

	_, err := m.Start(StartConfig{ID: "live", Cmd: "/bin/sh", Args: []string{"-c", "exec cat"}})
	if !errors.Is(err, ErrSessionExists) {
		t.Fatalf("second Start of a LIVE session: got %v, want ErrSessionExists", err)
	}
}

// A session whose child has already exited is self-healed on the next Start:
// evicted and re-spawned with a fresh token, exercising the real dead-session
// Close path (which must not block the manager lock).
func TestManager_Start_SelfHealsDeadRealSession(t *testing.T) {
	m := NewManager()
	res1, err := m.Start(StartConfig{ID: "s1", Cmd: "/bin/sh", Args: []string{"-c", "exit 0"}})
	if err != nil {
		t.Fatalf("first Start: %v", err)
	}
	sess, err := m.Get("s1")
	if err != nil {
		t.Fatalf("Get after first Start: %v", err)
	}
	<-sess.reaped
	if sess.IsAlive() {
		t.Fatal("reaped session still reports alive")
	}

	res2, err := m.Start(StartConfig{ID: "s1", Cmd: "/bin/sh", Args: []string{"-c", "exec cat"}})
	if err != nil {
		t.Fatalf("self-heal Start of a dead real session: got %v, want success", err)
	}
	defer m.Stop("s1")

	if !res2.Replaced {
		t.Fatal("self-heal should report Replaced=true")
	}
	if res2.Token == res1.Token {
		t.Fatal("self-heal must mint a fresh token")
	}
	if _, err := m.GetByToken(res1.Token); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("old token must be invalidated, got %v", err)
	}
	if got := m.Count(); got != 1 {
		t.Fatalf("Count()=%d, want 1 (corpse evicted, not accumulated)", got)
	}
}
