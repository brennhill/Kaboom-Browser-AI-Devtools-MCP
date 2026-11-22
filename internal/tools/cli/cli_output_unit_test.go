// cli_output_unit_test.go — Unit tests for CLI output formatters.
// Docs: docs/features/feature/mcp-persistent-server/index.md

package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/mcp"
)

// ============================================
// BuildCLIResult
// ============================================

func TestBuildCLIResult_Success(t *testing.T) {
	t.Parallel()

	result := &mcp.MCPToolResult{
		Content: []mcp.MCPContentBlock{{Type: "text", Text: `{"key":"value"}`}},
		IsError: false,
	}
	cli := BuildCLIResult("observe", "logs", result)

	if !cli.Success {
		t.Fatal("Success should be true")
	}
	if cli.Tool != "observe" {
		t.Fatalf("Tool = %q, want observe", cli.Tool)
	}
	if cli.Action != "logs" {
		t.Fatalf("Action = %q, want logs", cli.Action)
	}
	if cli.Data == nil {
		t.Fatal("Data should be parsed from JSON content")
	}
	if cli.Data["key"] != "value" {
		t.Fatalf("Data[key] = %v, want value", cli.Data["key"])
	}
	if cli.Error != "" {
		t.Fatalf("Error = %q, want empty", cli.Error)
	}
}

func TestBuildCLIResult_Error(t *testing.T) {
	t.Parallel()

	result := &mcp.MCPToolResult{
		Content: []mcp.MCPContentBlock{{Type: "text", Text: "Something went wrong"}},
		IsError: true,
	}
	cli := BuildCLIResult("interact", "click", result)

	if cli.Success {
		t.Fatal("Success should be false")
	}
	if cli.Error != "Something went wrong" {
		t.Fatalf("Error = %q, want 'Something went wrong'", cli.Error)
	}
}

func TestBuildCLIResult_NonJSON(t *testing.T) {
	t.Parallel()

	result := &mcp.MCPToolResult{
		Content: []mcp.MCPContentBlock{{Type: "text", Text: "plain text output"}},
		IsError: false,
	}
	cli := BuildCLIResult("configure", "health", result)

	if cli.Data != nil {
		t.Fatal("Data should be nil for non-JSON text")
	}
	if cli.TextContent != "plain text output" {
		t.Fatalf("TextContent = %q, want 'plain text output'", cli.TextContent)
	}
}

func TestBuildCLIResult_MultipleBlocks(t *testing.T) {
	t.Parallel()

	result := &mcp.MCPToolResult{
		Content: []mcp.MCPContentBlock{
			{Type: "text", Text: "first"},
			{Type: "text", Text: "second"},
		},
		IsError: false,
	}
	cli := BuildCLIResult("observe", "page", result)

	if cli.TextContent != "first\nsecond" {
		t.Fatalf("TextContent = %q, want 'first\\nsecond'", cli.TextContent)
	}
}

// ============================================
// FormatHuman
// ============================================

func TestFormatHuman_Success(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	r := &CLIResult{
		Success:     true,
		Tool:        "observe",
		Action:      "logs",
		TextContent: "Log output here",
	}
	if err := FormatHuman(&buf, r); err != nil {
		t.Fatalf("FormatHuman error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "[OK] observe logs") {
		t.Errorf("output should contain [OK], got: %s", output)
	}
	if !strings.Contains(output, "Log output here") {
		t.Errorf("output should contain text content, got: %s", output)
	}
}

func TestFormatHuman_Error(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	r := &CLIResult{
		Success: false,
		Tool:    "interact",
		Action:  "click",
		Error:   "Element not found",
	}
	if err := FormatHuman(&buf, r); err != nil {
		t.Fatalf("FormatHuman error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "[Error] interact click") {
		t.Errorf("output should contain [Error], got: %s", output)
	}
	if !strings.Contains(output, "Element not found") {
		t.Errorf("output should contain error message, got: %s", output)
	}
}

// ============================================
// FormatJSON
// ============================================

func TestFormatJSON_Success(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	r := &CLIResult{
		Success: true,
		Tool:    "observe",
		Action:  "vitals",
		Data:    map[string]any{"lcp": 2500, "fcp": 1200},
	}
	if err := FormatJSON(&buf, r); err != nil {
		t.Fatalf("FormatJSON error: %v", err)
	}

	var out map[string]any
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if out["success"] != true {
		t.Fatalf("success = %v, want true", out["success"])
	}
	if out["tool"] != "observe" {
		t.Fatalf("tool = %v, want observe", out["tool"])
	}
	// Data fields should be merged into output
	if out["lcp"] == nil {
		t.Error("data fields should be merged into output")
	}
}

func TestFormatJSON_Error(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	r := &CLIResult{
		Success: false,
		Tool:    "configure",
		Action:  "store",
		Error:   "Permission denied",
	}
	if err := FormatJSON(&buf, r); err != nil {
		t.Fatalf("FormatJSON error: %v", err)
	}

	var out map[string]any
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if out["error"] != "Permission denied" {
		t.Fatalf("error = %v, want Permission denied", out["error"])
	}
}

// ============================================
// FormatCSV
// ============================================

func TestFormatCSV_Success(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	r := &CLIResult{
		Success: true,
		Tool:    "observe",
		Action:  "vitals",
		Data:    map[string]any{"lcp": 2500.0},
	}
	if err := FormatCSV(&buf, r); err != nil {
		t.Fatalf("FormatCSV error: %v", err)
	}

	output := buf.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) < 2 {
		t.Fatalf("CSV should have header + data row, got %d lines", len(lines))
	}
	// Header should contain standard columns
	if !strings.Contains(lines[0], "success") {
		t.Errorf("header should contain 'success', got: %s", lines[0])
	}
	if !strings.Contains(lines[0], "lcp") {
		t.Errorf("header should contain data key 'lcp', got: %s", lines[0])
	}
	// Data row should contain values
	if !strings.Contains(lines[1], "true") {
		t.Errorf("data row should contain 'true', got: %s", lines[1])
	}
}

func TestFormatCSV_Error(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	r := &CLIResult{
		Success: false,
		Tool:    "interact",
		Action:  "type",
		Error:   "timeout",
	}
	if err := FormatCSV(&buf, r); err != nil {
		t.Fatalf("FormatCSV error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "false") {
		t.Errorf("data row should contain 'false', got: %s", output)
	}
	if !strings.Contains(output, "timeout") {
		t.Errorf("data row should contain error, got: %s", output)
	}
}
