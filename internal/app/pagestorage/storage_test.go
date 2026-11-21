// storage_test.go — Tests storage use-case helpers.
// Docs: docs/features/feature/observe/index.md

package pagestorage

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
)

func TestToIntAcceptsWireAndServerNumericKinds(t *testing.T) {
	t.Parallel()
	for _, value := range []any{int(4), int32(5), int64(6), float64(7)} {
		if got, ok := toInt(value); !ok || got < 4 || got > 7 {
			t.Fatalf("toInt(%T(%v)) = %d, %t", value, value, got, ok)
		}
	}
	if _, ok := toInt("8"); ok {
		t.Fatal("toInt accepted a string")
	}
}

type fakeRunner struct {
	enqueueErr error
	waitErr    error
	results    []string
	queued     []string
}

func (f *fakeRunner) Enqueue(query queries.PendingQuery, _ time.Duration, _ string) (string, error) {
	if f.enqueueErr != nil {
		return "", f.enqueueErr
	}
	f.queued = append(f.queued, query.Type)
	return query.Type, nil
}

func (f *fakeRunner) WaitQueryResult(context.Context, string, time.Duration) (json.RawMessage, error) {
	if f.waitErr != nil {
		return nil, f.waitErr
	}
	next := f.results[0]
	f.results = f.results[1:]
	return json.RawMessage(next), nil
}

func newTestService(runner queries.QueryRunner, tracked bool) *Service {
	return New(Deps{Commands: runner, TrackingEnabled: func() bool { return tracked }})
}

func failureKind(t *testing.T, err error) FailureKind {
	t.Helper()
	var failure *Failure
	if !errors.As(err, &failure) {
		t.Fatalf("error %v is not a *Failure", err)
	}
	return failure.Kind
}

func TestStorageClassifiesFailures(t *testing.T) {
	t.Parallel()
	cases := map[FailureKind]struct {
		runner  *fakeRunner
		tracked bool
	}{
		KindNotTracking:   {&fakeRunner{}, false},
		KindQueueFull:     {&fakeRunner{enqueueErr: errors.New("full")}, true},
		KindTimeout:       {&fakeRunner{waitErr: errors.New("slow")}, true},
		KindInvalidResult: {&fakeRunner{results: []string{`{bad`}}, true},
		KindExtension:     {&fakeRunner{results: []string{`{"error":"denied"}`}}, true},
	}
	for want, tc := range cases {
		if _, err := newTestService(tc.runner, tc.tracked).Storage(context.Background(), StorageOptions{}); failureKind(t, err) != want {
			t.Fatalf("kind = %v, want %v", failureKind(t, err), want)
		}
	}
}

func TestStorageFiltersSectionsAndSkipsIndexedDBForTypedRequests(t *testing.T) {
	t.Parallel()
	runner := &fakeRunner{results: []string{`{"url":"https://a.test","localStorage":{"k":"v"},"cookies":[{"name":"c"}]}`}}
	fields, err := newTestService(runner, true).Storage(context.Background(), StorageOptions{StorageType: "local"})
	if err != nil {
		t.Fatal(err)
	}
	if fields["url"] != "https://a.test" || fields["local_storage"] == nil || fields["cookies"] != nil || fields["indexeddb"] != nil {
		t.Fatalf("fields = %#v", fields)
	}
	if len(runner.queued) != 1 {
		t.Fatalf("queued = %v, want only state_capture", runner.queued)
	}
}

func TestIndexedDBReportsCountAndNotTracking(t *testing.T) {
	t.Parallel()
	if _, err := newTestService(&fakeRunner{}, false).IndexedDB("db", "s", 5); failureKind(t, err) != KindNotTracking {
		t.Fatal("untracked IndexedDB read did not fail with not_tracking")
	}
	runner := &fakeRunner{results: []string{`{"success":true,"result":{"entries":[{"id":1}],"count":7,"object_stores":["s"]}}`}}
	fields, err := newTestService(runner, true).IndexedDB("db", "s", 5)
	if err != nil {
		t.Fatal(err)
	}
	if fields["count"] != 7 || fields["object_stores"] == nil {
		t.Fatalf("fields = %#v", fields)
	}
}
