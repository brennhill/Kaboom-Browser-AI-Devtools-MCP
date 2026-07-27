// output.go — Human-facing diagnostic output reserved for stderr.
// Why: stdout belongs exclusively to framed MCP protocol traffic.

package diag

import (
	"fmt"
	"os"
)

// Print writes diagnostic text to stderr.
func Print(args ...any) {
	_, _ = fmt.Fprint(os.Stderr, args...)
}

// Printf writes formatted diagnostic text to stderr.
func Printf(format string, args ...any) {
	_, _ = fmt.Fprintf(os.Stderr, format, args...)
}

// Println writes a diagnostic line to stderr.
func Println(args ...any) {
	_, _ = fmt.Fprintln(os.Stderr, args...)
}
