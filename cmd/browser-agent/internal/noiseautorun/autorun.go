// autorun.go — Automatic noise detection scheduling and capture lifecycle wiring.

package noiseautorun

import (
	"os"
	"strings"
	"sync"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/diag"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/lifecycle"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/noise"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/util"
)

const (
	// EnvVar gates navigation-triggered automatic noise detection.
	EnvVar = "KABOOM_NOISE_AUTORUN"

	autoDetectInterval       = 30 * time.Second
	firstConnectDefaultDelay = 2 * time.Second
	firstConnectTestDelay    = 10 * time.Millisecond
)

// Enabled reports whether navigation-triggered detection is enabled.
func Enabled() bool {
	value := strings.ToLower(strings.TrimSpace(os.Getenv(EnvVar)))
	return value == "1" || value == "true" || value == "yes" || value == "on"
}

// FirstConnectDelay returns the capture warm-up delay for the current runtime.
func FirstConnectDelay() time.Duration {
	if strings.HasSuffix(os.Args[0], ".test") {
		return firstConnectTestDelay
	}
	return firstConnectDefaultDelay
}

// Runner coalesces detection requests within a minimum interval.
type Runner struct {
	mu       sync.Mutex
	run      func()
	interval time.Duration
	runtime  runnerRuntime
	lastRun  time.Time
	pending  bool
}

type runnerRuntime struct {
	now      func() time.Time
	dispatch func(func())
	delay    func(time.Duration, func())
}

func productionRunnerRuntime() runnerRuntime {
	return runnerRuntime{
		now:      time.Now,
		dispatch: util.SafeGo,
		delay: func(delay time.Duration, run func()) {
			util.SafeGo(func() {
				timer := time.NewTimer(delay)
				defer timer.Stop()
				<-timer.C
				run()
			})
		},
	}
}

// NewRunner constructs a debounced runner.
func NewRunner(run func(), interval time.Duration) *Runner {
	return newRunner(run, interval, productionRunnerRuntime())
}

func newRunner(run func(), interval time.Duration, runtime runnerRuntime) *Runner {
	return &Runner{run: run, interval: interval, runtime: runtime}
}

// Schedule requests a run, coalescing requests while one is pending.
func (runner *Runner) Schedule() {
	if runner.run == nil {
		return
	}
	immediate, delay, shouldRun := runner.plan(runner.runtime.now())
	if !shouldRun {
		return
	}
	if immediate {
		runner.runtime.dispatch(runner.execute)
		return
	}
	runner.runtime.delay(delay, runner.execute)
}

func (runner *Runner) plan(now time.Time) (immediate bool, delay time.Duration, shouldRun bool) {
	runner.mu.Lock()
	defer runner.mu.Unlock()
	if runner.pending {
		return false, 0, false
	}
	elapsed := now.Sub(runner.lastRun)
	runner.pending = true
	if elapsed >= runner.interval {
		return true, 0, true
	}
	return false, runner.interval - elapsed, true
}

func (runner *Runner) execute() {
	defer func() {
		runner.mu.Lock()
		defer runner.mu.Unlock()
		runner.lastRun = runner.runtime.now()
		runner.pending = false
	}()
	runner.run()
}

// WireNavigation installs debounced detection on navigation events when enabled.
func WireNavigation(store *capture.Capture, detect func()) {
	if store == nil || detect == nil || !Enabled() {
		return
	}
	runner := NewRunner(detect, autoDetectInterval)
	store.Telemetry().SetNavigationCallback(runner.Schedule)
	diag.Printf("[Kaboom] noise auto-detect enabled (triggers after navigation, debounce=%s)\n", autoDetectInterval)
}

// WireFirstConnect invokes detect once after the first extension connection.
func WireFirstConnect(store *capture.Capture, shutdown <-chan struct{}, detect func()) {
	wireFirstConnect(store, shutdown, detect, firstConnectRuntime{
		launch: util.SafeGo,
		after:  time.After,
	})
}

type firstConnectRuntime struct {
	launch func(func())
	after  func(time.Duration) <-chan time.Time
}

func wireFirstConnect(store *capture.Capture, shutdown <-chan struct{}, detect func(), runtime firstConnectRuntime) {
	if store == nil || detect == nil {
		return
	}
	var once sync.Once
	store.Lifecycle().Subscribe(func(event lifecycle.Event, _ map[string]any) {
		if event != lifecycle.EventExtensionConnected {
			return
		}
		once.Do(func() {
			runtime.launch(func() {
				select {
				case <-runtime.after(FirstConnectDelay()):
				case <-shutdown:
					return
				}
				detect()
			})
		})
	})
}

// Detect adapts captured telemetry to the canonical noise detector and applies
// only high-confidence proposals.
func Detect(config *noise.NoiseConfig, store *capture.Capture, logs []types.LogEntry) {
	if config == nil || store == nil {
		return
	}
	consoleEntries := make([]types.LogEntry, len(logs))
	for index, entry := range logs {
		consoleEntries[index] = types.LogEntry(entry)
	}
	// AutoDetect canonically applies high-confidence proposals. Do not add them a
	// second time here; duplicate application skews rule counts and diagnostics.
	proposals := config.AutoDetect(consoleEntries, store.Telemetry().NetworkBodies().Snapshot().Bodies, store.Telemetry().GetAllWebSocketEvents())
	if len(proposals) > 0 {
		diag.Printf("[Kaboom] noise auto-detect: %d proposals evaluated\n", len(proposals))
	}
}
