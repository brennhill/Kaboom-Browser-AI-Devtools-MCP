// writebuf.go — Non-blocking write buffer with backpressure for PTY input.
// Why: Prevents WebSocket handlers from blocking when the child process stalls on stdin.

package pty

import (
	"errors"
	"io"
	"sync"
)

// Write buffer constants.
const (
	writeBufferMax = 1 << 20   // 1 MB backpressure cap.
	writeChunkSize = 16 * 1024 // 16 KB per write syscall.
)

// ErrWriteBufferFull is returned when the write buffer exceeds the backpressure cap.
var ErrWriteBufferFull = errors.New("pty: write buffer full")

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
	go wb.drain() // lint:allow-bare-goroutine — long-lived drain loop, exits on Close
	return wb
}

// Write appends data to the buffer without blocking. Returns ErrWriteBufferFull
// if the buffer exceeds the backpressure cap.
func (wb *WriteBuffer) Write(data []byte) (int, error) {
	wb.mu.Lock() // lint:manual-unlock — multiple early-return paths
	if wb.closed {
		wb.mu.Unlock()
		return 0, ErrWriteBufferFull
	}
	if len(wb.buf)+len(data) > wb.maxSize {
		wb.mu.Unlock()
		return 0, ErrWriteBufferFull
	}
	wb.buf = append(wb.buf, data...)
	wb.mu.Unlock()
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
	wb.mu.Unlock()

	close(wb.notify)
	<-wb.done

	// Final synchronous flush.
	wb.flushAll()
	return nil
}
