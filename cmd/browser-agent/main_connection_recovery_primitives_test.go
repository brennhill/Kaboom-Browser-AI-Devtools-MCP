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
	"os/exec"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestIdentifyPortHolderFiltersInvalidAndSelfOwners(t *testing.T) {
	originalFind := daemonFindProcessOnPort
	originalCommand := daemonProcessCommand
	t.Cleanup(func() {
		daemonFindProcessOnPort = originalFind
		daemonProcessCommand = originalCommand
	})
	daemonProcessCommand = func(pid int) string { return "foreign-server --pid " + fmt.Sprint(pid) }
	daemonFindProcessOnPort = func(int) ([]int, error) { return []int{-1, os.Getpid(), 4242}, nil }
	pid, command := identifyPortHolder(7890)
	if pid != 4242 || !strings.Contains(command, "4242") {
		t.Fatalf("identified holder = %d %q", pid, command)
	}
	daemonFindProcessOnPort = func(int) ([]int, error) { return nil, errors.New("lookup failed") }
	if pid, command := identifyPortHolder(7890); pid != 0 || command != "" {
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

func TestTryShutdownViaHTTPStatuses(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		want   bool
	}{
		{name: "accepted", status: http.StatusOK, want: true},
		{name: "rejected", status: http.StatusForbidden, want: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			port := listener.Addr().(*net.TCPAddr).Port
			server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
				if request.Method != http.MethodPost || request.URL.Path != "/shutdown" {
					t.Errorf("request = %s %s", request.Method, request.URL.Path)
				}
				w.WriteHeader(tc.status)
			})}
			go func() { _ = server.Serve(listener) }()
			defer server.Close()
			if got := tryShutdownViaHTTP(port); got != tc.want {
				t.Fatalf("tryShutdownViaHTTP = %v, want %v", got, tc.want)
			}
		})
	}
	if tryShutdownViaHTTP(findFreePort(t)) {
		t.Fatal("shutdown succeeded without a server")
	}
}

func TestWaitForPortReleaseAndFreePortUpgradeStop(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})}
	go func() { _ = server.Serve(listener) }()
	if waitForPortRelease(port, 60*time.Millisecond) {
		t.Fatal("occupied port reported released")
	}
	if err := server.Close(); err != nil {
		t.Fatal(err)
	}
	if !waitForPortRelease(port, time.Second) {
		t.Fatal("closed port remained occupied")
	}
	if !stopServerForUpgrade(findFreePort(t)) {
		t.Fatal("free port could not be prepared for upgrade")
	}
}

func TestTerminatePIDQuietStopsChild(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test child command is POSIX")
	}
	cmd := exec.Command("sh", "-c", "trap 'exit 0' TERM; while :; do sleep 1; done")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	terminatePIDQuiet(cmd.Process.Pid, false)
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		_ = cmd.Process.Kill()
		t.Fatal("child survived quiet termination")
	}
}
