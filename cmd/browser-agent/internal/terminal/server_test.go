// server_test.go -- Tests for terminal mux setup and HTTP server lifecycle.
// Uses a real loopback listener (no PTY) so bind success/failure and graceful
// shutdown are exercised deterministically.

package terminal

import (
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	terminalintent "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/terminal/intent"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/terminal/sessionrelay"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/pty"
)

type fakeMuxIntentDeps struct {
	store  *terminalintent.Store
	relays *sessionrelay.Map
}

func (f *fakeMuxIntentDeps) GetPtyRelays() terminalintent.RelayMap { return f.relays }
func (f *fakeMuxIntentDeps) GetIntentStore() *terminalintent.Store { return f.store }

func TestSetupMux_WiresTerminalAndIntentRoutes(t *testing.T) {
	t.Parallel()
	deps := testDeps()
	mgr := pty.NewManager()
	defer mgr.StopAll()
	store := capture.NewCapture()

	mux, relays := SetupMux(deps, &fakeServerDeps{}, &fakeMuxIntentDeps{store: terminalintent.NewStore(), relays: sessionrelay.NewMap()}, mgr, store)
	if mux == nil || relays == nil {
		t.Fatal("SetupMux returned nil mux or relays")
	}

	// A terminal route (RegisterRoutes) is wired.
	req := httptest.NewRequest("GET", "/terminal/config", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("/terminal/config: expected 200, got %d", rec.Code)
	}

	// An intent route (RegisterIntentRoutes) is wired.
	req = httptest.NewRequest("GET", "/intent", nil)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("/intent GET: expected 405, got %d", rec.Code)
	}
}

func TestStartServer_BindsAndShutsDown(t *testing.T) {
	t.Parallel()
	deps := testDeps()
	mux := http.NewServeMux()

	// Port 0 => bind an OS-assigned free port on loopback.
	srv, done, err := StartServer(deps, 0, mux)
	if err != nil {
		t.Fatalf("StartServer: unexpected bind error: %v", err)
	}
	if srv == nil || done == nil {
		t.Fatal("StartServer returned nil server or done channel")
	}

	// Graceful shutdown closes the done channel.
	if err := srv.Close(); err != nil {
		t.Fatalf("srv.Close: %v", err)
	}
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("done channel not closed after server shutdown")
	}
}

func TestStartServer_BindFailure(t *testing.T) {
	t.Parallel()
	deps := testDeps()
	mux := http.NewServeMux()

	// Occupy a port so StartServer's bind on the same port fails.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("prep listen: %v", err)
	}
	defer func() { _ = ln.Close() }()
	port := ln.Addr().(*net.TCPAddr).Port

	srv, done, err := StartServer(deps, port, mux)
	if err == nil {
		if srv != nil {
			_ = srv.Close()
		}
		t.Fatal("expected bind failure on occupied port, got nil error")
	}
	if srv != nil || done != nil {
		t.Fatal("expected nil server and done on bind failure")
	}
}
