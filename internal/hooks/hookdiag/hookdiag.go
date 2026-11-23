// Purpose: Emits the hook's one-line stderr diagnostics.
// Why: The line is a parsed contract, so its shape has exactly one owner.
// Docs: docs/features/feature/quality-gates/index.md

package hookdiag

import (
	"fmt"
	"io"
	"os"
)

// Emit reports a failure the hook recovered from, on stderr.
//
// Stderr and not stdout: hooks own stdout, where the agent reads the JSON
// protocol response, so a diagnostic written there would corrupt the document the
// agent is parsing.
func Emit(code string) {
	EmitTo(os.Stderr, code)
}

// EmitTo writes one diagnostic record to w.
//
// Codes are short identifiers, never messages: a caller's error text can carry a
// path, and a path can carry a token or a customer name. Quoting through %q also
// keeps a code containing a quote or a newline from ending the JSON string early
// and splitting one failure into two records, the second unparseable.
func EmitTo(w io.Writer, code string) {
	// TERMINAL_LOG_SINK: if the local stderr sink itself is unavailable, there
	// is no second local channel that can report that failure without recursion.
	_, _ = fmt.Fprintf(w, "{\"kaboom_hook_diagnostic\":%q}\n", code)
}
