// Purpose: Debounces and triggers automatic noise detection on navigation events when KABOOM_NOISE_AUTORUN is enabled.
// Why: Coalesces rapid SPA route changes into a single noise-detection pass to avoid redundant analysis.
// Docs: docs/features/feature/noise-filtering/index.md

package main

import (
	"os"
	"strings"
	"sync"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/noise"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/util"
)

// noiseAutoDetectInterval is the minimum time between automatic noise detection runs.
// Navigation events closer together than this are coalesced into a single run.
const noiseAutoDetectInterval = 30 * time.Second

// noiseFirstConnectDefaultDelay allows initial capture buffers to warm before auto-detect.
const noiseFirstConnectDefaultDelay = 2 * time.Second

// noiseFirstConnectTestDelay keeps unit tests fast while preserving callback semantics.
const noiseFirstConnectTestDelay = 10 * time.Millisecond

// noiseAutoDetectEnvVar gates navigation-triggered noise auto-detection.
// Default is off; set to 1/true/on/yes to enable.
const noiseAutoDetectEnvVar = "KABOOM_NOISE_AUTORUN"

func noiseFirstConnectDelay() time.Duration {
	if strings.HasSuffix(os.Args[0], ".test") {
		return noiseFirstConnectTestDelay
	}
	return noiseFirstConnectDefaultDelay
}

// noiseAutoRunner debounces automatic noise detection. Multiple rapid navigation
// events (e.g., SPA route changes) are coalesced: only one auto-detect runs per
// debounce window. Thread-safe.
type noiseAutoRunner struct {
	mu       sync.Mutex
	fn       func()
	interval time.Duration
	lastRun  time.Time
	pending  bool
}

// newNoiseAutoRunner creates a debounced runner for the given function.
// The interval is the minimum time between runs.
func newNoiseAutoRunner(fn func(), interval time.Duration) *noiseAutoRunner {
	return &noiseAutoRunner{
		fn:       fn,
		interval: interval,
	}
}

// schedule requests that the function run after the debounce interval elapses.
// If a run is already scheduled, this is a no-op (coalesced).
// Thread-safe: may be called from any goroutine.
func (r *noiseAutoRunner) schedule() {
	if r.fn == nil {
		return
	}

	immediate, delay, shouldRun := r.planRunSchedule(time.Now())
	if !shouldRun {
		return
	}
	if immediate {
		// Enough time has passed — run immediately in background.
		util.SafeGo(r.run)
		return
	}

	// Schedule for after the remaining debounce period.
	util.SafeGo(func() {
		time.Sleep(delay)
		r.run()
	})
}

func (r *noiseAutoRunner) planRunSchedule(now time.Time) (immediate bool, delay time.Duration, shouldRun bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.pending {
		return false, 0, false
	}

	elapsed := now.Sub(r.lastRun)
	r.pending = true
	if elapsed >= r.interval {
		return true, 0, true
	}
	return false, r.interval - elapsed, true
}

// run executes the function and resets the debounce state.
func (r *noiseAutoRunner) run() {
	r.fn()

	r.mu.Lock()
	defer r.mu.Unlock()
	r.lastRun = time.Now()
	r.pending = false
}

// wireNoiseAutoDetect connects automatic noise detection to navigation events.
// Called once during NewToolHandler initialization.
func wireNoiseAutoDetect(h *ToolHandler) {
	if !noiseAutoDetectEnabled() {
		return
	}
	if h.capture == nil || h.noiseConfig == nil {
		return
	}

	runner := newNoiseAutoRunner(func() {
		h.runNoiseAutoDetect()
	}, noiseAutoDetectInterval)

	h.capture.SetNavigationCallback(func() {
		runner.schedule()
	})

	stderrf("[Kaboom] noise auto-detect enabled (triggers after navigation, debounce=%s)\n", noiseAutoDetectInterval)
}

// wireNoiseFirstConnect sets up a lifecycle callback to run noise auto-detection
// once on the first extension connection. This ensures sessions start with common
// noise patterns suppressed without requiring the LLM to call auto_detect.
// Issue #264: Auto-detect noise rules on first connection.
//
// Implementation: chains with any existing lifecycle callback instead of replacing it.
func wireNoiseFirstConnect(h *ToolHandler) {
	if h.capture == nil || h.noiseConfig == nil {
		return
	}

	var once sync.Once

	h.capture.AddLifecycleCallback(func(event string, data map[string]any) {
		if event != "extension_connected" {
			return
		}
		once.Do(func() {
			delay := noiseFirstConnectDelay()
			// Small delay to let the first batch of logs/network data arrive
			// before running auto-detection, so there's data to analyze.
			// Respects shutdownCtx so the goroutine exits promptly on server shutdown.
			util.SafeGo(func() {
				select {
				case <-time.After(delay):
				case <-h.shutdownCtx.Done():
					return
				}
				fn := h.noiseFirstConnectFn
				if fn != nil {
					fn()
					return
				}
				h.runNoiseAutoDetect()
				stderrf("[Kaboom] noise auto-detect: ran on first extension connection\n")
			})
		})
	})
}

func noiseAutoDetectEnabled() bool {
	val := strings.ToLower(strings.TrimSpace(os.Getenv(noiseAutoDetectEnvVar)))
	return val == "1" || val == "true" || val == "yes" || val == "on"
}

// IsConsoleNoise delegates to the noise config to check if a log entry is noise.
// Satisfies mcp.NoiseFilterer. Returns false if noise config is nil.
func (h *ToolHandler) IsConsoleNoise(entry map[string]any) bool {
	if h.noiseConfig == nil {
		return false
	}
	return h.noiseConfig.IsConsoleNoise(entry)
}

// runNoiseAutoDetect collects current buffer data and runs noise auto-detection.
// This is the same logic as noiseActionAutoDetect() but designed for background use.
func (h *ToolHandler) runNoiseAutoDetect() {
	logEntries := h.server.logs.Entries()
	consoleEntries := make([]noise.LogEntry, len(logEntries))
	for i, e := range logEntries {
		consoleEntries[i] = noise.LogEntry(e)
	}

	networkBodies := h.capture.GetNetworkBodies()
	wsEvents := h.capture.GetAllWebSocketEvents()

	proposals := h.noiseConfig.AutoDetect(consoleEntries, networkBodies, wsEvents)
	if len(proposals) > 0 {
		var toApply []noise.NoiseRule
		for _, p := range proposals {
			if p.Confidence >= 0.9 {
				toApply = append(toApply, p.Rule)
			}
		}
		if len(toApply) > 0 {
			_ = h.noiseConfig.AddRules(toApply)
		}
		stderrf("[Kaboom] noise auto-detect: %d proposals, %d auto-applied\n", len(proposals), len(toApply))
	}
}
