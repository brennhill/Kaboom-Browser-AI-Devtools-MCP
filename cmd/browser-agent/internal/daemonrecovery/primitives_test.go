// primitives_test.go — Tests daemon recovery I/O primitives.

package daemonrecovery

import (
	"net"
	"net/http"
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
			if got := TryShutdownViaHTTP(port); got != test.want {
				t.Fatalf("TryShutdownViaHTTP() = %v, want %v", got, test.want)
			}
		})
	}
	if TryShutdownViaHTTP(freePort(t)) {
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

	if WaitForPortRelease(port, 60*time.Millisecond) {
		t.Fatal("occupied port reported released")
	}
	if err := server.Close(); err != nil {
		t.Fatal(err)
	}
	if !WaitForPortRelease(port, time.Second) {
		t.Fatal("closed port remained occupied")
	}
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
