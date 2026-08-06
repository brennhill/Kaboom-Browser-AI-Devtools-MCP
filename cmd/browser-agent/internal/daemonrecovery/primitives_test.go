// primitives_test.go — Tests daemon recovery I/O primitives.

package daemonrecovery

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestTryShutdownViaHTTPReportsAcceptedAndRejectedResponses(t *testing.T) {
	for _, test := range []struct {
		name   string
		status int
		want   bool
	}{
		{name: "accepted", status: http.StatusOK, want: true},
		{name: "rejected", status: http.StatusForbidden, want: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
				if request.Method != http.MethodPost || request.URL.Path != "/shutdown" {
					t.Errorf("request = %s %s", request.Method, request.URL.Path)
				}
				w.WriteHeader(test.status)
			})}
			go func() { _ = server.Serve(listener) }()
			defer server.Close()

			port := listener.Addr().(*net.TCPAddr).Port
			if got := tryShutdownViaHTTP(port); got != test.want {
				t.Fatalf("tryShutdownViaHTTP() = %v, want %v", got, test.want)
			}
		})
	}
	if tryShutdownViaHTTP(freePort(t)) {
		t.Fatal("shutdown succeeded without a server")
	}
}

func TestWaitForPortReleaseObservesOccupiedAndClosedPorts(t *testing.T) {
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
}

func TestFetchDaemonHealthClassifiesReachableRefusedAndCancelled(t *testing.T) {
	t.Run("reachable", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_ = json.NewEncoder(w).Encode(map[string]string{"version": "1.2.3"})
		}))
		defer server.Close()
		port := portFromURL(t, server.URL)
		reachable, version, refused := fetchDaemonHealth(context.Background(), port, time.Second)
		if !reachable || version != "1.2.3" || refused {
			t.Fatalf("reachable=%v version=%q refused=%v", reachable, version, refused)
		}
	})

	t.Run("refused", func(t *testing.T) {
		port := freePort(t)
		reachable, _, refused := fetchDaemonHealth(context.Background(), port, time.Second)
		if reachable || !refused {
			t.Fatalf("reachable=%v refused=%v", reachable, refused)
		}
	})

	t.Run("cancelled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if reachable, _, _ := fetchDaemonHealth(ctx, 7890, time.Second); reachable {
			t.Fatal("cancelled probe reported reachable")
		}
	})
}

func portFromURL(t *testing.T, rawURL string) int {
	t.Helper()
	index := strings.LastIndexByte(rawURL, ':')
	if index < 0 {
		t.Fatalf("URL has no port: %q", rawURL)
	}
	var port int
	if _, err := fmt.Sscanf(rawURL[index+1:], "%d", &port); err != nil {
		t.Fatalf("parse port: %v", err)
	}
	return port
}

func TestStopServerForUpgradeAcceptsAlreadyFreePort(t *testing.T) {
	if !StopServerForUpgrade(freePort(t)) {
		t.Fatal("already-free port could not be prepared for upgrade")
	}
}

func freePort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	return port
}
