// Purpose: Guards that every daemonlife seam this package must supply is actually wired.
// Docs: docs/features/feature/mcp-persistent-server/index.md

package main

import (
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

// TestDaemonlifeDeps_AllSeamsWired is the regression guard for the one failure mode
// the Deps contract introduces: daemonlife calls every func field unconditionally, so
// a field left nil is a startup panic that no unit test of daemonlife itself can
// catch. Adding a field to daemonlife.Deps without wiring it here fails this test.
func TestDaemonlifeDeps_AllSeamsWired(t *testing.T) {
	server, err := NewServer(filepath.Join(t.TempDir(), "wiring.log"), 10)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}
	defer server.logs.shutdownAsyncLogger(2 * time.Second)

	deps := daemonlifeDeps(server)
	v := reflect.ValueOf(deps)
	for i := 0; i < v.NumField(); i++ {
		name := v.Type().Field(i).Name
		field := v.Field(i)
		switch field.Kind() {
		case reflect.Func, reflect.Interface:
			if field.IsNil() {
				t.Errorf("daemonlifeDeps().%s is nil; daemonlife calls every seam unconditionally", name)
			}
		case reflect.String:
			if field.String() == "" {
				t.Errorf("daemonlifeDeps().%s is empty", name)
			}
		}
	}
}

// TestDaemonlifeDeps_ReadsSeamsAtCallTime pins the property the call sites rely on:
// Deps is rebuilt per call, so swapping an injectable seam (as tests do) is visible
// to daemonlife. If this were captured once at init, test stubs would be ignored.
func TestDaemonlifeDeps_ReadsSeamsAtCallTime(t *testing.T) {
	server, err := NewServer(filepath.Join(t.TempDir(), "wiring-latebind.log"), 10)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}
	defer server.logs.shutdownAsyncLogger(2 * time.Second)

	oldAlive := daemonIsProcessAlive
	defer func() { daemonIsProcessAlive = oldAlive }()

	daemonIsProcessAlive = func(int) bool { return true }
	if !daemonlifeDeps(server).IsProcessAlive(1) {
		t.Fatal("IsProcessAlive should reflect the stub installed before the call")
	}
	daemonIsProcessAlive = func(int) bool { return false }
	if daemonlifeDeps(server).IsProcessAlive(1) {
		t.Fatal("Deps must be rebuilt per call; a stub swapped later was not picked up")
	}
}
