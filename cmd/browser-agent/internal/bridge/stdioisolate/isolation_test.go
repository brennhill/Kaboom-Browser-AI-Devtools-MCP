// isolation_test.go — Tests for the wrapper-log path resolver, which decides where
// bridge diagnostics land once os.Stderr has been redirected away from the terminal.

package stdioisolate

import (
	"os"
	"strings"
	"testing"
)

func TestResolveWrapperLogPath_ReturnsWrapperFile(t *testing.T) {
	for _, hint := range []string{"", "/tmp/hintdir/wrapper.log"} {
		got := ResolveWrapperLogPath(hint)
		if !strings.HasSuffix(got, WrapperLogFileName) {
			t.Fatalf("ResolveWrapperLogPath(%q) = %q, want suffix %q", hint, got, WrapperLogFileName)
		}
	}
}

func TestActiveMCPTransportWriter_DefaultsToStdout(t *testing.T) {
	// Ensure() is never called in this package's tests, so the transport pointer
	// stays nil and the accessor must fall back to the process stdout. Writing
	// MCP frames anywhere else would silently break the protocol stream.
	if got := ActiveMCPTransportWriter(); got != os.Stdout {
		t.Fatalf("ActiveMCPTransportWriter() = %v, want os.Stdout fallback", got)
	}
}

func TestActiveMCPTransportWriter_PrefersConfiguredTransport(t *testing.T) {
	// Once isolation has run, the duplicated fd — not os.Stdout — must be returned.
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error = %v", err)
	}
	t.Cleanup(func() {
		_ = r.Close()
		_ = w.Close()
		mcpTransport.Store(nil)
	})
	mcpTransport.Store(w)
	if got := ActiveMCPTransportWriter(); got != w {
		t.Fatalf("ActiveMCPTransportWriter() = %v, want the configured transport %v", got, w)
	}
}
