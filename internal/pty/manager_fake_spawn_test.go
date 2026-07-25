// manager_fake_spawn_test.go — exercises the Manager's token/index/limit
// bookkeeping WITHOUT launching real OS processes, via the injectable `spawn`
// seam. Previously the only way to reach ErrMaxSessions was to spawn all 10 real
// shells; ErrSessionExists / indexing likewise needed real processes.
//
// (No t.Parallel: these build a fresh Manager but follow manager_test.go's
// serial convention for the package.)

package pty

import (
	"errors"
	"os/exec"
	"testing"
)

// fakeSpawn returns a minimal Session that survives Pid() (cmd non-nil, Process
// nil -> Pid == -1) and is safe to Close(): `done` is open and `reaped` is
// pre-closed so Close() returns immediately without a real child. Because
// Process is nil, IsAlive() always reports false — so a fake is treated as a
// "dead" session, which is exactly what the self-heal path needs to exercise.
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
