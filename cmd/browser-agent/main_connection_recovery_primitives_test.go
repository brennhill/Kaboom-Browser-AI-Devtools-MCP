// main_connection_recovery_primitives_test.go — Tests daemon recovery I/O primitives.
// Docs: docs/features/feature/mcp-persistent-server/index.md

package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"syscall"
	"testing"
)

func TestIdentifyPortHolderFiltersInvalidAndSelfOwners(t *testing.T) {
	host := newDaemonHost()
	host.processCommand = func(pid int) string { return "foreign-server --pid " + fmt.Sprint(pid) }
	host.findProcessOnPort = func(int) ([]int, error) { return []int{-1, os.Getpid(), 4242}, nil }
	pid, command := identifyPortHolder(host, 7890)
	if pid != 4242 || !strings.Contains(command, "4242") {
		t.Fatalf("identified holder = %d %q", pid, command)
	}
	host.findProcessOnPort = func(int) ([]int, error) { return nil, errors.New("lookup failed") }
	if pid, command := identifyPortHolder(host, 7890); pid != 0 || command != "" {
		t.Fatalf("failed lookup holder = %d %q", pid, command)
	}
}

func TestHTTPServerStartupReportsBindFailureAndServesUntilShutdown(t *testing.T) {
	server := newTestServerForHandlers(t)
	mux := http.NewServeMux()
	mux.HandleFunc("/ready", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	port := findFreePort(t)
	srv, done, err := startHTTPServer(server, port, "", mux)
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

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	occupiedPort := listener.Addr().(*net.TCPAddr).Port
	defer listener.Close()
	if _, _, err := startHTTPServer(server, occupiedPort, "", mux); err == nil || !strings.Contains(err.Error(), "cannot bind") {
		t.Fatalf("occupied-port error = %v", err)
	}
}

func TestPreflightPortCheckAndSignalLabels(t *testing.T) {
	server := newTestServerForHandlers(t)
	if err := preflightPortCheck(server, findFreePort(t)); err != nil {
		t.Fatalf("free-port preflight: %v", err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	defer listener.Close()
	if err := preflightPortCheck(server, port); err == nil || !strings.Contains(err.Error(), "already in use") {
		t.Fatalf("occupied-port preflight error = %v", err)
	}
	for signal, want := range map[os.Signal]string{
		os.Interrupt: "Ctrl+C", syscall.SIGTERM: "SIGTERM", syscall.SIGHUP: "SIGHUP", syscall.Signal(99): "signal 99",
	} {
		if got := mapSignalSource(signal); !strings.Contains(got, want) {
			t.Errorf("mapSignalSource(%v) = %q, want %q", signal, got, want)
		}
	}
}
