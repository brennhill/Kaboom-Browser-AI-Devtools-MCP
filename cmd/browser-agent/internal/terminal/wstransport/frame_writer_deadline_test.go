// frame_writer_deadline_test.go — each WS frame write is bounded by a write
// deadline so a stalled reader (backgrounded-tab zero-window, hostile client)
// cannot wedge the downstream/ping goroutines. A nil conn (in-memory test writer)
// must be tolerated.

package wstransport

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/pty"
)

type deadlineRecorder struct {
	set []time.Time
}

func TestWriteDropReason(t *testing.T) {
	t.Parallel()
	tests := map[string]struct {
		err  error
		want string
	}{
		"closed":  {pty.ErrWriteBufferClosed, "session_ended"},
		"full":    {pty.ErrWriteBufferFull, "backpressure"},
		"wrapped": {fmt.Errorf("ws upstream: %w", pty.ErrWriteBufferClosed), "session_ended"},
		"unknown": {errors.New("boom"), "write_error"},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			if got := writeDropReason(test.err); got != test.want {
				t.Fatalf("writeDropReason(%v) = %q, want %q", test.err, got, test.want)
			}
		})
	}
}

func (d *deadlineRecorder) SetWriteDeadline(t time.Time) error {
	d.set = append(d.set, t)
	return nil
}

func TestNewFrameWriter_BoundsWriteWithDeadline(t *testing.T) {
	t.Parallel()
	var wire bytes.Buffer
	rw := bufio.NewReadWriter(bufio.NewReader(&wire), bufio.NewWriter(&wire))
	rec := &deadlineRecorder{}
	writeFrame := NewFrameWriter(rec, rw, testDeps())

	before := time.Now()
	if err := writeFrame(0x1, []byte("hi")); err != nil {
		t.Fatalf("writeFrame: %v", err)
	}
	if len(rec.set) != 1 {
		t.Fatalf("expected exactly one SetWriteDeadline call, got %d", len(rec.set))
	}
	// The deadline must be roughly now+WriteTimeout (allow slack for scheduling).
	if rec.set[0].Before(before.Add(WriteTimeout - time.Second)) {
		t.Fatalf("deadline %v should be ~%v ahead", rec.set[0], WriteTimeout)
	}
}

func TestNewFrameWriter_NilConnDoesNotPanic(t *testing.T) {
	t.Parallel()
	var wire bytes.Buffer
	rw := bufio.NewReadWriter(bufio.NewReader(&wire), bufio.NewWriter(&wire))
	writeFrame := NewFrameWriter(nil, rw, testDeps())
	if err := writeFrame(0x1, []byte("hi")); err != nil {
		t.Fatalf("writeFrame with nil conn: %v", err)
	}
}
