// beacon.go — Anonymous telemetry beacons. Disable with KABOOM_TELEMETRY=off.

package telemetry

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/incident"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/util"
)

// Version is set via ldflags at build time. Falls back to "dev" if unset.
var Version = "dev"

// beaconMu protects endpoint and llmName from concurrent access.
var beaconMu sync.RWMutex

// llmName is the MCP client name (e.g. "claude-code", "cursor").
var llmName string

// SetLLMName records which LLM client connected. Included in all subsequent beacons.
func SetLLMName(name string) {
	beaconMu.Lock()
	llmName = name
	beaconMu.Unlock()
}

// defaultEndpoint is the canonical telemetry ingest URL.
const defaultEndpoint = "https://t.gokaboom.dev/v1/event"

// endpoint is the telemetry ingest URL. Overridable for tests.
var endpoint = defaultEndpoint

// maxConcurrentBeacons caps in-flight beacon goroutines. Chosen to allow burst
// traffic (startup + first tool calls) without unbounded goroutine growth.
// A dropped beacon is harmless — telemetry is best-effort.
const maxConcurrentBeacons = 50

// maxPendingReliabilityEvents bounds incident projections waiting for the
// telemetry transport. Reliability telemetry is best-effort and must never
// create unbounded goroutines under a failure storm.
const maxPendingReliabilityEvents = 64
const reliabilityRateWindow = 5 * time.Minute

type ReliabilityDispatcherSnapshot struct {
	RateLimited uint64 `json:"rate_limited"`
	Saturated   uint64 `json:"saturated"`
	Panics      uint64 `json:"panics"`
	Pending     int64  `json:"pending"`
}

// sem caps the number of concurrent beacon goroutines to prevent runaway growth.
var sem = make(chan struct{}, maxConcurrentBeacons)

type reliabilityDispatcher struct {
	queue       chan incident.ReliabilityEvent
	deliver     func(incident.ReliabilityEvent)
	window      time.Duration
	now         func() time.Time
	mu          sync.Mutex
	lastByCode  map[incident.Code]time.Time
	once        sync.Once
	pending     sync.WaitGroup
	pendingNow  atomic.Int64
	rateLimited atomic.Uint64
	saturated   atomic.Uint64
	panics      atomic.Uint64
}

func newReliabilityDispatcher(capacity int, window time.Duration, now func() time.Time, deliver func(incident.ReliabilityEvent)) *reliabilityDispatcher {
	if capacity < 1 {
		capacity = 1
	}
	return &reliabilityDispatcher{queue: make(chan incident.ReliabilityEvent, capacity), deliver: deliver, window: window, now: now, lastByCode: make(map[incident.Code]time.Time)}
}

func (d *reliabilityDispatcher) Enqueue(event incident.ReliabilityEvent) bool {
	admittedAt, admitted := d.admit(event.Code)
	if !admitted {
		d.rateLimited.Add(1)
		deliveryCounters.suppressed.Add(1)
		callOnFireBeacon(false)
		return false
	}
	d.once.Do(func() {
		util.SafeGo(func() {
			for pending := range d.queue {
				d.deliverOne(pending)
			}
		})
	})
	d.pending.Add(1)
	d.pendingNow.Add(1)
	select {
	case d.queue <- event:
		return true
	default:
		d.releaseAdmission(event.Code, admittedAt)
		d.pending.Done()
		d.pendingNow.Add(-1)
		d.saturated.Add(1)
		deliveryCounters.dropped.Add(1)
		callOnFireBeacon(false)
		return false
	}
}

func (d *reliabilityDispatcher) admit(code incident.Code) (time.Time, bool) {
	if d.window <= 0 {
		return time.Time{}, true
	}
	now := d.now()
	d.mu.Lock()
	defer d.mu.Unlock()
	if last, ok := d.lastByCode[code]; ok && now.Sub(last) < d.window {
		return time.Time{}, false
	}
	d.lastByCode[code] = now
	return now, true
}

func (d *reliabilityDispatcher) releaseAdmission(code incident.Code, admittedAt time.Time) {
	if d.window <= 0 {
		return
	}
	d.mu.Lock()
	if d.lastByCode[code].Equal(admittedAt) {
		delete(d.lastByCode, code)
	}
	d.mu.Unlock()
}

func (d *reliabilityDispatcher) deliverOne(event incident.ReliabilityEvent) {
	defer func() {
		d.pendingNow.Add(-1)
		d.pending.Done()
	}()
	defer func() {
		if recover() != nil {
			deliveryCounters.dropped.Add(1)
			d.panics.Add(1)
			slog.Error("reliability event delivery panicked", "component", "telemetry_dispatcher", "stack", string(debug.Stack()))
		}
	}()
	d.deliver(event)
}

func (d *reliabilityDispatcher) WaitIdle() { d.pending.Wait() }

func (d *reliabilityDispatcher) Diagnostics() ReliabilityDispatcherSnapshot {
	return ReliabilityDispatcherSnapshot{RateLimited: d.rateLimited.Load(), Saturated: d.saturated.Load(), Panics: d.panics.Load(), Pending: d.pendingNow.Load()}
}

func (d *reliabilityDispatcher) resetForTest() {
	d.WaitIdle()
	d.mu.Lock()
	clear(d.lastByCode)
	d.mu.Unlock()
	d.rateLimited.Store(0)
	d.saturated.Store(0)
	d.panics.Store(0)
}

var reliabilityEvents = newReliabilityDispatcher(maxPendingReliabilityEvents, reliabilityRateWindow, time.Now, ReportReliability)

// beaconClient is a shared HTTP client for all beacons. Reuses connections.
var beaconClient = &http.Client{Timeout: 2 * time.Second}

// DeliverySnapshot exposes payload-free telemetry transport outcomes.
type DeliverySnapshot struct {
	Accepted      uint64                        `json:"accepted"`
	Rejected      uint64                        `json:"rejected"`
	NetworkErrors uint64                        `json:"network_errors"`
	Dropped       uint64                        `json:"dropped"`
	Suppressed    uint64                        `json:"suppressed"`
	LastStatus    int                           `json:"last_status"`
	Reliability   ReliabilityDispatcherSnapshot `json:"reliability"`
}

var deliveryCounters struct {
	accepted      atomic.Uint64
	rejected      atomic.Uint64
	networkErrors atomic.Uint64
	dropped       atomic.Uint64
	suppressed    atomic.Uint64
	lastStatus    atomic.Int64
}

// DeliveryDiagnostics returns aggregate transport outcomes without event data.
func DeliveryDiagnostics() DeliverySnapshot {
	return DeliverySnapshot{
		Accepted:      deliveryCounters.accepted.Load(),
		Rejected:      deliveryCounters.rejected.Load(),
		NetworkErrors: deliveryCounters.networkErrors.Load(),
		Dropped:       deliveryCounters.dropped.Load(),
		Suppressed:    deliveryCounters.suppressed.Load(),
		LastStatus:    int(deliveryCounters.lastStatus.Load()),
		Reliability:   reliabilityEvents.Diagnostics(),
	}
}

func resetDeliveryDiagnostics() {
	reliabilityEvents.resetForTest()
	deliveryCounters.accepted.Store(0)
	deliveryCounters.rejected.Store(0)
	deliveryCounters.networkErrors.Store(0)
	deliveryCounters.dropped.Store(0)
	deliveryCounters.suppressed.Store(0)
	deliveryCounters.lastStatus.Store(0)
}

// buildEnvelope returns the base fields included in every beacon.
// Only includes fields defined in the Counterscale contract shared envelope.
func buildEnvelope(event string) map[string]any {
	beaconMu.RLock()
	llm := llmName
	beaconMu.RUnlock()

	env := map[string]any{
		"event": event,
		"v":     Version,
		"os":    runtime.GOOS + "-" + runtime.GOARCH,
		"iid":   GetInstallID(),
		"sid":   GetSessionID(),
	}
	if llm != "" {
		env["llm"] = llm
	}
	return env
}

// AppError fires a structured app_error event from a fixed privacy-bounded schema.
func AppError(code incident.Code) {
	QueueReliability(incident.ReliabilityEvent{Code: code, Outcome: incident.OutcomePending, AttemptBucket: incident.AttemptZero})
}

// ReportReliability consumes the canonical privacy-safe incident projection.
// The current app_error wire contract records initial detection; recovery and
// retry aggregation remain local until the versioned reliability summary
// contract lands.
func ReportReliability(event incident.ReliabilityEvent) {
	if event.Outcome != incident.OutcomePending || event.AttemptBucket != incident.AttemptZero {
		return
	}
	definition, ok := reliabilityDefinition(event.Code)
	if !ok {
		return
	}
	fields := map[string]any{
		"event":      "app_error",
		"error_code": strings.ToUpper(string(event.Code)),
		"severity":   string(definition.Severity), "source": string(definition.Subsystem),
		"error_kind": string(definition.ErrorKind),
	}
	if definition.Retryable {
		fields["retryable"] = true
	}
	fireStructuredBeacon(fields)
}

func reliabilityDefinition(code incident.Code) (incident.Definition, bool) {
	definition, ok := incident.Lookup(code)
	if !ok {
		slog.Error("rejected unknown reliability event", "component", "telemetry_registry")
	}
	return definition, ok
}

// QueueReliability keeps analytics transport outside incident lifecycle locks
// and startup identity loading. This avoids recursively entering GetInstallID
// when the incident itself describes install-identity recovery.
func QueueReliability(event incident.ReliabilityEvent) {
	if _, ok := reliabilityDefinition(event.Code); !ok {
		return
	}
	reliabilityEvents.Enqueue(event)
}

// BeaconUsageSummary fires a structured usage_summary beacon.
func BeaconUsageSummary(windowMinutes int, snapshot *UsageSnapshot) {
	payload := BuildUsageSummaryPayload(windowMinutes, snapshot)
	if payload == nil {
		return
	}
	fireBeacon(payload)
}

// BuildUsageSummaryPayload builds the beacon payload without sending it.
// Used by debug endpoints to inspect what would be sent.
func BuildUsageSummaryPayload(windowMinutes int, snapshot *UsageSnapshot) map[string]any {
	if snapshot == nil {
		return nil
	}
	payload := buildEnvelope("usage_summary")
	payload["ts"] = time.Now().UTC().Format(time.RFC3339)
	payload["channel"] = Channel
	payload["window_m"] = windowMinutes
	payload["tool_stats"] = snapshot.ToolStats
	if len(snapshot.AsyncOutcomes) > 0 {
		payload["async_outcomes"] = snapshot.AsyncOutcomes
	}
	return payload
}

// telemetryOptedOut returns true if the user has disabled telemetry.
// Accepts KABOOM_TELEMETRY=off (case-insensitive).
func telemetryOptedOut() bool {
	return strings.EqualFold(os.Getenv("KABOOM_TELEMETRY"), "off")
}

// onFireBeaconMu protects the onFireBeacon test hook from concurrent access.
var onFireBeaconMu sync.Mutex

// onFireBeacon is a test hook called after fireBeacon decides to send or drop.
// When nil (production), no notification is sent. The bool arg is true if sent, false if dropped.
var onFireBeacon func(sent bool)

// setOnFireBeacon sets the test hook (use nil to clear).
func setOnFireBeacon(fn func(sent bool)) {
	onFireBeaconMu.Lock()
	onFireBeacon = fn
	onFireBeaconMu.Unlock()
}

func callOnFireBeacon(sent bool) {
	onFireBeaconMu.Lock()
	fn := onFireBeacon
	onFireBeaconMu.Unlock()
	if fn != nil {
		fn(sent)
	}
}

func fireBeacon(payload map[string]any) {
	if telemetryOptedOut() {
		deliveryCounters.suppressed.Add(1)
		callOnFireBeacon(false)
		return
	}
	if installID, _ := payload["iid"].(string); installID == "" {
		deliveryCounters.suppressed.Add(1)
		callOnFireBeacon(false)
		return
	}

	beaconMu.RLock()
	ep := endpoint
	beaconMu.RUnlock()
	testBinary := strings.HasSuffix(filepath.Base(os.Args[0]), ".test") ||
		strings.HasSuffix(filepath.Base(os.Args[0]), ".test.exe")
	if !shouldSendToEndpoint(ep, testBinary) {
		deliveryCounters.suppressed.Add(1)
		callOnFireBeacon(false)
		return
	}

	data, err := json.Marshal(payload)
	if err != nil {
		deliveryCounters.dropped.Add(1)
		callOnFireBeacon(false)
		return // best-effort
	}

	select {
	case sem <- struct{}{}:
		util.SafeGo(func() {
			defer func() { <-sem }()

			resp, err := beaconClient.Post(ep, "application/json", bytes.NewReader(data))
			if err != nil {
				deliveryCounters.networkErrors.Add(1)
				deliveryCounters.lastStatus.Store(0)
				callOnFireBeacon(false)
				return // best-effort
			}
			// Drain body before close so the HTTP transport can reuse the connection.
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
			deliveryCounters.lastStatus.Store(int64(resp.StatusCode))
			if resp.StatusCode == http.StatusAccepted {
				deliveryCounters.accepted.Add(1)
				callOnFireBeacon(true)
				return
			}
			deliveryCounters.rejected.Add(1)
			callOnFireBeacon(false)
		})
	default:
		// At capacity, drop this beacon silently
		deliveryCounters.dropped.Add(1)
		callOnFireBeacon(false)
	}
}

func shouldSendToEndpoint(ep string, testBinary bool) bool {
	return !testBinary || ep != defaultEndpoint
}

// overrideEndpoint sets a custom endpoint for testing.
func overrideEndpoint(url string) {
	// Test endpoints represent isolated telemetry runs; retain no rate-window
	// state from the preceding fixture.
	reliabilityEvents.resetForTest()
	beaconMu.Lock()
	endpoint = url
	beaconMu.Unlock()
}

// resetEndpoint restores the default endpoint after testing.
func resetEndpoint() {
	beaconMu.Lock()
	endpoint = defaultEndpoint
	beaconMu.Unlock()
}
