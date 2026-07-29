// main_connection_recovery_primitives_test.go — Tests daemon recovery I/O primitives.
// Docs: docs/features/feature/mcp-persistent-server/index.md

package main

import (
	"net"
	"net/http"
	"os/exec"
	"runtime"
	"testing"
	"time"
)

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
