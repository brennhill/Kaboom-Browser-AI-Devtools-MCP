// output.go — Human-facing diagnostic output reserved for stderr.
// Why: stdout belongs exclusively to framed MCP protocol traffic.

package diag

import (
	"fmt"
	"io"
	"os"
	"sync"
)

var (
	sinkMu sync.Mutex
	sink   io.Writer = os.Stderr
)

// SetSink redirects diagnostics. Nil leaves the current sink unchanged.
func SetSink(writer io.Writer) {
	if writer == nil {
		return
	}
	sinkMu.Lock()
	sink = writer
	sinkMu.Unlock()
}

// Sink returns the current diagnostic writer.
func Sink() io.Writer {
	sinkMu.Lock()
	defer sinkMu.Unlock()
	return sink
}

// Print writes diagnostic text to stderr.
func Print(args ...any) {
	sinkMu.Lock()
	defer sinkMu.Unlock()
	_, _ = fmt.Fprint(sink, args...)
}

// Printf writes formatted diagnostic text to stderr.
func Printf(format string, args ...any) {
	sinkMu.Lock()
	defer sinkMu.Unlock()
	_, _ = fmt.Fprintf(sink, format, args...)
}

// Println writes a diagnostic line to stderr.
func Println(args ...any) {
	sinkMu.Lock()
	defer sinkMu.Unlock()
	_, _ = fmt.Fprintln(sink, args...)
}
