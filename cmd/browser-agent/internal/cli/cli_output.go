// cli_output.go — Formats MCP tool results for human, JSON, and CSV CLI output modes.
// Why: Decouples output rendering from tool execution so CLI and MCP share the same result pipeline.
// Docs: docs/features/feature/enhanced-cli-config/index.md

package cli

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

// CLIResult holds parsed output data for formatting.
type CLIResult struct {
	Success     bool
	Tool        string
	Action      string
	TextContent string
	Error       string
	Data        map[string]any
}

// FormatResult formats the MCP tool result as human-facing diagnostic output.
func FormatResult(output io.Writer, format, tool, action string, result *mcp.MCPToolResult) int {
	cliRes := BuildCLIResult(tool, action, result)
	var err error

	switch format {
	case "json":
		err = FormatJSON(output, cliRes)
	case "csv":
		err = FormatCSV(output, cliRes)
	default:
		err = FormatHuman(output, cliRes)
	}

	if err != nil {
		fmt.Fprintf(output, "[Kaboom] output error: %v\n", err)
		return 1
	}

	if !cliRes.Success {
		return 1
	}
	return 0
}

// BuildCLIResult extracts text content from MCPToolResult and attempts JSON parse.
func BuildCLIResult(tool, action string, result *mcp.MCPToolResult) *CLIResult {
	r := &CLIResult{
		Success: !result.IsError,
		Tool:    tool,
		Action:  action,
	}

	// Concatenate text from content blocks
	var parts []string
	for _, block := range result.Content {
		if block.Text != "" {
			parts = append(parts, block.Text)
		}
	}
	r.TextContent = strings.Join(parts, "\n")

	if result.IsError {
		r.Error = r.TextContent
		return r
	}

	// Try to parse as JSON for structured output
	var data map[string]any
	if err := json.Unmarshal([]byte(r.TextContent), &data); err == nil {
		r.Data = data
	}

	return r
}

// FormatHuman writes human-readable output.
func FormatHuman(w io.Writer, r *CLIResult) error {
	var sb strings.Builder

	if r.Success {
		sb.WriteString(fmt.Sprintf("[OK] %s %s\n", r.Tool, r.Action))
	} else {
		sb.WriteString(fmt.Sprintf("[Error] %s %s\n", r.Tool, r.Action))
		if r.Error != "" {
			sb.WriteString(fmt.Sprintf("  %s\n", r.Error))
		}
	}

	if r.TextContent != "" {
		sb.WriteString("\n")
		sb.WriteString(r.TextContent)
		if !strings.HasSuffix(r.TextContent, "\n") {
			sb.WriteString("\n")
		}
	}

	_, err := io.WriteString(w, sb.String())
	return err
}

// FormatJSON writes pretty-printed JSON output with merged data fields.
func FormatJSON(w io.Writer, r *CLIResult) error {
	out := map[string]any{
		"success": r.Success,
		"tool":    r.Tool,
		"action":  r.Action,
	}

	if r.Error != "" {
		out["error"] = r.Error
	}

	// Merge data fields into output
	for k, v := range r.Data {
		out[k] = v
	}

	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	_, err = w.Write(data)
	return err
}

// FormatCSV writes CSV output with header + data row.
func FormatCSV(w io.Writer, r *CLIResult) error {
	// Collect data keys for additional columns
	var dataKeys []string
	for k := range r.Data {
		dataKeys = append(dataKeys, k)
	}
	sort.Strings(dataKeys)

	header := []string{"success", "tool", "action", "error"}
	header = append(header, dataKeys...)

	var sb strings.Builder
	cw := csv.NewWriter(&sb)

	if err := cw.Write(header); err != nil {
		return fmt.Errorf("write CSV header: %w", err)
	}

	row := []string{
		fmt.Sprintf("%t", r.Success),
		r.Tool,
		r.Action,
		r.Error,
	}
	for _, k := range dataKeys {
		val := ""
		if v, ok := r.Data[k]; ok {
			val = fmt.Sprintf("%v", v)
		}
		row = append(row, val)
	}

	if err := cw.Write(row); err != nil {
		return fmt.Errorf("write CSV row: %w", err)
	}

	cw.Flush()
	if err := cw.Error(); err != nil {
		return err
	}

	_, err := io.WriteString(w, sb.String())
	return err
}
