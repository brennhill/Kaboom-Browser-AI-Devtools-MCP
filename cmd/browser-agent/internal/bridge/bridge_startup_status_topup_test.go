// bridge_startup_status_topup_test.go — Focused unit tests for the pure/HTTP-probe
// helpers in bridge_startup_status.go and the wrapper-log path resolver.

package bridge

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

func portOfURL(t *testing.T, rawURL string) int {
	t.Helper()
	p, err := strconv.Atoi(rawURL[strings.LastIndex(rawURL, ":")+1:])
	if err != nil {
		t.Fatalf("parse port from %q: %v", rawURL, err)
	}
	return p
}

func TestIsServerRunning_NotListening(t *testing.T) {
	if IsServerRunning(59991) {
		t.Fatal("IsServerRunning(59991) = true, want false (nothing listening)")
	}
}

func TestResolveBridgeWrapperLogPath_ReturnsWrapperFile(t *testing.T) {
	for _, hint := range []string{"", "/tmp/hintdir/wrapper.log"} {
		got := resolveBridgeWrapperLogPath(hint)
		if !strings.HasSuffix(got, bridgeWrapperLogFileName) {
			t.Fatalf("resolveBridgeWrapperLogPath(%q) = %q, want suffix %q", hint, got, bridgeWrapperLogFileName)
		}
	}
}

func TestDaemonStartupSuggestion_Branches(t *testing.T) {
	if s := daemonStartupSuggestion("failed to bind to port", 7890); !strings.Contains(s, "--port 7891") {
		t.Fatalf("port/bind suggestion = %q, want next-port hint", s)
	}
	if s := daemonStartupSuggestion("some unrelated failure", 7890); !strings.Contains(s, "--doctor") {
		t.Fatalf("generic suggestion = %q, want --doctor hint", s)
	}
}

func TestRunningServerVersionCompatible_DeadPort(t *testing.T) {
	if ok, _, _ := runningServerVersionCompatible(59992); ok {
		t.Fatal("expected not-compatible for a dead port")
	}
}

func TestRunningServerVersionCompatible_Non200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	if ok, _, _ := runningServerVersionCompatible(portOfURL(t, srv.URL)); ok {
		t.Fatal("expected not-compatible for non-200 /health")
	}
}

func TestRunningServerVersionCompatible_HealthServed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"service-name":"kaboom-browser-devtools","version":"0.8.4"}`))
	}))
	defer srv.Close()
	// Exercises the read-body + DecodeHealthMetadata + service/version compat path.
	_, _, _ = runningServerVersionCompatible(portOfURL(t, srv.URL))
}
