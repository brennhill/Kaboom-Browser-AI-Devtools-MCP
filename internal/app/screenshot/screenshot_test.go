// screenshot_test.go — Direct tests of the transport-independent screenshot use case with fakes.

package screenshot

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/state"
)

type fakeCommands struct {
	enqueued    []queries.PendingQuery
	timeouts    []time.Duration
	enqueueErr  error
	result      json.RawMessage
	waitErr     error
	waitTimeout time.Duration
}

func (f *fakeCommands) Enqueue(query queries.PendingQuery, timeout time.Duration, _ string) (string, error) {
	f.enqueued = append(f.enqueued, query)
	f.timeouts = append(f.timeouts, timeout)
	if f.enqueueErr != nil {
		return "", f.enqueueErr
	}
	return "q-1", nil
}

func (f *fakeCommands) WaitQueryResult(_ context.Context, queryID string, timeout time.Duration) (json.RawMessage, error) {
	f.waitTimeout = timeout
	if queryID != "q-1" {
		return nil, errors.New("unexpected query id " + queryID)
	}
	return f.result, f.waitErr
}

func newService(commands Commands, tracked bool) *Service {
	return New(Deps{Commands: commands, TrackingEnabled: func() bool { return tracked }})
}

func kindOf(t *testing.T, err error) FailureKind {
	t.Helper()
	var failure *Failure
	if !errors.As(err, &failure) {
		t.Fatalf("error %v is not a *Failure", err)
	}
	return failure.Kind
}

func TestCaptureRejectsUntrackedAndInvalidOptionsWithoutEnqueueing(t *testing.T) {
	cases := []struct {
		name    string
		tracked bool
		opts    Options
		kind    FailureKind
	}{
		{"untracked", false, Options{}, KindNotTracking},
		{"format", true, Options{Format: "gif"}, KindInvalidFormat},
		{"quality-low", true, Options{Quality: -1}, KindInvalidQuality},
		{"quality-high", true, Options{Quality: 101}, KindInvalidQuality},
	}
	for _, tc := range cases {
		commands := &fakeCommands{}
		_, err := newService(commands, tc.tracked).Capture(context.Background(), tc.opts)
		if got := kindOf(t, err); got != tc.kind {
			t.Fatalf("%s: kind = %s, want %s", tc.name, got, tc.kind)
		}
		if len(commands.enqueued) != 0 {
			t.Fatalf("%s: enqueued %d commands", tc.name, len(commands.enqueued))
		}
	}
}

func TestCaptureEnqueuesCanonicalQueryAndSplitsDataURL(t *testing.T) {
	commands := &fakeCommands{result: json.RawMessage(`{"path":"/s/a.jpg","data_url":"data:image/jpeg;base64,QUJD"}`)}
	result, err := newService(commands, true).Capture(context.Background(), Options{
		Format: "jpeg", Quality: 80, FullPage: true, Selector: "#x", WaitForStable: true,
	})
	if err != nil {
		t.Fatalf("Capture: %v", err)
	}
	if len(commands.enqueued) != 1 || commands.enqueued[0].Type != "screenshot" {
		t.Fatalf("enqueued = %+v", commands.enqueued)
	}
	var params map[string]any
	if err := json.Unmarshal(commands.enqueued[0].Params, &params); err != nil {
		t.Fatal(err)
	}
	want := map[string]any{"format": "jpeg", "quality": float64(80), "full_page": true, "selector": "#x", "wait_for_stable": true}
	for k, v := range want {
		if params[k] != v {
			t.Fatalf("params[%s] = %v, want %v (all %v)", k, params[k], v, params)
		}
	}
	if commands.timeouts[0] != DefaultTimeout || commands.waitTimeout != DefaultTimeout {
		t.Fatalf("timeouts = %v / %v, want %v", commands.timeouts, commands.waitTimeout, DefaultTimeout)
	}
	if result.DataURL != "data:image/jpeg;base64,QUJD" {
		t.Fatalf("DataURL = %q", result.DataURL)
	}
	if _, dup := result.Fields["data_url"]; dup {
		t.Fatal("data_url must not be duplicated in Fields")
	}
	if result.Path() != "/s/a.jpg" {
		t.Fatalf("Path() = %q", result.Path())
	}
}

func TestCaptureEmptyOptionsSendEmptyParams(t *testing.T) {
	commands := &fakeCommands{result: json.RawMessage(`{"filename":"f.jpg"}`)}
	result, err := newService(commands, true).Capture(context.Background(), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if string(commands.enqueued[0].Params) != `{}` {
		t.Fatalf("params = %s", commands.enqueued[0].Params)
	}
	dir, err := state.ScreenshotsDir()
	if err != nil {
		t.Fatal(err)
	}
	if result.Path() != filepath.Join(dir, "f.jpg") || result.DataURL != "" {
		t.Fatalf("result = %+v", result)
	}
}

func TestCaptureMapsCommandFailures(t *testing.T) {
	cases := []struct {
		name     string
		commands *fakeCommands
		kind     FailureKind
	}{
		{"queue-full", &fakeCommands{enqueueErr: errors.New("full")}, KindQueueFull},
		{"timeout", &fakeCommands{waitErr: errors.New("deadline")}, KindTimeout},
		{"malformed", &fakeCommands{result: json.RawMessage(`not-json`)}, KindMalformedResult},
		{"extension", &fakeCommands{result: json.RawMessage(`{"error":"tab hidden"}`)}, KindExtensionError},
	}
	for _, tc := range cases {
		_, err := newService(tc.commands, true).Capture(context.Background(), Options{})
		if got := kindOf(t, err); got != tc.kind {
			t.Fatalf("%s: kind = %s, want %s", tc.name, got, tc.kind)
		}
	}
	_, err := newService(&fakeCommands{result: json.RawMessage(`{"error":"tab hidden"}`)}, true).Capture(context.Background(), Options{})
	var failure *Failure
	if !errors.As(err, &failure) || failure.Detail != "tab hidden" {
		t.Fatalf("extension failure detail = %+v", failure)
	}
}

func TestCaptureHonorsConfiguredTimeout(t *testing.T) {
	commands := &fakeCommands{result: json.RawMessage(`{}`)}
	service := New(Deps{Commands: commands, TrackingEnabled: func() bool { return true }, Timeout: time.Second})
	if _, err := service.Capture(context.Background(), Options{}); err != nil {
		t.Fatal(err)
	}
	if commands.timeouts[0] != time.Second || commands.waitTimeout != time.Second {
		t.Fatalf("timeouts = %v / %v", commands.timeouts, commands.waitTimeout)
	}
}

func TestCaptureSaveToWritesFileOrRecordsError(t *testing.T) {
	data := base64.StdEncoding.EncodeToString([]byte("png-bytes"))
	commands := &fakeCommands{result: json.RawMessage(`{"data_url":"data:image/png;base64,` + data + `"}`)}
	target := filepath.Join(t.TempDir(), "nested", "shot.png")
	result, err := newService(commands, true).Capture(context.Background(), Options{SaveTo: target})
	if err != nil {
		t.Fatal(err)
	}
	if result.Fields["save_to"] != target {
		t.Fatalf("save_to = %v", result.Fields["save_to"])
	}
	written, err := os.ReadFile(target)
	if err != nil || string(written) != "png-bytes" {
		t.Fatalf("written = %q, %v", written, err)
	}

	result, err = newService(commands, true).Capture(context.Background(), Options{SaveTo: "bad.txt"})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := result.Fields["save_to_error"].(string); !ok {
		t.Fatalf("expected save_to_error, got %v", result.Fields)
	}
}
