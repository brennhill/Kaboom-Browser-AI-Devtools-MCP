// beacon_test.go — Tests for anonymous telemetry beacons.
// Tests in this package must NOT use t.Parallel() due to shared package-level state
// (endpoint, llmName, sem, session, installID). Refactoring to an injectable Beacon
// struct would unlock parallelism — tracked as design debt, not a correctness issue.

package telemetry

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/incident"
)

func TestReliabilityDispatcherBoundsPendingWork(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	var delivered atomic.Uint64
	dispatcher := newReliabilityDispatcher(2, 0, time.Now, func(incident.ReliabilityEvent) {
		if delivered.Add(1) == 1 {
			close(entered)
		}
		<-release
	})
	event := incident.ReliabilityEvent{Code: incident.CodeStateRecoveryFailed}
	if !dispatcher.Enqueue(event) {
		t.Fatal("first reliability event was dropped")
	}
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("dispatcher did not begin delivery")
	}
	if !dispatcher.Enqueue(event) || !dispatcher.Enqueue(event) {
		t.Fatal("bounded pending queue rejected available capacity")
	}
	if dispatcher.Enqueue(event) {
		t.Fatal("dispatcher accepted work beyond its fixed capacity")
	}
	close(release)
	dispatcher.WaitIdle()
}

func TestReliabilityDispatcherSurvivesDeliveryPanic(t *testing.T) {
	delivered := make(chan struct{}, 1)
	var calls atomic.Uint64
	dispatcher := newReliabilityDispatcher(2, 0, time.Now, func(incident.ReliabilityEvent) {
		if calls.Add(1) == 1 {
			panic("synthetic delivery failure")
		}
		delivered <- struct{}{}
	})
	event := incident.ReliabilityEvent{Code: incident.CodeStateRecoveryFailed}
	if !dispatcher.Enqueue(event) || !dispatcher.Enqueue(event) {
		t.Fatal("dispatcher unexpectedly rejected work")
	}

	select {
	case <-delivered:
	case <-time.After(time.Second):
		t.Fatal("delivery worker did not continue after a panicking delivery")
	}
	done := make(chan struct{})
	go func() {
		dispatcher.WaitIdle()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("panicking delivery stranded pending work")
	}
}

func TestReliabilityDispatcherRateLimitsPerCodeWithoutBlocking(t *testing.T) {
	resetDeliveryDiagnostics()
	now := time.Date(2026, 8, 4, 10, 0, 0, 0, time.UTC)
	delivered := make(chan incident.ReliabilityEvent, 2)
	dispatcher := newReliabilityDispatcher(2, time.Minute, func() time.Time { return now }, func(event incident.ReliabilityEvent) {
		delivered <- event
	})
	event := incident.ReliabilityEvent{Code: incident.CodeStateRecoveryFailed}
	if !dispatcher.Enqueue(event) {
		t.Fatal("first event was rejected")
	}
	if dispatcher.Enqueue(event) {
		t.Fatal("duplicate event inside its rate window was accepted")
	}
	now = now.Add(time.Minute)
	if !dispatcher.Enqueue(event) {
		t.Fatal("event after its rate window was rejected")
	}
	dispatcher.WaitIdle()
	if got := len(delivered); got != 2 {
		t.Fatalf("delivered %d events, want 2", got)
	}
	stats := dispatcher.Diagnostics()
	if stats.RateLimited != 1 || stats.Saturated != 0 || stats.Pending != 0 {
		t.Fatalf("dispatcher diagnostics = %+v", stats)
	}
	if got := DeliveryDiagnostics(); got.Suppressed != 1 || got.Dropped != 0 {
		t.Fatalf("delivery diagnostics = %+v, want one intentional suppression", got)
	}
}

func TestReliabilityDispatcherDoesNotCollapseDistinctTransitionBuckets(t *testing.T) {
	delivered := make(chan incident.ReliabilityEvent, 2)
	dispatcher := newReliabilityDispatcher(2, time.Minute, time.Now, func(event incident.ReliabilityEvent) { delivered <- event })
	detected := incident.ReliabilityEvent{Code: incident.CodeStateRecoveryFailed, Outcome: incident.OutcomePending, AttemptBucket: incident.AttemptZero}
	recovered := incident.ReliabilityEvent{Code: incident.CodeStateRecoveryFailed, Outcome: incident.OutcomeRecovered, AttemptBucket: incident.AttemptOne, LatencyBucket: incident.LatencyOneToFive}
	if !dispatcher.Enqueue(detected) || !dispatcher.Enqueue(recovered) {
		t.Fatal("distinct lifecycle transitions were incorrectly rate limited")
	}
	dispatcher.WaitIdle()
	if got := len(delivered); got != 2 {
		t.Fatalf("delivered %d transitions, want 2", got)
	}
}

func TestReliabilityDispatcherClockRollbackStartsFreshWindow(t *testing.T) {
	now := time.Date(2026, 8, 4, 10, 0, 0, 0, time.UTC)
	dispatcher := newReliabilityDispatcher(2, time.Minute, func() time.Time { return now }, func(incident.ReliabilityEvent) {})
	event := incident.ReliabilityEvent{Code: incident.CodeStateRecoveryFailed}
	if !dispatcher.Enqueue(event) {
		t.Fatal("first event was rejected")
	}
	dispatcher.WaitIdle()
	now = now.Add(-time.Hour)
	if !dispatcher.Enqueue(event) {
		t.Fatal("clock rollback incorrectly extended the old rate window")
	}
	dispatcher.WaitIdle()
}

func TestReliabilityDispatcherSaturationDoesNotConsumeRateWindow(t *testing.T) {
	resetDeliveryDiagnostics()
	entered := make(chan struct{})
	release := make(chan struct{})
	dispatcher := newReliabilityDispatcher(1, time.Minute, time.Now, func(event incident.ReliabilityEvent) {
		if event.Code == incident.CodeDaemonPanic {
			close(entered)
			<-release
		}
	})
	if !dispatcher.Enqueue(incident.ReliabilityEvent{Code: incident.CodeDaemonPanic}) {
		t.Fatal("active event was rejected")
	}
	<-entered
	if !dispatcher.Enqueue(incident.ReliabilityEvent{Code: incident.CodeBridgeConnectionError}) {
		t.Fatal("queued event was rejected")
	}
	target := incident.ReliabilityEvent{Code: incident.CodeStateRecoveryFailed}
	if dispatcher.Enqueue(target) {
		t.Fatal("event beyond queue capacity was accepted")
	}
	close(release)
	dispatcher.WaitIdle()
	if !dispatcher.Enqueue(target) {
		t.Fatal("saturated event incorrectly consumed its rate window")
	}
	dispatcher.WaitIdle()
	if got := dispatcher.Diagnostics(); got.Saturated != 1 || got.Pending != 0 {
		t.Fatalf("dispatcher diagnostics = %+v", got)
	}
	if got := DeliveryDiagnostics(); got.Dropped != 1 || got.Suppressed != 0 {
		t.Fatalf("delivery diagnostics = %+v, want one pressure drop", got)
	}
}

func emitTestSessionStart() {
	fireStructuredBeacon(map[string]any{
		"event":  "session_start",
		"reason": "startup",
	})
}

func TestShouldSendToEndpoint_BlocksProductionFromTestBinaries(t *testing.T) {
	if shouldSendToEndpoint(defaultEndpoint, true) {
		t.Fatal("test binary must never send to the production telemetry endpoint")
	}
	if !shouldSendToEndpoint("http://127.0.0.1:12345", true) {
		t.Fatal("test binary must still be able to exercise an explicitly local endpoint")
	}
	if !shouldSendToEndpoint(defaultEndpoint, false) {
		t.Fatal("production binary must be able to send to the production endpoint")
	}
}

func TestBeaconSuppressesDeliveryWithoutStableInstallIdentity(t *testing.T) {
	drainSem()
	resetDeliveryDiagnostics()
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		calls++
	}))
	defer server.Close()
	overrideEndpoint(server.URL)
	defer resetEndpoint()

	fireBeacon(map[string]any{"event": "session_start", "iid": ""})
	got := DeliveryDiagnostics()
	if calls != 0 || got.Suppressed != 1 {
		t.Fatalf("delivery = calls:%d diagnostics:%+v, want suppressed", calls, got)
	}
}

func TestBeaconDeliveryRequiresAcceptedStatus(t *testing.T) {
	drainSem()
	resetDeliveryDiagnostics()

	statuses := make(chan int, 2)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(<-statuses)
	}))
	defer srv.Close()
	overrideEndpoint(srv.URL)
	defer resetEndpoint()

	statuses <- http.StatusOK
	fireStructuredBeacon(map[string]any{
		"event":  "session_start",
		"reason": "startup",
	})
	waitForDeliveryCount(t, 1)

	first := DeliveryDiagnostics()
	if first.Accepted != 0 || first.Rejected != 1 || first.LastStatus != http.StatusOK {
		t.Fatalf("200 delivery diagnostics = %+v, want one rejection", first)
	}

	statuses <- http.StatusAccepted
	fireStructuredBeacon(map[string]any{
		"event":  "session_start",
		"reason": "startup",
	})
	waitForDeliveryCount(t, 2)

	second := DeliveryDiagnostics()
	if second.Accepted != 1 || second.Rejected != 1 || second.LastStatus != http.StatusAccepted {
		t.Fatalf("202 delivery diagnostics = %+v, want one accepted and one rejected", second)
	}
}

func TestBeaconDeliveryRecordsNetworkFailureWithoutPayload(t *testing.T) {
	drainSem()
	resetDeliveryDiagnostics()
	overrideEndpoint("http://127.0.0.1:1")
	defer resetEndpoint()

	fireStructuredBeacon(map[string]any{
		"event":  "session_start",
		"reason": "startup",
	})
	waitForDeliveryCount(t, 1)

	got := DeliveryDiagnostics()
	if got.NetworkErrors != 1 || got.LastStatus != 0 {
		t.Fatalf("network failure diagnostics = %+v", got)
	}
}

func waitForDeliveryCount(t *testing.T, want uint64) {
	t.Helper()
	waitForBeaconDeliveryIdle()
	got := DeliveryDiagnostics()
	if got.Accepted+got.Rejected+got.NetworkErrors+got.Dropped+got.Suppressed < want {
		t.Fatalf("got fewer than %d delivery decisions: %+v", want, got)
	}
}

func TestWaitForBeaconDeliveryIdleTracksScheduledRequests(t *testing.T) {
	requestStarted := make(chan struct{})
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		close(requestStarted)
		<-release
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()
	overrideEndpoint(srv.URL)
	defer resetEndpoint()

	emitTestSessionStart()
	<-requestStarted
	idle := make(chan struct{})
	go func() {
		waitForBeaconDeliveryIdle()
		close(idle)
	}()
	select {
	case <-idle:
		t.Fatal("delivery reported idle while its HTTP request was still blocked")
	default:
	}
	close(release)
	select {
	case <-idle:
	case <-time.After(time.Second):
		t.Fatal("delivery did not report idle after the request completed")
	}
}

func TestBeacon_DisabledByEnv(t *testing.T) {
	t.Setenv("KABOOM_TELEMETRY", "off")

	fired := make(chan bool, 1)
	setOnFireBeacon(func(sent bool) {
		select {
		case fired <- sent:
		default:
		}
	})
	defer setOnFireBeacon(nil)

	var called bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	overrideEndpoint(srv.URL)
	defer resetEndpoint()

	emitTestSessionStart()

	select {
	case sent := <-fired:
		if sent {
			t.Fatal("beacon was sent despite KABOOM_TELEMETRY=off")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("onFireBeacon hook not called")
	}

	if called {
		t.Fatal("HTTP endpoint should not have been called when KABOOM_TELEMETRY=off")
	}
}

func TestBeacon_FireAndForget(t *testing.T) {
	// Verify canonical event emission returns immediately when unreachable.
	// Use a non-routable address so the goroutine doesn't linger on other test servers.
	overrideEndpoint("http://198.51.100.1:1") // TEST-NET-2, non-routable
	defer resetEndpoint()

	start := time.Now()
	emitTestSessionStart()
	elapsed := time.Since(start)

	if elapsed > 100*time.Millisecond {
		t.Fatalf("event emission blocked for %v, expected fire-and-forget", elapsed)
	}
}

func TestBeacon_IgnoresHTTPFailure(t *testing.T) {
	drainSem() // ensure clean semaphore from prior tests
	// Point at a closed server — should not panic or block the caller.
	overrideEndpoint("http://127.0.0.1:1") // nothing listening
	defer resetEndpoint()

	start := time.Now()
	emitTestSessionStart()
	elapsed := time.Since(start)

	if elapsed > 100*time.Millisecond {
		t.Fatalf("event emission blocked for %v on unreachable server, expected fire-and-forget", elapsed)
	}
	waitForBeaconDeliveryIdle()
}

// drainSem empties the semaphore so tests start from a clean state.
func drainSem() {
	waitForBeaconDeliveryIdle()
	for {
		select {
		case <-sem:
		default:
			return
		}
	}
}

func TestBeacon_SemaphoreBackpressure(t *testing.T) {
	// Ensure clean semaphore state on entry and exit.
	drainSem()
	t.Cleanup(drainSem)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	overrideEndpoint(srv.URL)
	defer resetEndpoint()

	// Fill all semaphore slots.
	for i := 0; i < maxConcurrentBeacons; i++ {
		sem <- struct{}{}
	}

	// Fire a 51st beacon — should be silently dropped (no panic, no block).
	done := make(chan struct{})
	go func() {
		emitTestSessionStart()
		close(done)
	}()

	select {
	case <-done:
		// Good — beacon was dropped without blocking.
	case <-time.After(2 * time.Second):
		t.Fatal("event emission blocked when semaphore was full — should drop silently")
	}

	// Drain and verify beacon works again after slots are freed.
	drainSem()

	received := make(chan struct{}, 1)
	srv2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case received <- struct{}{}:
		default:
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv2.Close()
	overrideEndpoint(srv2.URL)

	resetInstallIDState()
	dir := t.TempDir()
	overrideKaboomDir(dir)
	defer resetKaboomDir()

	emitTestSessionStart()
	select {
	case <-received:
		// Good — beacon fired after slots were freed.
	case <-time.After(2 * time.Second):
		t.Fatal("beacon did not fire after semaphore slots were freed")
	}
}

// #5: llm field included when SetLLMName is set, omitted when empty.
func TestBeacon_LLMFieldInEnvelope(t *testing.T) {
	received := make(chan map[string]any, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		select {
		case received <- body:
		default:
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	overrideEndpoint(srv.URL)
	defer resetEndpoint()
	SetLLMName("claude-code")
	defer SetLLMName("")

	emitTestSessionStart()

	var body map[string]any
	select {
	case body = <-received:
	case <-time.After(2 * time.Second):
		t.Fatal("beacon not received within timeout")
	}

	if body["llm"] != "claude-code" {
		t.Errorf("llm = %v, want claude-code", body["llm"])
	}
}

// #14: Opt-out tests for canonical events and BeaconUsageSummary.
func TestCanonicalBeacon_DisabledByEnv(t *testing.T) {
	t.Setenv("KABOOM_TELEMETRY", "off")
	waitForBeaconDeliveryIdle()
	drainSem()

	fired := make(chan bool, 1)
	setOnFireBeacon(func(sent bool) {
		select {
		case fired <- sent:
		default:
		}
	})
	defer setOnFireBeacon(nil)

	overrideEndpoint("http://198.51.100.1:1")
	defer resetEndpoint()

	emitTestSessionStart()

	select {
	case sent := <-fired:
		if sent {
			t.Fatal("canonical event was sent despite KABOOM_TELEMETRY=off")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("onFireBeacon hook not called")
	}
}

func TestBeaconUsageSummary_DisabledByEnv(t *testing.T) {
	t.Setenv("KABOOM_TELEMETRY", "off")
	waitForBeaconDeliveryIdle()
	drainSem()

	fired := make(chan bool, 1)
	setOnFireBeacon(func(sent bool) {
		select {
		case fired <- sent:
		default:
		}
	})
	defer setOnFireBeacon(nil)

	overrideEndpoint("http://198.51.100.1:1")
	defer resetEndpoint()

	BeaconUsageSummary(5, &UsageSnapshot{
		ToolStats: []ToolStat{{Tool: "observe:page", Family: "observe", Name: "page", Count: 1}},
	})

	select {
	case sent := <-fired:
		if sent {
			t.Fatal("BeaconUsageSummary was sent despite KABOOM_TELEMETRY=off")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("onFireBeacon hook not called")
	}
}

// M4: Case-insensitive opt-out.
func TestCanonicalBeacon_DisabledByEnv_CaseInsensitive(t *testing.T) {
	t.Setenv("KABOOM_TELEMETRY", "OFF")

	fired := make(chan bool, 1)
	setOnFireBeacon(func(sent bool) {
		select {
		case fired <- sent:
		default:
		}
	})
	defer setOnFireBeacon(nil)

	overrideEndpoint("http://198.51.100.1:1")
	defer resetEndpoint()

	emitTestSessionStart()

	select {
	case sent := <-fired:
		if sent {
			t.Fatal("canonical event was sent despite KABOOM_TELEMETRY=OFF (uppercase)")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("onFireBeacon hook not called")
	}
}

// L1: BeaconUsageSummary with nil snapshot — should not fire.
func TestBeaconUsageSummary_NilSnapshot(t *testing.T) {
	waitForBeaconDeliveryIdle()
	var called bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	overrideEndpoint(srv.URL)
	defer resetEndpoint()

	BeaconUsageSummary(5, nil)
	waitForBeaconDeliveryIdle()

	if called {
		t.Fatal("BeaconUsageSummary should not fire with nil snapshot")
	}
}

// #13: BuildUsageSummaryPayload structure test.
func TestBuildUsageSummaryPayload_Structure(t *testing.T) {
	resetInstallIDState()
	dir := t.TempDir()
	overrideKaboomDir(dir)
	defer resetKaboomDir()
	resetSessionState()
	TouchSession()

	snapshot := &UsageSnapshot{
		ToolStats: []ToolStat{
			{Tool: "observe:page", Family: "observe", Name: "page", Count: 3, LatencyAvgMs: 45, LatencyMaxMs: 100},
			{Tool: "interact:click", Family: "interact", Name: "click", Count: 1},
		},
		AsyncOutcomes: map[string]int{"complete": 2},
	}
	payload := BuildUsageSummaryPayload(5, snapshot)

	if payload["event"] != "usage_summary" {
		t.Errorf("event = %v, want usage_summary", payload["event"])
	}
	if payload["window_m"] != 5 {
		t.Errorf("window_m = %v, want 5", payload["window_m"])
	}
	if _, ok := payload["iid"].(string); !ok {
		t.Error("missing iid field")
	}
	if _, ok := payload["sid"].(string); !ok {
		t.Error("missing sid field")
	}
	if _, ok := payload["v"].(string); !ok {
		t.Error("missing v field")
	}
	if _, ok := payload["os"].(string); !ok {
		t.Error("missing os field")
	}
	if _, ok := payload["ts"].(string); !ok {
		t.Error("missing ts field")
	}
	if _, ok := payload["channel"].(string); !ok {
		t.Error("missing channel field")
	}
	stats, ok := payload["tool_stats"].([]ToolStat)
	if !ok {
		t.Fatalf("tool_stats type = %T, want []ToolStat", payload["tool_stats"])
	}
	if len(stats) != 2 {
		t.Fatalf("tool_stats length = %d, want 2", len(stats))
	}
	if stats[0].Tool != "observe:page" || stats[0].Count != 3 {
		t.Errorf("tool_stats[0] = %+v, want observe:page count=3", stats[0])
	}
	if _, exists := payload["session_depth"]; exists {
		t.Error("session_depth should not be in usage_summary payload — not in Counterscale contract")
	}
	for _, localOnly := range []string{"extension_logs", "diagnostic_logs", "diagnostic_lifecycle", "doctor"} {
		if _, exists := payload[localOnly]; exists {
			t.Errorf("local extension diagnostics leaked into usage telemetry field %q", localOnly)
		}
	}
}

// #6: Semaphore cleanup safety — drainSem prevents leaked slots from poisoning subsequent tests.
func TestBeacon_SemaphoreCleanupOnFailure(t *testing.T) {
	drainSem()
	t.Cleanup(drainSem)

	// Fill sem completely.
	for i := 0; i < maxConcurrentBeacons; i++ {
		sem <- struct{}{}
	}

	// Verify beacon is dropped (not blocked) when semaphore is full.
	done := make(chan struct{})
	go func() {
		emitTestSessionStart()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("beacon blocked on full semaphore")
	}
}
