// server_test.go — Verifies daemon HTTP binding, serving, and conflict diagnostics.

package daemonhttp

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"
	"testing"
)

func TestStartServesUntilShutdownAndReportsLifecycle(t *testing.T) {
	events := make([]string, 0, 1)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	_ = listener.Close()
	mux := http.NewServeMux()
	mux.HandleFunc("/ready", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })

	srv, done, err := Start(Deps{
		LogLifecycle: func(event string, _ int, _ map[string]any) { events = append(events, event) },
	}, port, "", mux)
	if err != nil {
		t.Fatalf("start HTTP server: %v", err)
	}
	response, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/ready", port)) // #nosec G107 -- loopback test server
	if err != nil {
		t.Fatalf("GET ready: %v", err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusNoContent {
		t.Fatalf("ready status = %d", response.StatusCode)
	}
	if err := srv.Shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown HTTP server: %v", err)
	}
	<-done
	if len(events) != 1 || events[0] != "http_bind_success" {
		t.Fatalf("lifecycle events = %v", events)
	}
}

func TestStartAndPreflightDescribeOccupiedPort(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	port := listener.Addr().(*net.TCPAddr).Port
	events := make([]string, 0, 2)
	deps := Deps{
		IdentifyPortHolder: func(int) (int, string) { return 4242, "other-daemon" },
		LogLifecycle:       func(event string, _ int, _ map[string]any) { events = append(events, event) },
	}
	if err := Preflight(deps, port); err == nil || !strings.Contains(err.Error(), "pid 4242 (other-daemon)") {
		t.Fatalf("occupied-port preflight error = %v", err)
	}
	if _, _, err := Start(deps, port, "", http.NewServeMux()); err == nil || !strings.Contains(err.Error(), "cannot bind") {
		t.Fatalf("occupied-port start error = %v", err)
	}
	if len(events) != 2 || events[0] != "port_conflict_detected" || events[1] != "http_bind_failed" {
		t.Fatalf("lifecycle events = %v", events)
	}
}
