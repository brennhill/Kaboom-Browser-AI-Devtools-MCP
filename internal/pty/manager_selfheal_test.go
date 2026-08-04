// manager_selfheal_test.go — real-process coverage for Start's self-heal:
// a LIVE session with the same ID still returns ErrSessionExists, while a session
// whose child has exited is evicted and re-spawned (exercising the real Close
// path on a dead session, which the fake-spawn tests cannot).

package pty

import (
	"errors"
	"testing"
)

// A live session must NOT be self-healed: a second Start of the same ID returns
// ErrSessionExists (the client uses that 409 to reconnect to the running shell).
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
