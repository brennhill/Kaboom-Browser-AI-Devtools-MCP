// native_install_connect.go — Post-install "did the extension connect?" loop.
// After the installer starts the daemon, poll /health until the browser
// extension connects, showing live status and a targeted hint on timeout. The
// loop's clock, fetch, and output sink are injected so it is unit-testable
// without a real daemon or real time.

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"time"
)

const (
	connectWaitTimeout = 30 * time.Second
	connectPollEvery   = 750 * time.Millisecond
	connectHealthRead  = 800 * time.Millisecond
)

// installHealth is the slice of /health the connect loop cares about.
type installHealth struct {
	reachable          bool
	extensionConnected bool
	version            string
}

// connectPhase classifies a health snapshot into a connect phase.
func connectPhase(h installHealth) string {
	if h.extensionConnected {
		return "connected"
	}
	if h.reachable {
		return "waiting_extension"
	}
	return "daemon_unreachable"
}

// connectProgressLine is the one-time line shown when the loop enters a phase,
// or "" for phases that need no narration (connected has its own final line).
func connectProgressLine(phase string) string {
	switch phase {
	case "daemon_unreachable":
		return "   … waiting for the Kaboom server to come up"
	case "waiting_extension":
		return "   … server is up — load the extension in your browser to finish"
	default:
		return ""
	}
}

// connectHintLine is the targeted next step when the loop times out. Pure.
func connectHintLine(lastPhase string, port int, extDir string) string {
	if lastPhase == "waiting_extension" {
		return fmt.Sprintf(
			"The Kaboom server is running on port %d, but the extension has not connected yet.\n"+
				"   Open chrome://extensions, enable Developer mode, click Load unpacked, and select:\n   %s",
			port, extDir)
	}
	return fmt.Sprintf(
		"The Kaboom server is not answering on port %d yet.\n"+
			"   Re-run the installer, or start Kaboom manually, then load the extension.",
		port)
}

// connectWaitDeps are the injectable seams for waitForExtensionConnected.
type connectWaitDeps struct {
	fetch func(ctx context.Context, port int) installHealth
	now   func() time.Time
	after func(time.Duration) <-chan time.Time // inter-poll wait; defaults to time.After
	sink  func(string)                         // progress narration; nil to stay silent
}

type connectResult struct {
	connected bool
	aborted   bool
	lastPhase string
}

// waitForExtensionConnected polls until the extension connects, the deadline
// passes, or ctx is cancelled (Ctrl-C "skip"), narrating each new phase exactly
// once via deps.sink. ctx is honored promptly: it is checked at the loop top and
// again during the inter-poll wait (via a select on ctx.Done()), and it is
// threaded into each /health fetch so an in-flight poll cancels immediately.
func waitForExtensionConnected(ctx context.Context, port int, timeout, poll time.Duration, deps connectWaitDeps) connectResult {
	start := deps.now()
	rendered := ""
	for {
		if ctx.Err() != nil {
			return connectResult{aborted: true, lastPhase: rendered}
		}
		phase := connectPhase(deps.fetch(ctx, port))
		if phase != rendered {
			rendered = phase
			if deps.sink != nil {
				if line := connectProgressLine(phase); line != "" {
					deps.sink(line)
				}
			}
		}
		if phase == "connected" {
			return connectResult{connected: true, lastPhase: phase}
		}
		if deps.now().Sub(start) >= timeout {
			return connectResult{connected: false, lastPhase: phase}
		}
		// Interruptible inter-poll wait: a Ctrl-C during the wait returns the
		// aborted result immediately rather than after the next poll cycle.
		select {
		case <-ctx.Done():
			return connectResult{aborted: true, lastPhase: rendered}
		case <-deps.after(poll):
		}
	}
}

// fetchInstallHealth reads /health once. Never errors — an unreachable daemon
// (or a cancelled ctx) yields reachable=false; a non-200 or unparseable body
// counts as reachable. ctx is threaded into the request so an in-flight poll
// cancels immediately on Ctrl-C; the per-request timeout still bounds a hung read.
func fetchInstallHealth(ctx context.Context, port int, timeout time.Duration) installHealth {
	client := &http.Client{Timeout: timeout}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("http://127.0.0.1:%d/health", port), nil)
	if err != nil {
		return installHealth{}
	}
	resp, err := client.Do(req)
	if err != nil {
		return installHealth{}
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return installHealth{reachable: true}
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 256*1024))
	if err != nil {
		return installHealth{reachable: true}
	}
	var parsed struct {
		Version string `json:"version"`
		Capture struct {
			ExtensionConnected bool `json:"extension_connected"`
		} `json:"capture"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return installHealth{reachable: true}
	}
	return installHealth{reachable: true, extensionConnected: parsed.Capture.ExtensionConnected, version: parsed.Version}
}

// installWaitDisabled reports whether the user opted out of the connect wait.
func installWaitDisabled() bool {
	return envFlagEnabled("KABOOM_NO_WAIT", "KABOOM_INSTALL_NO_WAIT")
}

// isTerminal reports whether f is an interactive terminal (stdlib only, so the
// zero-deps rule holds). Used to skip the blocking wait for piped/CI installs.
func isTerminal(f *os.File) bool {
	if f == nil {
		return false
	}
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

// runExtensionConnectWait runs the connect loop against the real daemon and
// prints progress + outcome. Skipped for opted-out or non-interactive installs
// (the daemon is already running either way).
func runExtensionConnectWait(port int, extDir string) {
	if installWaitDisabled() || !isTerminal(os.Stderr) {
		return
	}
	// Ctrl-C cancels the wait (a real "skip"), observed promptly: the
	// inter-poll wait selects on ctx.Done() and ctx is threaded into each
	// /health fetch, so an in-flight poll or wait aborts at once. NotifyContext
	// suppresses the default SIGINT exit until the deferred stop() runs, so
	// default Ctrl-C behavior is restored only once this function returns — a
	// second Ctrl-C during the (short) abort window is absorbed, not a force-quit.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	stderrf("\n\033[1;33m⏳ Waiting for the browser extension to connect (Ctrl-C to skip)…\033[0m\n")
	res := waitForExtensionConnected(ctx, port, connectWaitTimeout, connectPollEvery, connectWaitDeps{
		fetch: func(c context.Context, p int) installHealth { return fetchInstallHealth(c, p, connectHealthRead) },
		now:   time.Now,
		after: time.After,
		sink:  func(line string) { stderrf("%s\n", line) },
	})
	if res.connected {
		stderrf("\033[1;32m✅ Extension connected — Kaboom is fully wired up!\033[0m\n")
	} else if res.aborted {
		stderrf("\033[1;33m⏭️  Skipped waiting — the Kaboom server is running; load the extension and it will connect.\033[0m\n")
	} else {
		stderrf("\033[1;33m⚠️  %s\033[0m\n", connectHintLine(res.lastPhase, port, extDir))
	}
}
