// output_test.go — Verifies diagnostic sink redirection without touching MCP stdout.

package diag

import (
	"bytes"
	"io"
	"testing"
)

func TestSetSinkRedirectsAllDiagnosticWriters(t *testing.T) {
	previous := Sink()
	t.Cleanup(func() { SetSink(previous) })

	var output bytes.Buffer
	SetSink(&output)
	Print("one")
	Printf(" %s", "two")
	Println(" three")

	if got := output.String(); got != "one two three\n" {
		t.Fatalf("diagnostic output = %q", got)
	}
}

func TestSetSinkIgnoresNil(t *testing.T) {
	previous := Sink()
	t.Cleanup(func() { SetSink(previous) })

	SetSink(io.Discard)
	SetSink(nil)
	if Sink() != io.Discard {
		t.Fatal("nil sink should leave the current diagnostic sink unchanged")
	}
}
