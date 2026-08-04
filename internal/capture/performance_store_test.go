package capture

import (
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/performance"
)

func TestPerformanceStore_AppendSnapshotsEvictsOldest(t *testing.T) {
	store := PerformanceStore{
		snapshots:       make(map[string]performance.PerformanceSnapshot),
		snapshotOrder:   make([]string, 0),
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

func TestPerformanceStore_RetainsBoundedRepeatedSamples(t *testing.T) {
	store := newPerformanceStore()
	for i := 0; i < maxPerformanceSamples+3; i++ {
		store.appendSnapshots([]performance.PerformanceSnapshot{{
			URL: "https://app.local/repeated", Timestamp: itoa(i),
			Timing: performance.PerformanceTiming{Load: float64(i)},
		}})
	}

	samples := store.samplesList()
	if len(samples) != maxPerformanceSamples {
		t.Fatalf("sample count = %d, want %d", len(samples), maxPerformanceSamples)
	}
	if samples[0].Timing.Load != 3 || samples[len(samples)-1].Timing.Load != float64(maxPerformanceSamples+2) {
		t.Fatalf("retained samples = first %.0f last %.0f", samples[0].Timing.Load, samples[len(samples)-1].Timing.Load)
	}
	pressure := store.Pressure()
	if pressure.Samples.Size != maxPerformanceSamples || pressure.Samples.Dropped != 3 {
		t.Fatalf("sample pressure = %#v", pressure.Samples)
	}
}

func TestPerformanceStore_SnapshotsListDetached(t *testing.T) {
	store := PerformanceStore{
		snapshots:       make(map[string]performance.PerformanceSnapshot),
		snapshotOrder:   make([]string, 0),
		beforeSnapshots: make(map[string]performance.PerformanceSnapshot),
	}
	input := performance.PerformanceSnapshot{
		URL:       "https://app.local",
		Resources: []performance.ResourceEntry{{URL: "original"}},
		Network:   performance.NetworkSummary{ByType: map[string]performance.TypeSummary{"script": {Count: 1}}},
		UserTiming: &performance.UserTimingData{
			Marks: []performance.UserTimingEntry{{Name: "original"}},
		},
	}
	store.appendSnapshots([]performance.PerformanceSnapshot{input})
	input.Resources[0].URL = "input-mutated"
	input.Network.ByType["script"] = performance.TypeSummary{Count: 99}
	input.UserTiming.Marks[0].Name = "input-mutated"

	list := store.snapshotsList()
	if len(list) != 1 {
		t.Fatalf("list len = %d, want 1", len(list))
	}
	list[0].URL = "mutated"
	list[0].Resources[0].URL = "output-mutated"
	list[0].Network.ByType["script"] = performance.TypeSummary{Count: 88}
	list[0].UserTiming.Marks[0].Name = "output-mutated"
	if got, _ := store.snapshotByURL("https://app.local"); got.URL != "https://app.local" ||
		got.Resources[0].URL != "original" || got.Network.ByType["script"].Count != 1 || got.UserTiming.Marks[0].Name != "original" {
		t.Fatalf("store mutated through snapshotsList: %+v", got)
	}
}

func TestPerformanceStore_BeforeSnapshotStoreAndTake(t *testing.T) {
	store := PerformanceStore{
		snapshots:       make(map[string]performance.PerformanceSnapshot),
		snapshotOrder:   make([]string, 0),
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
		beforeSnapshots: make(map[string]performance.PerformanceSnapshot),
	}
	store.appendSnapshots([]performance.PerformanceSnapshot{{URL: "https://app.local"}})
	store.storeBeforeSnapshot("corr-1", performance.PerformanceSnapshot{URL: "https://app.local/before"})

	store.clear()

	if len(store.snapshots) != 0 || len(store.snapshotOrder) != 0 {
		t.Fatalf("expected snapshots cleared, got map=%d order=%d", len(store.snapshots), len(store.snapshotOrder))
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
