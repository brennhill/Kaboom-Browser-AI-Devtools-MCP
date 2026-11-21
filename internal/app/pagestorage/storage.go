// storage.go — Storage and IndexedDB workflows: tracking gate, extension queries, filtering, summaries.
// Docs: docs/features/feature/observe/index.md

package pagestorage

import (
	"context"
	"encoding/json"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
)

// stateCaptureTimeout bounds queueing and the wait for the state_capture reply.
const stateCaptureTimeout = 10 * time.Second

// FailureKind classifies why a storage read failed.
type FailureKind string

// Failure kinds; adapters map each to a protocol error.
const (
	KindNotTracking   FailureKind = "not_tracking"
	KindQueueFull     FailureKind = "queue_full"
	KindTimeout       FailureKind = "timeout"
	KindInvalidResult FailureKind = "invalid_result"
	KindExtension     FailureKind = "extension_error"
)

// Failure is a typed storage failure.
type Failure struct {
	Kind   FailureKind
	Detail string
}

func (f *Failure) Error() string { return string(f.Kind) + ": " + f.Detail }

// Deps wires the use case to its ports.
type Deps struct {
	Commands        queries.QueryRunner
	TrackingEnabled func() bool
}

// Service runs the storage workflows.
type Service struct{ deps Deps }

// New binds the use case to its ports.
func New(deps Deps) *Service { return &Service{deps: deps} }

// StorageOptions selects and shapes storage sections.
type StorageOptions struct {
	Summary     bool
	StorageType string // "local", "session", "cookies", or "" for all
	Key         string // specific key/cookie name filter
}

func (o StorageOptions) includes(storageType string) bool {
	return o.StorageType == "" || o.StorageType == storageType
}

func (s *Service) tracked() bool {
	return s.deps.TrackingEnabled != nil && s.deps.TrackingEnabled() && s.deps.Commands != nil
}

// Storage returns the url plus the selected storage sections. With no type
// filter it also attaches a best-effort IndexedDB listing (indexeddb, and
// indexeddb_error when the listing failed).
func (s *Service) Storage(ctx context.Context, opts StorageOptions) (map[string]any, error) {
	if !s.tracked() {
		return nil, &Failure{Kind: KindNotTracking}
	}
	state, err := s.captureState(ctx)
	if err != nil {
		return nil, err
	}
	fields := map[string]any{"url": state["url"]}
	attachSections(fields, state, opts)
	if opts.StorageType == "" {
		if indexeddb, listErr := listing(s.deps.Commands); listErr != nil {
			fields["indexeddb"] = map[string]any{"supported": false, "databases": []any{}}
			fields["indexeddb_error"] = listErr.Error()
		} else {
			fields["indexeddb"] = indexeddb
		}
	}
	return fields, nil
}

func (s *Service) captureState(ctx context.Context) (map[string]any, error) {
	queryID, err := s.deps.Commands.Enqueue(queries.PendingQuery{
		Type: "state_capture", Params: json.RawMessage(`{"action":"capture"}`),
	}, stateCaptureTimeout, "")
	if err != nil {
		return nil, &Failure{Kind: KindQueueFull, Detail: err.Error()}
	}
	raw, err := s.deps.Commands.WaitQueryResult(ctx, queryID, stateCaptureTimeout)
	if err != nil {
		return nil, &Failure{Kind: KindTimeout, Detail: err.Error()}
	}
	var state map[string]any
	if err := json.Unmarshal(raw, &state); err != nil {
		return nil, &Failure{Kind: KindInvalidResult, Detail: err.Error()}
	}
	if message, ok := state["error"].(string); ok {
		return nil, &Failure{Kind: KindExtension, Detail: message}
	}
	return state, nil
}

// IndexedDB returns up to limit rows from one object store: entries, count
// and object_stores when the page reported them.
func (s *Service) IndexedDB(database, store string, limit int) (map[string]any, error) {
	if !s.tracked() {
		return nil, &Failure{Kind: KindNotTracking}
	}
	data, err := entries(s.deps.Commands, database, store, limit)
	if err != nil {
		return nil, &Failure{Kind: KindExtension, Detail: err.Error()}
	}
	rows, _ := data["entries"].([]any)
	count := len(rows)
	if c, ok := toInt(data["count"]); ok {
		count = c
	}
	fields := map[string]any{"entries": rows, "count": count}
	if v, ok := data["object_stores"]; ok {
		fields["object_stores"] = v
	}
	return fields, nil
}

// toInt reads a count out of the page's reply, which arrives as float64 through
// encoding/json but may be any numeric kind when the value is synthesized server-side.
func toInt(v any) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case int32:
		return int(n), true
	case int64:
		return int(n), true
	case float64:
		return int(n), true
	default:
		return 0, false
	}
}
