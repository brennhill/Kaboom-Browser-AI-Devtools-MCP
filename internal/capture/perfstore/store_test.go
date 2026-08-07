// store_test.go — Defines deterministic performance-retention ownership contracts.
package perfstore

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/performance"
)

func TestWaitForURLSnapshotChangeUsesGenerationEvents(t *testing.T) {
	t.Parallel()
	store := New()
	baseline := performance.PerformanceSnapshot{URL: "/dashboard", Timestamp: "before"}
	store.Add([]performance.PerformanceSnapshot{baseline})
	_, notify := store.snapshotChangeSnapshot("/dashboard")
	result := make(chan performance.PerformanceSnapshot, 1)
	go func() {
		snapshot, changed := store.WaitForURLSnapshotChange(context.Background(), baseline.URL, baseline.Timestamp, time.Second)
		if !changed {
			result <- performance.PerformanceSnapshot{}
			return
		}
		result <- snapshot
	}()
	after := performance.PerformanceSnapshot{URL: baseline.URL, Timestamp: "after"}
	store.Add([]performance.PerformanceSnapshot{after})
	select {
	case <-notify:
	default:
		t.Fatal("snapshot addition did not close the change generation")
	}
	select {
	case snapshot := <-result:
		if snapshot.Timestamp != "after" {
			t.Fatalf("changed snapshot = %#v", snapshot)
		}
	case <-time.After(time.Second):
		t.Fatal("snapshot waiter did not wake")
	}
}

func TestWaitForURLSnapshotChangeHonorsImmediateAndCancelledBounds(t *testing.T) {
	t.Parallel()
	store := New()
	store.Add([]performance.PerformanceSnapshot{{URL: "/dashboard", Timestamp: "before"}})
	if snapshot, changed := store.WaitForURLSnapshotChange(context.Background(), "/dashboard", "before", 0); changed || snapshot.Timestamp != "before" {
		t.Fatalf("immediate result = %#v, %t", snapshot, changed)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if snapshot, changed := store.WaitForURLSnapshotChange(ctx, "/dashboard", "before", time.Second); changed || snapshot.Timestamp != "before" {
		t.Fatalf("cancelled result = %#v, %t", snapshot, changed)
	}
}

func TestStoreRetainsRepeatedSamplesAndEvictsOldestURL(t *testing.T) {
	now := time.Unix(100, 0)
	store := newStore(func() time.Time { return now })
	for index := 0; index < maxSamples+3; index++ {
		store.Add([]performance.PerformanceSnapshot{
			{URL: "https://app.local/repeated", Timing: performance.PerformanceTiming{Load: float64(index)}},
			{URL: fmt.Sprintf("https://app.local/page-%d", index)},
		})
	}

	samples := store.Samples()
	if len(samples) != maxSamples || samples[0].URL != "https://app.local/repeated" {
		t.Fatalf("retained samples = %d, first=%#v", len(samples), samples[0])
	}
	if _, exists := store.ByURL("https://app.local/page-0"); exists {
		t.Fatal("oldest URL snapshot was not evicted")
	}
	if store.Pressure().Samples.Dropped != int64((maxSamples+3)*2-maxSamples) {
		t.Fatalf("sample pressure = %#v", store.Pressure().Samples)
	}
}

func TestStoreSnapshotsAreDetachedAndBeforeSnapshotsAreConsumed(t *testing.T) {
	store := newStore(func() time.Time { return time.Unix(100, 0) })
	input := performance.PerformanceSnapshot{
		URL:       "https://app.local",
		Resources: []performance.ResourceEntry{{URL: "original"}},
		Network:   performance.NetworkSummary{ByType: map[string]performance.TypeSummary{"script": {Count: 1}}},
	}
	store.Add([]performance.PerformanceSnapshot{input})
	input.Resources[0].URL = "input-mutated"
	entries := store.Entries()
	entries[0].Resources[0].URL = "output-mutated"
	stored, exists := store.ByURL("https://app.local")
	if !exists || stored.Resources[0].URL != "original" || stored.Network.ByType["script"].Count != 1 {
		t.Fatalf("stored snapshot aliases caller data: %#v", stored)
	}

	store.StoreBefore("corr-1", stored)
	before, exists := store.TakeBefore("corr-1")
	if !exists || before.URL != stored.URL {
		t.Fatalf("before snapshot = %#v, exists=%t", before, exists)
	}
	if _, exists := store.TakeBefore("corr-1"); exists {
		t.Fatal("before snapshot was not consumed")
	}
}

func TestStorePressureUsesControlledClockAndClearPreservesDrops(t *testing.T) {
	now := time.Unix(100, 0)
	store := newStore(func() time.Time { return now })
	for index := 0; index < maxBefore+2; index++ {
		store.StoreBefore(fmt.Sprintf("corr-%d", index), performance.PerformanceSnapshot{URL: "https://app.local"})
	}
	now = now.Add(3 * time.Second)
	pressure := store.Pressure().BeforeSnapshots
	if pressure.Size != maxBefore || pressure.Dropped != 2 || pressure.OldestAge != 3*time.Second {
		t.Fatalf("before pressure = %#v", pressure)
	}

	store.Clear()
	pressure = store.Pressure().BeforeSnapshots
	if pressure.Size != 0 || pressure.Dropped != 2 || pressure.OldestAge != 0 {
		t.Fatalf("pressure after clear = %#v", pressure)
	}
}
