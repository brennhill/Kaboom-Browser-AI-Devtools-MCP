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
	defer server.logs.Shutdown(2 * time.Second)

	deps := server.daemonRecovery.LifecycleDeps()
	v := reflect.ValueOf(deps)
	for i := 0; i < v.NumField(); i++ {
		name := v.Type().Field(i).Name
		field := v.Field(i)
		switch field.Kind() {
		case reflect.Func, reflect.Interface:
			if field.IsNil() {
				t.Errorf("LifecycleDeps().%s is nil; daemonlife calls every seam unconditionally", name)
			}
		case reflect.String:
			if field.String() == "" {
				t.Errorf("LifecycleDeps().%s is empty", name)
			}
		}
	}
}
