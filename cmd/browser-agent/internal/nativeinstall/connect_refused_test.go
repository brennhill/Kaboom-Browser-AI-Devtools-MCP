// connect_refused_test.go — FetchHealth must distinguish a
// connection-refused failure (nothing listening → definitively gone) from other
// failures, so the takeover probe can skip its retry budget on a certainty (L).

package nativeinstall

import (
	"context"
	"net"
	"testing"
	"time"
)

func TestFetchInstallHealth_RefusedOnClosedPort(t *testing.T) {
	// Bind a port then immediately release it, so nothing is listening on it.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	if err := ln.Close(); err != nil {
		t.Fatalf("close listener: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	h := FetchHealth(ctx, port, time.Second)

	if h.Reachable {
		t.Fatal("a closed port must not be reachable")
	}
	if !h.Refused {
		t.Fatal("a closed port must be reported as connection-refused (refused=true) so the probe skips retries")
	}
}
