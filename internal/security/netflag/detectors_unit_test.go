// detectors_unit_test.go — Branch tests for the individual netflag detectors.
// Purpose: Unit tests for security flagging logic.
// Docs: docs/features/feature/security-hardening/index.md

package netflag

import (
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
	"testing"
)

func TestCheckNonStandardPortBranches(t *testing.T) {
	t.Parallel()

	if got := checkNonStandardPort("://bad-url"); got != nil {
		t.Fatalf("invalid URL should not produce a flag, got %+v", got)
	}
	if got := checkNonStandardPort("https://example.com"); got != nil {
		t.Fatalf("default HTTPS port should not be flagged, got %+v", got)
	}
	if got := checkNonStandardPort("https://example.com:443"); got != nil {
		t.Fatalf("standard HTTPS port should not be flagged, got %+v", got)
	}
	if got := checkNonStandardPort("http://localhost:3000"); got != nil {
		t.Fatalf("localhost dev port should not be flagged, got %+v", got)
	}
	if got := checkNonStandardPort("http://localhost:9999"); got == nil {
		t.Fatal("non-whitelisted localhost port should be flagged")
	}
	if got := checkNonStandardPort("https://example.com:8443"); got == nil {
		t.Fatal("non-standard external port should be flagged")
	}
}

func TestFlaggingInputValidationBranches(t *testing.T) {
	t.Parallel()

	if got := checkSuspiciousTLD("://bad-url"); got != nil {
		t.Fatalf("invalid URL should not be flagged for suspicious TLD, got %+v", got)
	}
	if got := checkIPAddressOrigin("://bad-url"); got != nil {
		t.Fatalf("invalid URL should not be flagged for IP origin, got %+v", got)
	}

	if got := checkMixedContent(types.NetworkWaterfallEntry{URL: "http://cdn.example.com/a.js"}, "://bad-page-url"); got != nil {
		t.Fatalf("invalid page URL should not produce mixed-content flag, got %+v", got)
	}
	if got := checkMixedContent(types.NetworkWaterfallEntry{URL: "://bad-entry-url"}, "https://example.com"); got != nil {
		t.Fatalf("invalid resource URL should not produce mixed-content flag, got %+v", got)
	}

	flags := Analyze(types.NetworkWaterfallEntry{URL: "://bad-url"}, "https://example.com")
	if len(flags) != 0 {
		t.Fatalf("invalid entry URL should return no flags, got %v", flags)
	}
}

func TestCheckMixedContentSeverityByInitiatorType(t *testing.T) {
	t.Parallel()

	scriptFlag := checkMixedContent(
		types.NetworkWaterfallEntry{
			URL:           "http://cdn.example.com/script.js",
			InitiatorType: "script",
		},
		"https://app.example.com",
	)
	if scriptFlag == nil || scriptFlag.Severity != "high" {
		t.Fatalf("script mixed-content severity = %+v, want high", scriptFlag)
	}

	imageFlag := checkMixedContent(
		types.NetworkWaterfallEntry{
			URL:           "http://cdn.example.com/image.png",
			InitiatorType: "img",
		},
		"https://app.example.com",
	)
	if imageFlag == nil || imageFlag.Severity != "medium" {
		t.Fatalf("non-script mixed-content severity = %+v, want medium", imageFlag)
	}
}
