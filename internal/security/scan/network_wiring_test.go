// network_wiring_test.go — Verifies the network check is reachable through the scan dispatcher.
// Purpose: Tests that the network check is wired into the scan dispatcher.
// Docs: docs/features/feature/security-hardening/index.md

package scan

import (
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
	"strings"
	"testing"
)

// ============================================
// Network Security Check Wiring Tests
// ============================================

func TestScan_NetworkCheckDetectsSuspiciousTLD(t *testing.T) {
	scanner := NewScanner()

	input := Input{
		WaterfallEntries: []types.NetworkWaterfallEntry{
			{URL: "https://cdn-analytics.xyz/tracker.js", InitiatorType: "script"},
		},
		PageURLs: []string{"https://myapp.com"},
		Checks:   []string{"network"},
	}

	result := scanner.Scan(input)

	if len(result.Findings) == 0 {
		t.Fatal("Expected network findings for suspicious TLD .xyz")
	}

	found := false
	for _, f := range result.Findings {
		if f.Check == "network" && strings.Contains(f.Title, ".xyz") {
			found = true
			if f.Severity != "medium" {
				t.Errorf("Expected medium severity for .xyz TLD, got %s", f.Severity)
			}
		}
	}
	if !found {
		t.Error("Expected a 'network' finding mentioning .xyz TLD")
	}
}

func TestScan_NetworkCheckDetectsTyposquatting(t *testing.T) {
	scanner := NewScanner()

	input := Input{
		WaterfallEntries: []types.NetworkWaterfallEntry{
			{URL: "https://unpkg.cm/library.js", InitiatorType: "script"},
		},
		PageURLs: []string{"https://myapp.com"},
		Checks:   []string{"network"},
	}

	result := scanner.Scan(input)

	found := false
	for _, f := range result.Findings {
		if f.Check == "network" && f.Severity == "high" {
			found = true
		}
	}
	if !found {
		t.Error("Expected high-severity network finding for typosquatting domain unpkg.cm")
	}
}

func TestScan_NetworkCheckDetectsMixedContent(t *testing.T) {
	scanner := NewScanner()

	input := Input{
		WaterfallEntries: []types.NetworkWaterfallEntry{
			{URL: "http://cdn.example.com/script.js", InitiatorType: "script"},
		},
		PageURLs: []string{"https://myapp.com"},
		Checks:   []string{"network"},
	}

	result := scanner.Scan(input)

	found := false
	for _, f := range result.Findings {
		if f.Check == "network" && strings.Contains(f.Title, "mixed content") {
			found = true
			if f.Severity != "high" {
				t.Errorf("Expected high severity for script mixed content, got %s", f.Severity)
			}
		}
	}
	if !found {
		t.Error("Expected mixed content finding for HTTP script on HTTPS page")
	}
}

func TestScan_NetworkCheckSafeOriginNoFindings(t *testing.T) {
	scanner := NewScanner()

	input := Input{
		WaterfallEntries: []types.NetworkWaterfallEntry{
			{URL: "https://cdn.example.com/library.js", InitiatorType: "script"},
		},
		PageURLs: []string{"https://myapp.com"},
		Checks:   []string{"network"},
	}

	result := scanner.Scan(input)

	// URLsScanned counts network bodies, and this input is waterfall-based, so
	// the positive evidence here is discrimination: the same scanner over a
	// suspicious origin must produce the finding this case asserts the absence
	// of. Without it, "no network findings" is equally satisfied by a network
	// check that never ran.
	requireCheckDiscriminates(t, scanner.Scan(Input{
		WaterfallEntries: []types.NetworkWaterfallEntry{
			{URL: "https://cdn-analytics.xyz/tracker.js", InitiatorType: "script"},
		},
		PageURLs: []string{"https://myapp.com"},
		Checks:   []string{"network"},
	}), "network")

	for _, f := range result.Findings {
		if f.Check == "network" {
			t.Errorf("Safe origin should not produce network findings, got: %s", f.Title)
		}
	}
}

func TestScan_NetworkCheckIncludedByDefault(t *testing.T) {
	scanner := NewScanner()

	// No explicit Checks — should run all including "network"
	input := Input{
		WaterfallEntries: []types.NetworkWaterfallEntry{
			{URL: "https://cdn-analytics.xyz/tracker.js", InitiatorType: "script"},
		},
		PageURLs: []string{"https://myapp.com"},
	}

	result := scanner.Scan(input)

	found := false
	for _, f := range result.Findings {
		if f.Check == "network" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Network check should run by default when no checks specified")
	}
}
