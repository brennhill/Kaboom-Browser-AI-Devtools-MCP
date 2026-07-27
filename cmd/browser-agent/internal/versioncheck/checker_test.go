// checker_test.go — Release-check fetch, cache, and lifecycle contracts.

package versioncheck

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestCheckerFetchesNewerVersionAndCaches(t *testing.T) {
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		_, _ = w.Write([]byte(`{"tag_name":"v9.9.9"}`))
	}))
	defer server.Close()

	checker := New(Options{CurrentVersion: "1.0.0", ReleaseURL: server.URL, HTTPClient: server.Client()})
	checker.Check()
	checker.Check()
	if got := checker.Available(); got != "9.9.9" {
		t.Fatalf("Available() = %q, want 9.9.9", got)
	}
	if got := hits.Load(); got != 1 {
		t.Fatalf("release requests = %d, want 1", got)
	}
}

func TestCheckerDoesNotAdvertiseOlderOrInvalidReleases(t *testing.T) {
	for _, response := range []string{`{"tag_name":"v0.9.0"}`, `{"tag_name":"v1.0.0"}`, `{"tag_name":""}`, `{invalid`} {
		response := response
		t.Run(response, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(response))
			}))
			defer server.Close()
			checker := New(Options{CurrentVersion: "1.0.0", ReleaseURL: server.URL, HTTPClient: server.Client()})
			checker.Check()
			if got := checker.Available(); got != "" {
				t.Fatalf("Available() = %q, want empty", got)
			}
		})
	}
}

func TestCheckerIgnoresHTTPFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()
	checker := New(Options{CurrentVersion: "1.0.0", ReleaseURL: server.URL, HTTPClient: server.Client()})
	checker.Check()
	if got := checker.Available(); got != "" {
		t.Fatalf("Available() = %q, want empty", got)
	}
}

func TestCheckerStartChecksImmediately(t *testing.T) {
	checked := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		checked <- struct{}{}
		_, _ = w.Write([]byte(`{"tag_name":"v2.0.0"}`))
	}))
	defer server.Close()
	checker := New(Options{
		CurrentVersion: "1.0.0", ReleaseURL: server.URL, HTTPClient: server.Client(),
		Interval: time.Hour,
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	checker.Start(ctx)
	select {
	case <-checked:
	case <-time.After(time.Second):
		t.Fatal("Start() did not perform the initial check")
	}
}
