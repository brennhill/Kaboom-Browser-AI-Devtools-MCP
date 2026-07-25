// writebuf.go — Non-blocking write buffer with backpressure for PTY input.
// Why: Prevents WebSocket handlers from blocking when the child process stalls on stdin.

package pty

import (
	"errors"
	"io"
	"sync"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/util"
)

// Write buffer constants.
const (
	writeBufferMax = 1 << 20   // 1 MB backpressure cap.
	writeChunkSize = 16 * 1024 // 16 KB per write syscall.
)

// writeBufferCloseTimeout bounds how long Close() waits for the drain goroutine to
// exit. drain can be blocked inside writer.Write (a PTY child that stopped reading
// stdin), which close(notify) cannot interrupt — without this bound, Close, and
// therefore daemon shutdown, would hang forever. A var (not const) so tests can
// shorten it to exercise the timeout path without a real multi-second wait.
var writeBufferCloseTimeout = 2 * time.Second

// writeBufferCloseTimeoutHook, if set, is invoked when Close times out waiting for
// the drain goroutine (a stuck writer leaks the goroutine + its fd, which cannot be
// safely interrupted). It exists purely so that leak is not SILENT (finding M): the
// daemon wires it to the structured log. Package-level so no logger has to be
// threaded through NewWriteBuffer/NewRelay for this rare defense-in-depth signal;
// guarded by a mutex because the setter and the Close-time reader can run
// concurrently (e.g. RegisterRoutes wiring vs. a shutdown Close).
var (
	writeBufferHookMu           sync.Mutex
	writeBufferCloseTimeoutHook func(pending int)
)

// SetWriteBufferCloseTimeoutHook installs (or clears, with nil) the diagnostics
// hook invoked when WriteBuffer.Close times out. Called at daemon wiring; also used
// by tests.
func SetWriteBufferCloseTimeoutHook(fn func(pending int)) {
	writeBufferHookMu.Lock()
	writeBufferCloseTimeoutHook = fn
	writeBufferHookMu.Unlock()
}

// fireWriteBufferCloseTimeout invokes the hook (if any) under the lock-free read of
// a snapshot, so a concurrent SetWriteBufferCloseTimeoutHook cannot race the call.
func fireWriteBufferCloseTimeout(pending int) {
	writeBufferHookMu.Lock()
	fn := writeBufferCloseTimeoutHook
	writeBufferHookMu.Unlock()
	if fn != nil {
		fn(pending)
	}
}

// ErrWriteBufferFull is returned when the write buffer exceeds the backpressure cap.
var ErrWriteBufferFull = errors.New("pty: write buffer full")

// ErrWriteBufferCloseTimeout is returned by Close when the drain goroutine did not
// exit within writeBufferCloseTimeout (the underlying writer is stuck). Surfaced so
// the caller can log it; the fd is reclaimed by the OS on process exit.
var ErrWriteBufferCloseTimeout = errors.New("pty: write buffer close timed out")

// WriteBuffer provides non-blocking writes with async draining to an io.Writer.
// Data remains in the buffer until successfully written to the underlying writer,
// providing accurate backpressure.
type WriteBuffer struct {
	mu      sync.Mutex
	buf     []byte
	maxSize int
	writer  io.Writer
	notify  chan struct{}
	done    chan struct{}
	closed  bool
}

// NewWriteBuffer creates a buffered writer that drains asynchronously.
func NewWriteBuffer(w io.Writer) *WriteBuffer {
	wb := &WriteBuffer{
		maxSize: writeBufferMax,
		writer:  w,
		notify:  make(chan struct{}, 1),
		done:    make(chan struct{}),
	}
	// Panic-recovered: the drain loop calls writer.Write on a hot path; a panic
	// there must never crash the daemon. drain's own `defer close(wb.done)` still
	// runs during unwind, so Close never hangs even if drain panics.
	util.SafeGo(wb.drain)
	return wb
}

// Write appends data to the buffer without blocking. Returns ErrWriteBufferFull
// if the buffer exceeds the backpressure cap.
func (wb *WriteBuffer) Write(data []byte) (int, error) {
	// The notify signal is sent while holding mu, together with the `closed`
	// check: Close closes wb.notify under the same lock, so a Write can never
	// send on an already-closed channel (a data race, and a "send on closed
	// channel" panic). The send is non-blocking, so holding mu across it is cheap.
	wb.mu.Lock()
	defer wb.mu.Unlock()
	if wb.closed {
		return 0, ErrWriteBufferFull
	}
	if len(wb.buf)+len(data) > wb.maxSize {
		return 0, ErrWriteBufferFull
	}
	wb.buf = append(wb.buf, data...)
	select {
	case wb.notify <- struct{}{}:
	default:
	}
	return len(data), nil
}

// drain waits for notifications and flushes buffered data to the writer.
func (wb *WriteBuffer) drain() {
	defer close(wb.done)
	for {
		_, ok := <-wb.notify
		if !ok {
			return
		}
		wb.flushAll()
	}
}

// flushAll writes all buffered data to the underlying writer in chunks.
// Data is only removed from the buffer after a successful write, so Pending()
// reflects the true amount of undelivered data.
func (wb *WriteBuffer) flushAll() {
	for {
		wb.mu.Lock() // lint:manual-unlock — lock/unlock brackets I/O outside lock
		if len(wb.buf) == 0 {
			wb.mu.Unlock()
			return
		}
		n := len(wb.buf)
		if n > writeChunkSize {
			n = writeChunkSize
		}
		chunk := make([]byte, n)
		copy(chunk, wb.buf[:n])
		wb.mu.Unlock()

		_, err := wb.writer.Write(chunk)
		if err != nil {
			// Leave data in buffer — caller can retry or Close will
			// attempt a final flush. For PTY stdin this typically means
			// the child process exited, so the data is undeliverable.
			return
		}

		wb.mu.Lock() // lint:manual-unlock — same pattern as above
		if len(wb.buf) >= n {
			wb.buf = wb.buf[n:]
		}
		if len(wb.buf) == 0 {
			wb.buf = nil
		}
		wb.mu.Unlock()
	}
}

// Pending returns the number of bytes waiting in the buffer.
func (wb *WriteBuffer) Pending() int {
	wb.mu.Lock()
	defer wb.mu.Unlock()
	return len(wb.buf)
}

// Close stops the drain goroutine and flushes remaining data.
func (wb *WriteBuffer) Close() error {
	wb.mu.Lock() // lint:manual-unlock — unlock before blocking on drain goroutine
	if wb.closed {
		wb.mu.Unlock()
		return nil
	}
	wb.closed = true
	// Close notify under mu, paired with Write's guarded send above, so the two
	// never race. drain receives the close without needing mu, so this cannot
	// deadlock; flushAll (which does take mu) only runs once mu is released below.
	close(wb.notify)
	wb.mu.Unlock()

	// Bound the wait. If drain is blocked inside writer.Write (the child stopped
	// reading stdin), close(notify) cannot wake it, so waiting unconditionally
	// would hang shutdown forever. On timeout, skip the final flush too — it would
	// re-block on the same stuck writer — and return the timeout so the caller can
	// log it. Reversing daemon shutdown order (StopAll before CloseAll) closes the
	// PTY first, which makes writer.Write return and this deadline essentially
	// never fires; the bound is defense-in-depth against any other writer stall.
	select {
	case <-wb.done:
		wb.flushAll() // drain exited cleanly — safe to do a final synchronous flush
		return nil
	case <-time.After(writeBufferCloseTimeout):
		// The drain goroutine is stuck inside writer.Write and cannot be safely
		// interrupted, so its goroutine + fd leak. Fire the diagnostics hook so the
		// leak is not silent (finding M) — Pending() is safe here (mu is released,
		// and flushAll releases mu around the blocked Write).
		fireWriteBufferCloseTimeout(wb.Pending())
		return ErrWriteBufferCloseTimeout
	}
}
