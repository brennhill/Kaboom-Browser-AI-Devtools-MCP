// Purpose: Guards that every daemonlife seam this package must supply is actually wired.
// Docs: docs/features/feature/mcp-persistent-server/index.md

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"strings"
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
	defer server.logs.Shutdown(2 * time.Second)

	oldAlive := server.daemonHost.isProcessAlive
	defer func() { server.daemonHost.isProcessAlive = oldAlive }()

	server.daemonHost.isProcessAlive = func(int) bool { return true }
	if !daemonlifeDeps(server).IsProcessAlive(1) {
		t.Fatal("IsProcessAlive should reflect the stub installed before the call")
	}
	server.daemonHost.isProcessAlive = func(int) bool { return false }
	if daemonlifeDeps(server).IsProcessAlive(1) {
		t.Fatal("Deps must be rebuilt per call; a stub swapped later was not picked up")
	}
}

// TestFetchDaemonHealth reduces a real /health response to the three facts the
// takeover policy consumes. Getting `refused` wrong is what decides whether a
// live peer is retried or SIGTERM'd, so it is pinned here against a real server.
func TestFetchDaemonHealth(t *testing.T) {
	t.Run("reachable reports the version", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_ = json.NewEncoder(w).Encode(map[string]any{"version": "1.2.3"})
		}))
		defer srv.Close()
		port := portFromTestServerURL(t, srv.URL)

		reachable, ver, refused := fetchDaemonHealth(context.Background(), port, time.Second)
		if !reachable || ver != "1.2.3" || refused {
			t.Fatalf("got reachable=%v version=%q refused=%v, want true/1.2.3/false", reachable, ver, refused)
		}
	})

	t.Run("nothing listening reports refused, not merely unreachable", func(t *testing.T) {
		// Bind and immediately close so the port is almost certainly free.
		srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
		port := portFromTestServerURL(t, srv.URL)
		srv.Close()

		reachable, _, refused := fetchDaemonHealth(context.Background(), port, time.Second)
		if reachable {
			t.Fatal("a closed port must not report reachable")
		}
		if !refused {
			t.Fatal("a closed port must report refused so the caller can skip its retry budget")
		}
	})

	t.Run("a cancelled context is unreachable, never a hang", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if reachable, _, _ := fetchDaemonHealth(ctx, 7890, time.Second); reachable {
			t.Fatal("a cancelled probe must not report reachable")
		}
	})
}

// portFromTestServerURL extracts the port from an httptest server URL.
func portFromTestServerURL(t *testing.T, rawURL string) int {
	t.Helper()
	idx := strings.LastIndex(rawURL, ":")
	if idx < 0 {
		t.Fatalf("no port in %q", rawURL)
	}
	var port int
	if _, err := fmt.Sscanf(rawURL[idx+1:], "%d", &port); err != nil {
		t.Fatalf("parse port from %q: %v", rawURL, err)
	}
	return port
}
