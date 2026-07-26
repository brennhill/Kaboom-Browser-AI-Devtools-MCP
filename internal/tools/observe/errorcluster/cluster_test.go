// Purpose: Tests error clustering grouping rules and deterministic response ordering.
// Docs: docs/features/feature/error-clustering/index.md

package errorcluster

import (
	"encoding/json"
	"fmt"
	"testing"
)

func entry(level, msg, ts, url, stack string) map[string]any {
	return map[string]any{
		"level": level, "message": msg, "timestamp": ts, "url": url, "stackTrace": stack,
	}
}

// TestAnalyze_SiblingsCollapse is the regression test for the defect this package fixes:
// keying on the raw message made every error carrying an id its own singleton "cluster".
func TestAnalyze_SiblingsCollapse(t *testing.T) {
	entries := []map[string]any{
		entry("error", "Cannot read property 'id' of undefined at /users/12345", "t1", "https://a/1", ""),
		entry("error", "Cannot read property 'id' of undefined at /users/67890", "t2", "https://a/2", ""),
		entry("error", "Cannot read property 'id' of undefined at /users/24680", "t3", "https://a/1", ""),
	}
	got := Analyze(entries)
	if len(got) != 1 {
		t.Fatalf("want 1 cluster, got %d: %+v", len(got), got)
	}
	if got[0]["count"] != 3 {
		t.Fatalf("want count 3, got %v", got[0]["count"])
	}
	// The representative stays the first raw message, not the placeholder form.
	if got[0]["message"] != "Cannot read property 'id' of undefined at /users/12345" {
		t.Fatalf("unexpected representative message: %v", got[0]["message"])
	}
	if got[0]["first_seen"] != "t1" || got[0]["last_seen"] != "t3" {
		t.Fatalf("want first_seen=t1 last_seen=t3, got %v/%v", got[0]["first_seen"], got[0]["last_seen"])
	}
	urls, _ := got[0]["urls"].([]string)
	if len(urls) != 2 || urls[0] != "https://a/1" || urls[1] != "https://a/2" {
		t.Fatalf("want deduped sorted urls [https://a/1 https://a/2], got %v", urls)
	}
}

func TestAnalyze_DistinctErrorsStaySeparate(t *testing.T) {
	entries := []map[string]any{
		entry("error", "Cannot read property 'id' of undefined", "t1", "", ""),
		entry("error", "NetworkError: connection refused", "t2", "", ""),
	}
	if got := Analyze(entries); len(got) != 2 {
		t.Fatalf("want 2 clusters, got %d: %+v", len(got), got)
	}
}

func TestAnalyze_IgnoresNonErrorsAndEmptyMessages(t *testing.T) {
	entries := []map[string]any{
		entry("warn", "a warning", "t1", "", ""),
		entry("info", "some info", "t2", "", ""),
		entry("error", "", "t3", "", ""),
		entry("error", "real failure", "t4", "", ""),
	}
	got := Analyze(entries)
	if len(got) != 1 {
		t.Fatalf("want 1 cluster, got %d: %+v", len(got), got)
	}
	if got[0]["message"] != "real failure" {
		t.Fatalf("wrong cluster survived: %v", got[0]["message"])
	}
}

func TestAnalyze_OrderedByCountDescending(t *testing.T) {
	var entries []map[string]any
	for i := 0; i < 2; i++ {
		entries = append(entries, entry("error", "rare failure", "t", "", ""))
	}
	for i := 0; i < 7; i++ {
		entries = append(entries, entry("error", "common failure", "t", "", ""))
	}
	for i := 0; i < 4; i++ {
		entries = append(entries, entry("error", "middling failure", "t", "", ""))
	}
	got := Analyze(entries)
	if len(got) != 3 {
		t.Fatalf("want 3 clusters, got %d", len(got))
	}
	want := []int{7, 4, 2}
	for i, w := range want {
		if got[i]["count"] != w {
			t.Fatalf("cluster %d: want count %d, got %v (full: %+v)", i, w, got[i]["count"], got)
		}
	}
}

// TestAnalyze_Deterministic pins the fix for the map-iteration defect. Both the cluster
// list and each url list were built by ranging a Go map, whose iteration order is
// randomized per run, so identical input produced differently-ordered output every call.
func TestAnalyze_Deterministic(t *testing.T) {
	var entries []map[string]any
	for i := 0; i < 12; i++ {
		// Equal counts across many clusters maximizes the chance a map-order
		// regression shows up: the tie-break is the only thing imposing order.
		msg := fmt.Sprintf("failure of kind %c", 'a'+rune(i))
		for r := 0; r < 3; r++ {
			entries = append(entries, entry("error", msg, "t", fmt.Sprintf("https://h%d/p", r), ""))
		}
	}

	first, err := json.Marshal(Analyze(entries))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for i := 0; i < 50; i++ {
		next, err := json.Marshal(Analyze(entries))
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		if string(next) != string(first) {
			t.Fatalf("call %d differed from call 0:\n first=%s\n next =%s", i+1, first, next)
		}
	}
}

func TestAnalyze_EmptyInput(t *testing.T) {
	got := Analyze(nil)
	if got == nil {
		t.Fatal("want empty non-nil slice so the JSON field serializes as [] not null")
	}
	if len(got) != 0 {
		t.Fatalf("want 0 clusters, got %d", len(got))
	}
}
