// pagescripts.go — MAIN-world page scripts the daemon queues for interact actions.
// Why: each script lives in its own .js file so the exact bytes the browser runs are
// executable in Node fixtures, instead of being a Go string only assertable by substring.
// Docs: docs/features/feature/interact-explore/index.md

package pagescripts

import _ "embed"

// ClipboardRead is the bounded, self-classifying clipboard read.
// It decides from the Permissions API before touching navigator.clipboard, so a
// prompt-state origin can never strand the session behind an unanswerable modal,
// and every failure names its cause instead of a generic execution_timeout.
//
//go:embed clipboard-read.js
var ClipboardRead string
