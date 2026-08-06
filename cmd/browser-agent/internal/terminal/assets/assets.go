// static.go -- Embedded static assets for the in-browser terminal.
// Why: Bundles xterm.js and the terminal HTML page into the Go binary via go:embed,
// keeping the extension zero-dep while providing a full terminal emulator.

package assets

import "embed"

//go:embed terminal_assets
var FS embed.FS
