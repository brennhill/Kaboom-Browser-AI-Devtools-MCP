// telemetry.go — Bounded local diagnostics for bridge fast-path handling.
package fastpathtelemetry

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	statecfg "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/state"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/util"
)

const queueCapacity = 256

type counters struct {
	mu      sync.Mutex
	success int64
	failure int64
}

type record struct {
	path string
	line []byte
}

var (
	methodCounters       counters
	resourceReadCounters counters
	workerOnce           sync.Once
	queue                = make(chan record, queueCapacity)
	pending              sync.WaitGroup
)

func startWorker() {
	util.SafeGo(func() {
		for item := range queue {
			appendRecord(item)
			pending.Done()
		}
	})
}

func enqueue(path string, line []byte) {
	workerOnce.Do(startWorker)
	pending.Add(1)
	select {
	case queue <- record{path: path, line: line}:
	default:
		pending.Done()
	}
}

func appendRecord(item record) {
	if err := os.MkdirAll(filepath.Dir(item.path), 0o750); err != nil {
		return
	}
	// #nosec G304 -- paths are deterministic under the local state root.
	file, err := os.OpenFile(item.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	_, _ = file.Write(append(item.line, '\n'))
	_ = file.Close()
}

// Flush waits for diagnostics accepted by the bounded queue.
func Flush() { pending.Wait() }

func update(target *counters, success bool) (int64, int64) {
	target.mu.Lock()
	defer target.mu.Unlock()
	if success {
		target.success++
	} else {
		target.failure++
	}
	return target.success, target.failure
}

func reset(target *counters) {
	target.mu.Lock()
	defer target.mu.Unlock()
	target.success = 0
	target.failure = 0
}

// ResetMethodCounters resets fast-path method counters.
func ResetMethodCounters() { reset(&methodCounters) }

// ResetResourceReadCounters resets fast-path resource-read counters.
func ResetResourceReadCounters() { reset(&resourceReadCounters) }

// SnapshotResourceReadCounters returns current resource-read counts.
func SnapshotResourceReadCounters() (int64, int64) {
	resourceReadCounters.mu.Lock()
	defer resourceReadCounters.mu.Unlock()
	return resourceReadCounters.success, resourceReadCounters.failure
}

// MethodLogPath returns the local method diagnostic log path.
func MethodLogPath() (string, error) {
	return statecfg.InRoot("logs", "bridge-fastpath-events.jsonl")
}

// ResourceReadLogPath returns the local resource-read diagnostic log path.
func ResourceReadLogPath() (string, error) {
	return statecfg.InRoot("logs", "bridge-fastpath-resource-read.jsonl")
}

// RecordMethod records one bridge fast-path method outcome locally.
func RecordMethod(version, method string, success bool, errorCode int) {
	successCount, failureCount := update(&methodCounters, success)
	write(MethodLogPath, map[string]any{
		"timestamp": time.Now().UTC().Format(time.RFC3339Nano), "event": "bridge_fastpath_method",
		"method": method, "success": success, "error_code": errorCode,
		"success_count": successCount, "failure_count": failureCount,
		"pid": os.Getpid(), "version": version,
	})
}

// RecordResourceRead records one bridge fast-path resource-read outcome locally.
func RecordResourceRead(version, uri string, success bool, errorCode int) {
	successCount, failureCount := update(&resourceReadCounters, success)
	write(ResourceReadLogPath, map[string]any{
		"timestamp": time.Now().UTC().Format(time.RFC3339Nano), "event": "bridge_fastpath_resources_read",
		"uri": uri, "success": success, "error_code": errorCode,
		"success_count": successCount, "failure_count": failureCount,
		"pid": os.Getpid(), "bridge_version": version,
	})
}

func write(pathFn func() (string, error), entry map[string]any) {
	path, err := pathFn()
	if err != nil {
		return
	}
	payload, err := json.Marshal(entry)
	if err != nil {
		return
	}
	enqueue(path, payload)
}
