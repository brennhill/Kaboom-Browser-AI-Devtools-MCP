// frame_writer_deadline_test.go — each WS frame write is bounded by a write
// deadline so a stalled reader (backgrounded-tab zero-window, hostile client)
// cannot wedge the downstream/ping goroutines. A nil conn (in-memory test writer)
// must be tolerated.

package terminal

import (
	"bufio"
	"bytes"
	"testing"
	"time"
)

type deadlineRecorder struct {
	set []time.Time
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
	// The deadline must be roughly now+WSWriteTimeout (allow slack for scheduling).
	if rec.set[0].Before(before.Add(WSWriteTimeout - time.Second)) {
		t.Fatalf("deadline %v should be ~%v ahead", rec.set[0], WSWriteTimeout)
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
