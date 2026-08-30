// semver_test.go — Pins the version ordering that decides daemon takeover and
// upgrade prompts. Moved here from daemonlife so one comparison serves every caller.

package semver_test

import (
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/semver"
)

func TestParts(t *testing.T) {
	tests := []struct {
		input string
		want  []int
	}{
		{"1.2.3", []int{1, 2, 3}},
		{"v1.2.3", []int{1, 2, 3}},
		{"0.9.0", []int{0, 9, 0}},
		{"1", []int{1}},
		{"1.2", []int{1, 2}},
		{"", nil},
		{"abc", nil},
		{"1.x.3", []int{1}},
	}
	for _, test := range tests {
		got := semver.Parts(test.input)
		if len(got) != len(test.want) {
			t.Errorf("Parts(%q) = %v, want %v", test.input, got, test.want)
			continue
		}
		for index := range got {
			if got[index] != test.want[index] {
				t.Errorf("Parts(%q)[%d] = %d, want %d", test.input, index, got[index], test.want[index])
			}
		}
	}
}

func TestPartsMalformedNeverNegative(t *testing.T) {
	for _, input := range []string{"-1.2.3", "1.-2.3", "v-1", "..", "1..2"} {
		for _, part := range semver.Parts(input) {
			if part < 0 {
				t.Errorf("Parts(%q) returned negative part: %d", input, part)
			}
		}
	}
}

func TestIsNewer(t *testing.T) {
	tests := []struct {
		candidate, current string
		want               bool
	}{
		{"1.2.4", "1.2.3", true},
		{"1.3.0", "1.2.9", true},
		{"2.0.0", "1.9.9", true},
		{"1.2.3", "1.2.3", false},
		{"1.2.2", "1.2.3", false},
		{"v1.2.4", "1.2.3", true},
		{"1.2.4", "v1.2.3", true},
		{"1.2", "1.2.0", false},
		{"1.2.1", "1.2", true},
		{"", "1.2.3", false},
		{"1.2.3", "", false},
		{"garbage", "1.2.3", false},
		{"1.2.3", "garbage", false},
	}
	for _, test := range tests {
		if got := semver.IsNewer(test.candidate, test.current); got != test.want {
			t.Errorf("IsNewer(%q, %q) = %v, want %v", test.candidate, test.current, got, test.want)
		}
	}
}

// Equality must be symmetric under IsNewer: neither side newer means same version.
// daemonlife's install-epoch tiebreaker depends on exactly this.
func TestSameVersionIsNeitherNewer(t *testing.T) {
	if semver.IsNewer("0.9.0", "v0.9.0") || semver.IsNewer("v0.9.0", "0.9.0") {
		t.Error("IsNewer() reported a difference between equal versions")
	}
}

// Same is the equal-version predicate daemonlife's install-epoch tiebreaker keys
// off: at the same version, the newer install wins, so "same" must be exact and
// must never be true when either side is unknown.
func TestSame(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"0.9.0", "0.9.0", true},
		{"v0.9.0", "0.9.0", true},
		{"0.9.0", "v0.9.0", true},
		{"0.8.8", "0.9.0", false},
		{"0.9.0", "0.8.8", false},
		{"", "0.8.8", false},
		{"0.8.8", "", false},
		{"", "", false},
	}
	for _, c := range cases {
		if got := semver.Same(c.a, c.b); got != c.want {
			t.Errorf("Same(%q, %q) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}
