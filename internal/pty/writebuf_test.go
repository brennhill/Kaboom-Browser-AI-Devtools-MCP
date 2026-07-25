// writebuf_test.go — Tests for non-blocking write buffer with backpressure.

package pty

import (
	"bytes"
	"errors"
	"io"
	"sync"
	"testing"
	"time"
)

// TestWriteBuffer_CloseDoesNotHangOnStuckWriter reproduces the confirmed shutdown
// hang: drain is blocked inside writer.Write (a PTY child that stopped reading
// stdin), which close(notify) cannot interrupt. Close must return within its
// bound with ErrWriteBufferCloseTimeout instead of blocking forever.
func TestWriteBuffer_CloseDoesNotHangOnStuckWriter(t *testing.T) {
	gw := &gatedWriter{gate: make(chan struct{})} // gate never closed -> Write blocks forever
	wb := NewWriteBuffer(gw)
	if _, err := wb.Write([]byte("stuck")); err != nil {
		t.Fatalf("write: %v", err)
	}

	done := make(chan error, 1)
	go func() { done <- wb.Close() }()

	select {
	case err := <-done:
		if !errors.Is(err, ErrWriteBufferCloseTimeout) {
			t.Fatalf("Close on a stuck writer should time out, got %v", err)
		}
	case <-time.After(writeBufferCloseTimeout + 3*time.Second):
		t.Fatal("Close hung on a stuck writer — the bound did not fire")
	}
	close(gw.gate) // let the blocked drain goroutine unwind so it does not leak
}

// The stuck-writer close timeout leaks a drain goroutine + fd that cannot be safely
// interrupted; that leak must not be SILENT. Close must both return
// ErrWriteBufferCloseTimeout AND fire the diagnostics hook so it is diagnosable (M).
func TestWriteBuffer_CloseTimeoutSurfacesViaHookAndError(t *testing.T) {
	// Shorten the bound so the timeout path runs fast; restore after.
	origTimeout := writeBufferCloseTimeout
	writeBufferCloseTimeout = 40 * time.Millisecond
	defer func() { writeBufferCloseTimeout = origTimeout }()

	var firedPending int
	var fired bool
	SetWriteBufferCloseTimeoutHook(func(pending int) { fired = true; firedPending = pending })
	defer SetWriteBufferCloseTimeoutHook(nil)

	gw := &gatedWriter{gate: make(chan struct{})} // Write blocks forever
	wb := NewWriteBuffer(gw)
	if _, err := wb.Write([]byte("stuck")); err != nil {
		t.Fatalf("write: %v", err)
	}

	err := wb.Close()
	if !errors.Is(err, ErrWriteBufferCloseTimeout) {
		t.Fatalf("Close on a stuck writer must return the timeout error, got %v", err)
	}
	if !fired {
		t.Fatal("the close timeout must fire the diagnostics hook — a silent goroutine+fd leak is not diagnosable")
	}
	if firedPending != 5 {
		t.Fatalf("the hook should report the undrained byte count (5), got %d", firedPending)
	}
	close(gw.gate) // let the blocked drain goroutine unwind so it does not leak into other tests
}

func TestWriteBuffer_BasicWrite(t *testing.T) {
	var dest bytes.Buffer
	wb := NewWriteBuffer(&dest)

	n, err := wb.Write([]byte("hello"))
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	if n != 5 {
		t.Fatalf("expected 5 bytes, got %d", n)
	}

	// Close waits for drain to complete, making dest safe to read.
	wb.Close()

	if dest.String() != "hello" {
		t.Fatalf("expected %q, got %q", "hello", dest.String())
	}
}

func TestWriteBuffer_Backpressure(t *testing.T) {
	gw := &gatedWriter{gate: make(chan struct{})}
	wb := NewWriteBuffer(gw)
	defer func() {
		close(gw.gate)
		wb.Close()
	}()

	// Fill buffer to capacity.
	data := make([]byte, writeBufferMax)
	_, err := wb.Write(data)
	if err != nil {
		t.Fatalf("write to fill: %v", err)
	}

	// Exceeding capacity should fail.
	_, err = wb.Write([]byte("x"))
	if err != ErrWriteBufferFull {
		t.Fatalf("expected ErrWriteBufferFull, got: %v", err)
	}
}

// gatedWriter blocks on Write until gate channel is closed.
type gatedWriter struct {
	gate chan struct{}
}

func (w *gatedWriter) Write(p []byte) (int, error) {
	<-w.gate
	return len(p), nil
}

func TestWriteBuffer_Pending(t *testing.T) {
	gw := &gatedWriter{gate: make(chan struct{})}
	wb := NewWriteBuffer(gw)
	defer func() {
		close(gw.gate)
		wb.Close()
	}()

	wb.Write([]byte("hello"))
	// Pending must reflect the buffered bytes while the drain is blocked. drain()
	// only reslices wb.buf *after* the underlying Write returns (here: never,
	// until the gate closes in the deferred cleanup), so all 5 bytes stay buffered
	// and Pending() is deterministically 5. (The old `p < 0` check on a len()-based
	// value could never fail and proved nothing.)
	if p := wb.Pending(); p != 5 {
		t.Fatalf("expected 5 pending bytes while drain is blocked, got %d", p)
	}
}

func TestWriteBuffer_CloseFlushes(t *testing.T) {
	var dest bytes.Buffer
	wb := NewWriteBuffer(&dest)

	wb.Write([]byte("data"))
	wb.Close()

	if dest.String() != "data" {
		t.Fatalf("expected %q after close, got %q", "data", dest.String())
	}
}

func TestWriteBuffer_DoubleClose(t *testing.T) {
	var dest bytes.Buffer
	wb := NewWriteBuffer(&dest)
	wb.Close()
	if err := wb.Close(); err != nil {
		t.Fatalf("double close: %v", err)
	}
}

func TestWriteBuffer_WriteAfterClose(t *testing.T) {
	var dest bytes.Buffer
	wb := NewWriteBuffer(&dest)
	wb.Close()

	_, err := wb.Write([]byte("x"))
	if err != ErrWriteBufferFull {
		t.Fatalf("expected ErrWriteBufferFull after close, got: %v", err)
	}
}

func TestWriteBuffer_LargeWrite(t *testing.T) {
	var dest bytes.Buffer
	wb := NewWriteBuffer(&dest)

	// Write data larger than one chunk to exercise chunked flushing.
	data := make([]byte, writeChunkSize*3)
	for i := range data {
		data[i] = byte(i % 256)
	}

	n, err := wb.Write(data)
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	if n != len(data) {
		t.Fatalf("expected %d bytes, got %d", len(data), n)
	}

	// Close waits for drain to complete, making dest safe to read.
	wb.Close()

	if dest.Len() != len(data) {
		t.Fatalf("expected %d drained bytes, got %d", len(data), dest.Len())
	}
}

func TestWriteBuffer_ConcurrentWriteDuringClose(t *testing.T) {
	// Write signals wb.notify and Close closes it: a keystroke arriving as the
	// shell exits pits handlers.go's Write against relay.go's deferred Close on
	// the same channel. Unsynchronized, that is a data race the detector flags
	// and can panic with "send on closed channel". Many rounds with several
	// concurrent writers make the interleaving reliable under `go test -race`.
	for round := 0; round < 300; round++ {
		wb := NewWriteBuffer(io.Discard)
		var wg sync.WaitGroup
		for w := 0; w < 4; w++ {
			wg.Add(1)
			go func() { // lint:allow-bare-goroutine — bounded by the loop, joined via wg
				defer wg.Done()
				for i := 0; i < 25; i++ {
					_, _ = wb.Write([]byte("x"))
				}
			}()
		}
		// Close concurrently with the in-flight writers — the whole point.
		_ = wb.Close()
		wg.Wait()
	}
}
