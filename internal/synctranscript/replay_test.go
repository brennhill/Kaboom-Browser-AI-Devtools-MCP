// replay_test.go — Pins the fake extension's side of the /sync contract.

package synctranscript

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture/syncruntime"
)

// fakeDaemon is the daemon side of /sync: it hands out a queued command and
// records what the client sends back.
type fakeDaemon struct {
	mu       sync.Mutex
	queued   []syncruntime.SyncCommand
	requests []syncruntime.SyncRequest
	clientID string
	server   *httptest.Server
}

func newFakeDaemon(t *testing.T, commands ...syncruntime.SyncCommand) *fakeDaemon {
	t.Helper()
	daemon := &fakeDaemon{queued: commands}
	daemon.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request syncruntime.SyncRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		daemon.mu.Lock()
		daemon.clientID = r.Header.Get("X-Kaboom-Client")
		daemon.requests = append(daemon.requests, request)
		batch := daemon.queued
		daemon.queued = nil
		daemon.mu.Unlock()

		_ = json.NewEncoder(w).Encode(syncruntime.SyncResponse{
			Ack:                  true,
			ConnectionGeneration: 7,
			Commands:             batch,
			NextPollMs:           10,
		})
	}))
	t.Cleanup(daemon.server.Close)
	return daemon
}

func (d *fakeDaemon) received() []syncruntime.SyncRequest {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]syncruntime.SyncRequest(nil), d.requests...)
}

func newTestClient(t *testing.T, daemon *fakeDaemon, records []Record) *Client {
	t.Helper()
	return NewClient(Options{
		Endpoint:   daemon.server.URL,
		Transcript: NewTranscript(records),
		TrackedTab: TrackedTab{ID: 42, URL: "http://localhost:9999/fixture.html", Title: "fixture"},
	})
}

// A probe is deliberately never adopted as the extension session and so never
// receives commands. The replay client has to be the extension or it sits idle
// forever while the suite times out.
func TestClientIdentifiesAsTheExtension(t *testing.T) {
	daemon := newFakeDaemon(t)
	client := newTestClient(t, daemon, nil)
	if err := client.SyncOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	daemon.mu.Lock()
	defer daemon.mu.Unlock()
	if daemon.clientID != "kaboom-extension" {
		t.Errorf("X-Kaboom-Client = %q, want kaboom-extension", daemon.clientID)
	}
}

// The daemon gates most commands on a tracked tab; without one every connected
// category fails on "no tab is being tracked" instead of exercising anything.
func TestClientReportsATrackedTab(t *testing.T) {
	daemon := newFakeDaemon(t)
	client := newTestClient(t, daemon, nil)
	if err := client.SyncOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	requests := daemon.received()
	if len(requests) != 1 || requests[0].Settings == nil {
		t.Fatalf("requests = %+v, want one carrying settings", requests)
	}
	settings := requests[0].Settings
	if !settings.TrackingEnabled || settings.TrackedTabID != 42 {
		t.Errorf("settings = %+v, want tracking enabled on tab 42", settings)
	}
	if settings.TrackedTabURL != "http://localhost:9999/fixture.html" {
		t.Errorf("TrackedTabURL = %q", settings.TrackedTabURL)
	}
}

func TestClientAnswersACommandFromTheTranscript(t *testing.T) {
	daemon := newFakeDaemon(t, syncruntime.SyncCommand{
		ID:     "cmd-1",
		Type:   "analyze",
		Params: json.RawMessage(`{"what":"dom","selector":"body"}`),
	})
	recorder := NewRecorder()
	recorder.Observe(cmd("analyze", `{"what":"dom","selector":"body"}`), Result{
		Status: "complete",
		Result: json.RawMessage(`{"elements":3}`),
	})
	client := newTestClient(t, daemon, recorder.Records())

	if err := client.SyncOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	// The answer rides the next request, exactly as the extension sends it.
	if err := client.SyncOnce(context.Background()); err != nil {
		t.Fatal(err)
	}

	requests := daemon.received()
	if len(requests) != 2 {
		t.Fatalf("requests = %d, want 2", len(requests))
	}
	results := requests[1].CommandResults
	if len(results) != 1 {
		t.Fatalf("CommandResults = %+v, want one", results)
	}
	if results[0].ID != "cmd-1" || results[0].Status != "complete" {
		t.Errorf("result = %+v, want cmd-1 complete", results[0])
	}
	if string(results[0].Result) != `{"elements":3}` {
		t.Errorf("Result = %s", results[0].Result)
	}
}

// The defect this whole exercise guards against: an unanswered command must
// surface as an error, not as an empty success that reads like a clean page.
func TestClientReportsAnUnrecordedCommandAsAnError(t *testing.T) {
	daemon := newFakeDaemon(t, syncruntime.SyncCommand{
		ID: "cmd-1", Type: "analyze", Params: json.RawMessage(`{"what":"never_recorded"}`),
	})
	client := newTestClient(t, daemon, nil)

	if err := client.SyncOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := client.SyncOnce(context.Background()); err != nil {
		t.Fatal(err)
	}

	results := daemon.received()[1].CommandResults
	if len(results) != 1 {
		t.Fatalf("CommandResults = %+v, want one", results)
	}
	if results[0].Status != "error" {
		t.Errorf("Status = %q, want error — an unrecorded command must not look complete", results[0].Status)
	}
	if results[0].Error == "" {
		t.Error("an unrecorded command produced no error message")
	}
	if len(results[0].Result) != 0 {
		t.Errorf("Result = %s, want nothing alongside the error", results[0].Result)
	}
}

func TestClientPreservesARecordedFailure(t *testing.T) {
	daemon := newFakeDaemon(t, syncruntime.SyncCommand{
		ID: "cmd-1", Type: "analyze", Params: json.RawMessage(`{"what":"computed_styles"}`),
	})
	recorder := NewRecorder()
	recorder.Observe(cmd("analyze", `{"what":"computed_styles"}`), Result{Status: "error", Error: "no active tab"})
	client := newTestClient(t, daemon, recorder.Records())

	_ = client.SyncOnce(context.Background())
	_ = client.SyncOnce(context.Background())

	results := daemon.received()[1].CommandResults
	if len(results) != 1 || results[0].Status != "error" || results[0].Error != "no active tab" {
		t.Errorf("results = %+v, want the recorded failure replayed verbatim", results)
	}
}

// Results must not be re-sent forever; the daemon would see duplicate terminal
// outcomes for a command it already closed.
func TestClientSendsEachResultOnce(t *testing.T) {
	daemon := newFakeDaemon(t, syncruntime.SyncCommand{
		ID: "cmd-1", Type: "analyze", Params: json.RawMessage(`{"what":"dom"}`),
	})
	recorder := NewRecorder()
	recorder.Observe(cmd("analyze", `{"what":"dom"}`), Result{Status: "complete"})
	client := newTestClient(t, daemon, recorder.Records())

	for range 3 {
		if err := client.SyncOnce(context.Background()); err != nil {
			t.Fatal(err)
		}
	}

	sent := 0
	for _, request := range daemon.received() {
		sent += len(request.CommandResults)
	}
	if sent != 1 {
		t.Errorf("sent %d result(s), want exactly 1", sent)
	}
}

func TestClientAdoptsTheConnectionGeneration(t *testing.T) {
	daemon := newFakeDaemon(t)
	client := newTestClient(t, daemon, nil)
	_ = client.SyncOnce(context.Background())
	_ = client.SyncOnce(context.Background())

	requests := daemon.received()
	if requests[1].ConnectionGeneration != 7 {
		t.Errorf("ConnectionGeneration = %d, want the 7 the daemon assigned", requests[1].ConnectionGeneration)
	}
}

func TestClientKeepsOneSessionIdentityAcrossSyncs(t *testing.T) {
	daemon := newFakeDaemon(t)
	client := newTestClient(t, daemon, nil)
	_ = client.SyncOnce(context.Background())
	_ = client.SyncOnce(context.Background())

	requests := daemon.received()
	if requests[0].ExtSessionID == "" {
		t.Fatal("no ext_session_id sent")
	}
	if requests[0].ExtSessionID != requests[1].ExtSessionID {
		t.Error("session identity changed between syncs; the daemon would treat it as a reconnect")
	}
}

// An HTTP failure must stop the run. Swallowing it would leave the suite
// waiting on a daemon that is not being polled, and blame the timeout on
// whatever category happened to be running.
func TestSyncOnceReportsAnHTTPFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusForbidden)
	}))
	defer server.Close()
	client := NewClient(Options{Endpoint: server.URL, Transcript: NewTranscript(nil)})
	if err := client.SyncOnce(context.Background()); err == nil {
		t.Error("a 403 from /sync was not reported")
	}
}

func TestSyncOnceReportsAnUnparseableResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"error":"boom"}`))
	}))
	defer server.Close()
	client := NewClient(Options{Endpoint: server.URL, Transcript: NewTranscript(nil)})
	if err := client.SyncOnce(context.Background()); err == nil {
		t.Error("a response carrying no sync fields was accepted")
	}
}

func TestRunStopsWhenTheContextIsCancelled(t *testing.T) {
	daemon := newFakeDaemon(t)
	client := newTestClient(t, daemon, nil)
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() { done <- client.Run(ctx) }()
	cancel()

	select {
	case err := <-done:
		if err != nil && err != context.Canceled {
			t.Errorf("Run() = %v, want a clean stop", err)
		}
	case <-t.Context().Done():
		t.Fatal("Run did not return after cancellation")
	}
}
