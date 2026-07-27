package capture

import (
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/performance"
)

func TestPerformanceStore_AppendSnapshotsEvictsOldest(t *testing.T) {
	store := PerformanceStore{
		snapshots:       make(map[string]performance.PerformanceSnapshot),
		snapshotOrder:   make([]string, 0),
		baselines:       make(map[string]performance.PerformanceBaseline),
		baselineOrder:   make([]string, 0),
		beforeSnapshots: make(map[string]performance.PerformanceSnapshot),
	}

	input := make([]performance.PerformanceSnapshot, 0, 101)
	for i := 0; i < 101; i++ {
		input = append(input, performance.PerformanceSnapshot{URL: "https://app.local/page-" + itoa(i)})
	}
	store.appendSnapshots(input)

	if got := len(store.snapshots); got != 100 {
		t.Fatalf("snapshot count = %d, want 100", got)
	}
	if _, ok := store.snapshotByURL("https://app.local/page-0"); ok {
		t.Fatal("expected oldest snapshot to be evicted")
	}
	if _, ok := store.snapshotByURL("https://app.local/page-100"); !ok {
		t.Fatal("expected newest snapshot to remain")
	}
}

func TestPerformanceStore_SnapshotsListDetached(t *testing.T) {
	store := PerformanceStore{
		snapshots:       make(map[string]performance.PerformanceSnapshot),
		snapshotOrder:   make([]string, 0),
		baselines:       make(map[string]performance.PerformanceBaseline),
		baselineOrder:   make([]string, 0),
		beforeSnapshots: make(map[string]performance.PerformanceSnapshot),
	}
	store.appendSnapshots([]performance.PerformanceSnapshot{{URL: "https://app.local"}})

	list := store.snapshotsList()
	if len(list) != 1 {
		t.Fatalf("list len = %d, want 1", len(list))
	}
	list[0].URL = "mutated"
	if got, _ := store.snapshotByURL("https://app.local"); got.URL != "https://app.local" {
		t.Fatalf("store mutated through snapshotsList: %+v", got)
	}
}

func TestPerformanceStore_BeforeSnapshotStoreAndTake(t *testing.T) {
	store := PerformanceStore{
		snapshots:       make(map[string]performance.PerformanceSnapshot),
		snapshotOrder:   make([]string, 0),
		baselines:       make(map[string]performance.PerformanceBaseline),
		baselineOrder:   make([]string, 0),
		beforeSnapshots: make(map[string]performance.PerformanceSnapshot),
	}

	store.storeBeforeSnapshot("corr-1", performance.PerformanceSnapshot{URL: "https://app.local/before"})
	got, ok := store.takeBeforeSnapshot("corr-1")
	if !ok {
		t.Fatal("expected before snapshot to be found")
	}
	if got.URL != "https://app.local/before" {
		t.Fatalf("before snapshot URL = %q, want %q", got.URL, "https://app.local/before")
	}
	if _, ok := store.takeBeforeSnapshot("corr-1"); ok {
		t.Fatal("expected before snapshot to be consume-on-read")
	}
}

func TestPerformanceStore_Clear(t *testing.T) {
	store := PerformanceStore{
		snapshots:       make(map[string]performance.PerformanceSnapshot),
		snapshotOrder:   make([]string, 0),
		baselines:       make(map[string]performance.PerformanceBaseline),
		baselineOrder:   make([]string, 0),
		beforeSnapshots: make(map[string]performance.PerformanceSnapshot),
	}
	store.appendSnapshots([]performance.PerformanceSnapshot{{URL: "https://app.local"}})
	store.storeBeforeSnapshot("corr-1", performance.PerformanceSnapshot{URL: "https://app.local/before"})

	store.clear()

	if len(store.snapshots) != 0 || len(store.snapshotOrder) != 0 {
		t.Fatalf("expected snapshots cleared, got map=%d order=%d", len(store.snapshots), len(store.snapshotOrder))
	}
	if len(store.baselines) != 0 || len(store.baselineOrder) != 0 {
		t.Fatalf("expected baselines cleared, got map=%d order=%d", len(store.baselines), len(store.baselineOrder))
	}
	if len(store.beforeSnapshots) != 0 {
		t.Fatalf("expected beforeSnapshots cleared, got %d", len(store.beforeSnapshots))
	}
}

func itoa(v int) string {
	if v == 0 {
		return "0"
	}
	sign := ""
	if v < 0 {
		sign = "-"
		v = -v
	}
	var digits [20]byte
	i := len(digits)
	for v > 0 {
		i--
		digits[i] = byte('0' + v%10)
		v /= 10
	}
	return sign + string(digits[i:])
}
