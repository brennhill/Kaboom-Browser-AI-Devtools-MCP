// native_install_connect.go — Post-install "did the extension connect?" loop.
// After the installer starts the daemon, poll /health until the browser
// extension connects, showing live status and a targeted hint on timeout. The
// loop's clock, fetch, and output sink are injected so it is unit-testable
// without a real daemon or real time.

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
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
	fetch func(port int) installHealth
	now   func() time.Time
	sleep func(time.Duration)
	sink  func(string) // progress narration; nil to stay silent
}

type connectResult struct {
	connected bool
	lastPhase string
}

// waitForExtensionConnected polls until the extension connects or the deadline
// passes, narrating each new phase exactly once via deps.sink.
func waitForExtensionConnected(port int, timeout, poll time.Duration, deps connectWaitDeps) connectResult {
	start := deps.now()
	rendered := ""
	for {
		phase := connectPhase(deps.fetch(port))
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
		deps.sleep(poll)
	}
}

// fetchInstallHealth reads /health once. Never errors — an unreachable daemon
// yields reachable=false; a non-200 or unparseable body counts as reachable.
func fetchInstallHealth(port int, timeout time.Duration) installHealth {
	client := &http.Client{Timeout: timeout}
	resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/health", port))
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
	stderrf("\n\033[1;33m⏳ Waiting for the browser extension to connect (Ctrl-C to skip)…\033[0m\n")
	res := waitForExtensionConnected(port, connectWaitTimeout, connectPollEvery, connectWaitDeps{
		fetch: func(p int) installHealth { return fetchInstallHealth(p, connectHealthRead) },
		now:   time.Now,
		sleep: time.Sleep,
		sink:  func(line string) { stderrf("%s\n", line) },
	})
	if res.connected {
		stderrf("\033[1;32m✅ Extension connected — Kaboom is fully wired up!\033[0m\n")
	} else {
		stderrf("\033[1;33m⚠️  %s\033[0m\n", connectHintLine(res.lastPhase, port, extDir))
	}
}
