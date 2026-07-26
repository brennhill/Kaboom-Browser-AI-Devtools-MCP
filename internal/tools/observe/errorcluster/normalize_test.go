// Purpose: Tests error-message normalization against a regex reference implementation.
// Why: The production normalizer is a hand-written byte scanner chosen for speed; a slow,
// obviously-correct regex version lives here as the oracle so the fast path cannot drift.
// Docs: docs/features/feature/error-clustering/index.md

package errorcluster

import (
	"math/rand"
	"regexp"
	"strings"
	"testing"
)

// referenceNormalize is the original implementation from internal/analysis/clustering,
// kept verbatim as a test oracle. It is the readable statement of intent;
// normalizeErrorMessage is the fast equivalent that ships. Four sequential
// ReplaceAllString passes cost ~2.8us and 13 allocations per message, which at the
// 10,000-entry cap turned one analyze call into 28ms of regex work.
func referenceNormalize(msg string) string {
	result := refUUIDRegex.ReplaceAllString(msg, "{uuid}")
	result = refURLRegex.ReplaceAllString(result, "{url}")
	result = refTimestampRegex.ReplaceAllString(result, "{timestamp}")
	result = refNumericIDRegex.ReplaceAllString(result, "{id}")
	return result
}

var (
	refUUIDRegex      = regexp.MustCompile(`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`)
	refURLRegex       = regexp.MustCompile(`https?://[^\s"']+`)
	refTimestampRegex = regexp.MustCompile(`\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}[^\s]*`)
	refNumericIDRegex = regexp.MustCompile(`\b\d{3,}\b`)
)

func TestNormalizeErrorMessage_Table(t *testing.T) {
	cases := []struct{ name, in, want string }{
		{"no dynamic tokens is returned untouched",
			"Uncaught ReferenceError: analytics is not defined",
			"Uncaught ReferenceError: analytics is not defined"},
		{"numeric id",
			"Cannot read property 'id' of undefined at /users/12345",
			"Cannot read property 'id' of undefined at /users/{id}"},
		{"two-digit run is not an id",
			"retry 42 failed",
			"retry 42 failed"},
		{"digits glued to a word are not an id",
			"module abc123 missing",
			"module abc123 missing"},
		{"url",
			"fetch failed for https://api.example.com/v2/orders?x=1",
			"fetch failed for {url}"},
		{"uuid",
			"request 3f8a1c2e-9b4d-4f1a-8c7e-2d5b6a9f0e31 timed out",
			"request {uuid} timed out"},
		{"timestamp",
			"failed at 2026-07-26T14:33:00Z while hydrating",
			"failed at {timestamp} while hydrating"},
		{"sibling errors collapse to one key",
			"Cannot read property 'id' of undefined at /users/67890",
			"Cannot read property 'id' of undefined at /users/{id}"},
		{"digit run abutting a timestamp splits",
			"9992026-07-26T14:33:00Z",
			"{id}{timestamp}"},
		{"digit run abutting a uuid splits",
			"999deadbeef-0000-1111-2222-333344445555",
			"{id}{uuid}"},
		{"empty", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := normalizeErrorMessage(tc.in); got != tc.want {
				t.Fatalf("normalizeErrorMessage(%q) = %q, want %q", tc.in, got, tc.want)
			}
			// The oracle must agree, or the expectation itself is wrong.
			if ref := referenceNormalize(tc.in); ref != tc.want {
				t.Fatalf("reference disagrees with expectation for %q: got %q, want %q", tc.in, ref, tc.want)
			}
		})
	}
}

var normalizeFragments = []string{
	"Cannot read property", "of undefined", "at /users/", "12345", "42", "7",
	"https://api.example.com/v2/x?y=1", "http://a.b/c",
	"3f8a1c2e-9b4d-4f1a-8c7e-2d5b6a9f0e31", "deadbeef-0000-1111-2222-333344445555",
	"2026-07-26T14:33:00Z", "ver2026-01-02T00:00:00", "TypeError:",
	"abc123", "x_9999", "999", "1.2.3", "id=00042", "  ", "\"", "'",
}

// TestNormalizeErrorMessage_MatchesReference is the real guard. The scanner collapses
// four sequential global regex passes into one left-to-right pass, and that collapse has
// non-obvious edge cases: a rewritten span manufactures a word boundary that the original
// text lacks, and an earlier pass can claim a match starting partway inside a digit run.
// Both bugs were present in the first draft and both were caught here, not by the table above.
//
// The case count is deliberately modest. An earlier revision ran 200,000 cases, which cost
// ~10s of saturated CPU under -race locally and considerably more on a two-core CI runner —
// enough contention to push the timing-sensitive daemon tests in cmd/browser-agent from 206s
// past their 600s package timeout. Both known divergences reproduce within the first few
// hundred cases; exhaustive search belongs in FuzzNormalizeErrorMessage, which runs on demand.
func TestNormalizeErrorMessage_MatchesReference(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	for n := 0; n < 3000; n++ {
		var sb strings.Builder
		for k := rng.Intn(6); k >= 0; k-- {
			sb.WriteString(normalizeFragments[rng.Intn(len(normalizeFragments))])
			if rng.Intn(2) == 0 {
				sb.WriteByte(' ')
			}
		}
		in := sb.String()
		if want, got := referenceNormalize(in), normalizeErrorMessage(in); want != got {
			t.Fatalf("scanner diverged from reference on %q:\n  reference=%q\n  scanner  =%q", in, want, got)
		}
	}
}

// FuzzNormalizeErrorMessage is where unbounded search lives. Under plain `go test` it
// replays only the seed corpus (cheap); run `go test -fuzz=FuzzNormalizeErrorMessage
// ./internal/tools/observe/errorcluster/` to search properly.
func FuzzNormalizeErrorMessage(f *testing.F) {
	for _, s := range normalizeFragments {
		f.Add(s)
	}
	f.Add("deadbeef-0000-1111-2222-333344445555999 42") // boundary manufactured by a rewrite
	f.Add("9992026-07-26T14:33:00Z")                    // earlier pass claims the tail of a digit run
	f.Add("")

	f.Fuzz(func(t *testing.T, in string) {
		if want, got := referenceNormalize(in), normalizeErrorMessage(in); want != got {
			t.Fatalf("scanner diverged from reference on %q:\n  reference=%q\n  scanner  =%q", in, want, got)
		}
	})
}

func BenchmarkNormalizeErrorMessage(b *testing.B) {
	msg := "TypeError: fetch failed for https://api.example.com/v2/orders/4821?retry=true"
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = normalizeErrorMessage(msg)
	}
}

func BenchmarkNormalizeErrorMessage_Reference(b *testing.B) {
	msg := "TypeError: fetch failed for https://api.example.com/v2/orders/4821?retry=true"
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = referenceNormalize(msg)
	}
}
