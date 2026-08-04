// beacon.go — Anonymous telemetry beacons. Disable with KABOOM_TELEMETRY=off.

package telemetry

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
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

// sem caps the number of concurrent beacon goroutines to prevent runaway growth.
var sem = make(chan struct{}, maxConcurrentBeacons)

type reliabilityDispatcher struct {
	queue   chan incident.ReliabilityEvent
	deliver func(incident.ReliabilityEvent)
	once    sync.Once
}

func newReliabilityDispatcher(capacity int, deliver func(incident.ReliabilityEvent)) *reliabilityDispatcher {
	if capacity < 1 {
		capacity = 1
	}
	return &reliabilityDispatcher{queue: make(chan incident.ReliabilityEvent, capacity), deliver: deliver}
}

func (d *reliabilityDispatcher) Enqueue(event incident.ReliabilityEvent) bool {
	d.once.Do(func() {
		util.SafeGo(func() {
			for pending := range d.queue {
				d.deliver(pending)
			}
		})
	})
	select {
	case d.queue <- event:
		return true
	default:
		return false
	}
}

var reliabilityEvents = newReliabilityDispatcher(maxPendingReliabilityEvents, ReportReliability)

// beaconClient is a shared HTTP client for all beacons. Reuses connections.
var beaconClient = &http.Client{Timeout: 2 * time.Second}

// DeliverySnapshot exposes payload-free telemetry transport outcomes.
type DeliverySnapshot struct {
	Accepted      uint64 `json:"accepted"`
	Rejected      uint64 `json:"rejected"`
	NetworkErrors uint64 `json:"network_errors"`
	Dropped       uint64 `json:"dropped"`
	Suppressed    uint64 `json:"suppressed"`
	LastStatus    int    `json:"last_status"`
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
	}
}

func resetDeliveryDiagnostics() {
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
func AppError(category string) {
	errorKind, severity, source, retryable := classifyAppError(category)

	fields := map[string]any{
		"event":      "app_error",
		"error_kind": errorKind,
		"error_code": normalizeAppErrorCode(category),
		"severity":   severity,
		"source":     source,
	}
	if retryable {
		fields["retryable"] = true
	}
	fireStructuredBeacon(fields)
}

// ReportReliability consumes the canonical privacy-safe incident projection.
// The current app_error wire contract records initial detection; recovery and
// retry aggregation remain local until the versioned reliability summary
// contract lands.
func ReportReliability(event incident.ReliabilityEvent) {
	if event.Outcome != incident.OutcomePending || event.AttemptBucket != incident.AttemptZero {
		return
	}
	fireStructuredBeacon(map[string]any{
		"event": "app_error", "error_kind": "internal",
		"error_code": strings.ToUpper(string(event.Code)),
		"severity":   string(event.Severity), "source": string(event.Subsystem),
		"retryable": event.Retryable,
	})
}

// QueueReliability keeps analytics transport outside incident lifecycle locks
// and startup identity loading. This avoids recursively entering GetInstallID
// when the incident itself describes install-identity recovery.
func QueueReliability(event incident.ReliabilityEvent) {
	if reliabilityEvents.Enqueue(event) {
		return
	}
	deliveryCounters.dropped.Add(1)
	callOnFireBeacon(false)
}

func classifyAppError(category string) (errorKind string, severity string, source string, retryable bool) {
	switch category {
	case "daemon_panic":
		return "internal", "fatal", "daemon", false
	case "daemon_start_failed":
		return "internal", "fatal", "startup", false
	case "tool_rate_limited":
		return "integration", "warning", "daemon", true
	case "bridge_connection_error":
		return "integration", "error", "bridge", true
	case "bridge_port_blocked":
		return "integration", "error", "bridge", false
	case "bridge_spawn_build_error", "bridge_spawn_start_error":
		return "internal", "fatal", "bridge", false
	case "bridge_spawn_timeout":
		return "internal", "error", "bridge", true
	case "bridge_exit_error":
		return "internal", "error", "bridge", false
	case "extension_disconnect":
		return "integration", "warning", "extension", false
	case "install_config_error":
		return "internal", "error", "installer", false
	default:
		return "unknown", "error", "daemon", false
	}
}

func normalizeAppErrorCode(category string) string {
	replacer := strings.NewReplacer("-", "_", " ", "_")
	return strings.ToUpper(replacer.Replace(strings.TrimSpace(category)))
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
