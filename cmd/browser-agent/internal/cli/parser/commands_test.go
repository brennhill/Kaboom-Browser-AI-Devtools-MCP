// commands_test.go — Tests for CLI argument parser branches.
// Docs: docs/features/feature/mcp-persistent-server/index.md

package parser

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParserPackageRespectsTenFileBoundary(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	files := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			files++
		}
	}
	if files > 10 {
		t.Fatalf("CLI parser package has %d files; want at most 10 change-coupled owners", files)
	}
}

// ============================================
// ParseObserveArgs — uncovered flags
// ============================================

func TestParseObserveArgs_StatusMaxAndLastN(t *testing.T) {
	t.Parallel()

	result, err := ParseObserveArgs("network_bodies", []string{
		"--status-max", "499",
		"--last-n", "10",
	})
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if result["status_max"] != 499 {
		t.Fatalf("status_max = %v, want 499", result["status_max"])
	}
	if result["last_n"] != 10 {
		t.Fatalf("last_n = %v, want 10", result["last_n"])
	}
}

func TestParseObserveArgs_BodyPath(t *testing.T) {
	t.Parallel()

	result, err := ParseObserveArgs("network_bodies", []string{
		"--body-path", "data.items[0]",
	})
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if result["body_path"] != "data.items[0]" {
		t.Fatalf("body_path = %v, want data.items[0]", result["body_path"])
	}
}

func TestParseObserveArgs_CommandResultCorrelationID(t *testing.T) {
	t.Parallel()

	result, err := ParseObserveArgs("command_result", []string{"--correlation-id", "corr-123"})
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if result["correlation_id"] != "corr-123" {
		t.Fatalf("correlation_id = %v, want corr-123", result["correlation_id"])
	}
}

// ============================================
// ParseAnalyzeArgs — parity coverage
// ============================================

func TestParseAnalyzeArgs_BasicFlags(t *testing.T) {
	t.Parallel()

	result, err := ParseAnalyzeArgs("accessibility", []string{
		"--scope", "#app",
		"--tags", "wcag2a,wcag2aa",
		"--force-refresh",
		"--timeout-ms", "25000",
		"--frame", "all",
	})
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if result["what"] != "accessibility" {
		t.Fatalf("what = %v, want accessibility", result["what"])
	}
	if result["scope"] != "#app" {
		t.Fatalf("scope = %v, want #app", result["scope"])
	}
	tags, _ := result["tags"].([]string)
	if len(tags) != 2 || tags[0] != "wcag2a" || tags[1] != "wcag2aa" {
		t.Fatalf("tags = %#v, want [wcag2a wcag2aa]", result["tags"])
	}
	if result["force_refresh"] != true {
		t.Fatal("force_refresh should be true")
	}
	if result["timeout_ms"] != 25000 {
		t.Fatalf("timeout_ms = %v, want 25000", result["timeout_ms"])
	}
	if result["frame"] != "all" {
		t.Fatalf("frame = %v, want all", result["frame"])
	}
}

// ============================================
// ParseGenerateArgs — uncovered flags
// ============================================

func TestParseGenerateArgs_MethodAndBaseURL(t *testing.T) {
	t.Parallel()

	result, err := ParseGenerateArgs("har", []string{
		"--method", "POST",
		"--base-url", "http://localhost:3000",
		"--url", "https://api.example.com",
		"--include-screenshots",
	})
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if result["method"] != "POST" {
		t.Fatalf("method = %v, want POST", result["method"])
	}
	if result["base_url"] != "http://localhost:3000" {
		t.Fatalf("base_url = %v, want http://localhost:3000", result["base_url"])
	}
	if result["url"] != "https://api.example.com" {
		t.Fatalf("url = %v, want https://api.example.com", result["url"])
	}
	if result["include_screenshots"] != true {
		t.Fatal("include_screenshots should be true")
	}
}

// ============================================
// ParseConfigureArgs — uncovered flags
// ============================================

func TestParseConfigureArgs_DataPlainString(t *testing.T) {
	t.Parallel()

	result, err := ParseConfigureArgs("store", []string{"--data", "not-json"})
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if result["data"] != "not-json" {
		t.Fatalf("data = %v, want not-json (plain string fallback)", result["data"])
	}
}

func TestParseConfigureArgs_RemainingFlags(t *testing.T) {
	t.Parallel()

	result, err := ParseConfigureArgs("noise_rule", []string{
		"--rule-id", "rule-1",
		"--store-action", "save",
		"--pattern", "err-*",
		"--namespace", "test-ns",
	})
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	checks := map[string]string{
		"rule_id":      "rule-1",
		"store_action": "save",
		"pattern":      "err-*",
		"namespace":    "test-ns",
	}
	for k, want := range checks {
		if got, _ := result[k].(string); got != want {
			t.Errorf("result[%q] = %q, want %q", k, got, want)
		}
	}
}

// ============================================
// ParseInteractArgs — uncovered flags
// ============================================

func TestParseInteractArgs_TimeoutAndSubtitle(t *testing.T) {
	t.Parallel()

	result, err := ParseInteractArgs("click", []string{
		"--selector", "#btn",
		"--timeout-ms", "5000",
		"--subtitle", "Clicking button",
		"--reason", "testing flow",
		"--name", "submit",
		"--value", "42",
	})
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if result["timeout_ms"] != 5000 {
		t.Fatalf("timeout_ms = %v, want 5000", result["timeout_ms"])
	}
	if result["subtitle"] != "Clicking button" {
		t.Fatalf("subtitle = %v, want Clicking button", result["subtitle"])
	}
	if result["reason"] != "testing flow" {
		t.Fatalf("reason = %v, want testing flow", result["reason"])
	}
	if result["name"] != "submit" {
		t.Fatalf("name = %v, want submit", result["name"])
	}
	if result["value"] != "42" {
		t.Fatalf("value = %v, want 42", result["value"])
	}
}

func TestParseInteractArgs_FilePathRelative(t *testing.T) {
	t.Parallel()

	result, err := ParseInteractArgs("upload", []string{
		"--selector", "input[type=file]",
		"--file-path", "test.png",
	})
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	fp, _ := result["file_path"].(string)
	if !filepath.IsAbs(fp) {
		t.Fatalf("relative file_path should be resolved to absolute, got: %s", fp)
	}
}

func TestParseInteractArgs_FilePathAbsolute(t *testing.T) {
	t.Parallel()

	result, err := ParseInteractArgs("upload", []string{
		"--selector", "input[type=file]",
		"--file-path", "/tmp/test.png",
	})
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if result["file_path"] != "/tmp/test.png" {
		t.Fatalf("file_path = %v, want /tmp/test.png", result["file_path"])
	}
}

func TestParseInteractArgs_UploadWithAPIEndpointOnly(t *testing.T) {
	t.Parallel()

	result, err := ParseInteractArgs("upload", []string{
		"--api-endpoint", "https://example.com/upload",
		"--file-path", "/tmp/test.png",
		"--submit",
		"--escalation-timeout-ms", "12000",
	})
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if result["api_endpoint"] != "https://example.com/upload" {
		t.Fatalf("api_endpoint = %v, want https://example.com/upload", result["api_endpoint"])
	}
	if result["submit"] != true {
		t.Fatal("submit should be true")
	}
	if result["escalation_timeout_ms"] != 12000 {
		t.Fatalf("escalation_timeout_ms = %v, want 12000", result["escalation_timeout_ms"])
	}
}

func TestParseGenerateArgs_UnknownFlag(t *testing.T) {
	t.Parallel()

	_, err := ParseGenerateArgs("har", []string{"--not-a-flag"})
	if err == nil {
		t.Fatal("expected unknown flag error")
	}
}
