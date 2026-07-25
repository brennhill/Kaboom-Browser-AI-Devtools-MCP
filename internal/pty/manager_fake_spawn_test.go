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
// nil -> Pid == -1). These tests never Close() the fakes, so ptmx/done stay nil.
func fakeSpawn(cfg SpawnConfig) (*Session, error) {
	return &Session{ID: cfg.ID, cmd: &exec.Cmd{}}, nil
}

func newFakeManager() *Manager {
	m := NewManager()
	m.spawn = fakeSpawn
	return m
}

func TestManager_StartWithFakeSpawn_SessionExists(t *testing.T) {
	m := newFakeManager()

	if _, err := m.Start(StartConfig{ID: "s1"}); err != nil {
		t.Fatalf("first Start: unexpected error %v", err)
	}
	_, err := m.Start(StartConfig{ID: "s1"})
	if !errors.Is(err, ErrSessionExists) {
		t.Fatalf("second Start of same ID: got %v, want ErrSessionExists", err)
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
