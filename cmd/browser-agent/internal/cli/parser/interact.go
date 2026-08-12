// interact.go — Tool-specific CLI flag-to-MCP argument mapping for interact actions.
// Why: Isolates interact parser contracts and validation from other tool parsers.
// Docs: docs/features/feature/enhanced-cli-config/index.md

package parser

import (
	"fmt"
	"os"
	"path/filepath"
)

func actionRequiresTarget(action string) bool {
	switch action {
	case "click", "type", "get_text", "get_value", "get_attribute",
		"set_attribute", "wait_for", "scroll_to", "focus", "check",
		"paste", "highlight":
		return true
	default:
		return false
	}
}

// ParseInteractArgs parses CLI flags for the interact tool into MCP arguments.
func ParseInteractArgs(action string, args []string) (map[string]any, error) {
	mcpArgs := map[string]any{"what": action}
	parsed, err := parseFlagsBySpec(args, map[string]cliFlagSpec{
		// Cross-cutting
		"--telemetry-mode": {MCPKey: "telemetry_mode", Kind: FlagString},
		"--background":     {MCPKey: "background", Kind: FlagBool},
		// Element targeting
		"--selector":         {MCPKey: "selector", Kind: FlagString},
		"--element-id":       {MCPKey: "element_id", Kind: FlagString},
		"--index":            {MCPKey: "index", Kind: FlagInt},
		"--index-generation": {MCPKey: "index_generation", Kind: FlagString},
		"--nth":              {MCPKey: "nth", Kind: FlagInt},
		"--scope-selector":   {MCPKey: "scope_selector", Kind: FlagString},
		"--scope-rect":       {MCPKey: "scope_rect", Kind: FlagJSON},
		"--frame":            {MCPKey: "frame", Kind: FlagIntOrString},
		"--x":                {MCPKey: "x", Kind: FlagInt},
		"--y":                {MCPKey: "y", Kind: FlagInt},
		// List/query filters
		"--visible-only":    {MCPKey: "visible_only", Kind: FlagBool},
		"--verbose":         {MCPKey: "verbose", Kind: FlagBool},
		"--limit":           {MCPKey: "limit", Kind: FlagInt},
		"--text-contains":   {MCPKey: "text_contains", Kind: FlagString},
		"--role":            {MCPKey: "role", Kind: FlagString},
		"--exclude-nav":     {MCPKey: "exclude_nav", Kind: FlagBool},
		"--query-type":      {MCPKey: "query_type", Kind: FlagString},
		"--attribute-names": {MCPKey: "attribute_names", Kind: FlagStringList},
		// Core action params
		"--text":        {MCPKey: "text", Kind: FlagString},
		"--value":       {MCPKey: "value", Kind: FlagString},
		"--name":        {MCPKey: "name", Kind: FlagString},
		"--clear":       {MCPKey: "clear", Kind: FlagBool},
		"--checked":     {MCPKey: "checked", Kind: FlagBool},
		"--direction":   {MCPKey: "direction", Kind: FlagString},
		"--structured":  {MCPKey: "structured", Kind: FlagBool},
		"--script":      {MCPKey: "script", Kind: FlagString},
		"--world":       {MCPKey: "world", Kind: FlagString},
		"--timeout-ms":  {MCPKey: "timeout_ms", Kind: FlagInt},
		"--duration-ms": {MCPKey: "duration_ms", Kind: FlagInt},
		"--subtitle":    {MCPKey: "subtitle", Kind: FlagString},
		// Navigation
		"--url":             {MCPKey: "url", Kind: FlagString},
		"--tab-id":          {MCPKey: "tab_id", Kind: FlagInt},
		"--tab-index":       {MCPKey: "tab_index", Kind: FlagInt},
		"--set-tracked":     {MCPKey: "set_tracked", Kind: FlagBool},
		"--new-tab":         {MCPKey: "new_tab", Kind: FlagBool},
		"--include-content": {MCPKey: "include_content", Kind: FlagBool},
		"--analyze":         {MCPKey: "analyze", Kind: FlagBool},
		// Wait / stability
		"--wait-for":            {MCPKey: "wait_for", Kind: FlagString},
		"--url-contains":        {MCPKey: "url_contains", Kind: FlagString},
		"--absent":              {MCPKey: "absent", Kind: FlagBool},
		"--wait-for-stable":     {MCPKey: "wait_for_stable", Kind: FlagBool},
		"--wait-for-url-change": {MCPKey: "wait_for_url_change", Kind: FlagBool},
		"--stability-ms":        {MCPKey: "stability_ms", Kind: FlagInt},
		"--auto-dismiss":        {MCPKey: "auto_dismiss", Kind: FlagBool},
		// Output enrichments
		"--include-screenshot":  {MCPKey: "include_screenshot", Kind: FlagBool},
		"--include-interactive": {MCPKey: "include_interactive", Kind: FlagBool},
		"--observe-mutations":   {MCPKey: "observe_mutations", Kind: FlagBool},
		"--action-diff":         {MCPKey: "action_diff", Kind: FlagBool},
		"--evidence":            {MCPKey: "evidence", Kind: FlagString},
		"--reason":              {MCPKey: "reason", Kind: FlagString},
		"--correlation-id":      {MCPKey: "correlation_id", Kind: FlagString},
		// State management
		"--snapshot-name": {MCPKey: "snapshot_name", Kind: FlagString},
		"--include-url":   {MCPKey: "include_url", Kind: FlagBool},
		"--storage-type":  {MCPKey: "storage_type", Kind: FlagString},
		"--key":           {MCPKey: "key", Kind: FlagString},
		"--domain":        {MCPKey: "domain", Kind: FlagString},
		"--path":          {MCPKey: "path", Kind: FlagString},
		// Form filling
		"--fields":          {MCPKey: "fields", Kind: FlagJSON},
		"--submit-selector": {MCPKey: "submit_selector", Kind: FlagString},
		"--submit-index":    {MCPKey: "submit_index", Kind: FlagInt},
		// Recording
		"--audio":         {MCPKey: "audio", Kind: FlagString},
		"--fps":           {MCPKey: "fps", Kind: FlagInt},
		"--annot-session": {MCPKey: "annot_session", Kind: FlagString},
		// Upload
		"--file-path":             {MCPKey: "file_path", Kind: FlagString},
		"--api-endpoint":          {MCPKey: "api_endpoint", Kind: FlagString},
		"--submit":                {MCPKey: "submit", Kind: FlagBool},
		"--escalation-timeout-ms": {MCPKey: "escalation_timeout_ms", Kind: FlagInt},
		// Batch
		"--steps":             {MCPKey: "steps", Kind: FlagJSON},
		"--step-timeout-ms":   {MCPKey: "step_timeout_ms", Kind: FlagInt},
		"--continue-on-error": {MCPKey: "continue_on_error", Kind: FlagBool},
		"--stop-after-step":   {MCPKey: "stop_after_step", Kind: FlagInt},
		// Save output
		"--save-to": {MCPKey: "save_to", Kind: FlagString},
	})
	if err != nil {
		return nil, err
	}
	for k, v := range parsed {
		mcpArgs[k] = v
	}
	parseInteractFilePath(mcpArgs)

	return mcpArgs, validateInteractArgs(action, mcpArgs)
}

// parseInteractFilePath extracts --file-path and resolves relative paths to absolute.
func parseInteractFilePath(mcpArgs map[string]any) {
	filePath, _ := mcpArgs["file_path"].(string)
	if filePath == "" {
		return
	}
	if !filepath.IsAbs(filePath) {
		if cwd, err := os.Getwd(); err == nil {
			filePath = filepath.Join(cwd, filePath)
		}
	}
	mcpArgs["file_path"] = filePath
}

// validateInteractArgs checks required fields for specific interact actions.
func validateInteractArgs(action string, mcpArgs map[string]any) error {
	if actionRequiresTarget(action) && !hasTargetingParam(mcpArgs) {
		return fmt.Errorf("interact %s: requires a targeting param (--selector, --element-id, --index, or --x/--y)", action)
	}
	if action == "upload" && mcpArgs["selector"] == nil && mcpArgs["element_id"] == nil && mcpArgs["api_endpoint"] == nil {
		return fmt.Errorf("interact upload: --selector, --element-id, or --api-endpoint is required")
	}
	if action == "navigate" && mcpArgs["url"] == nil {
		return fmt.Errorf("interact navigate: --url is required")
	}
	if action == "execute_js" && mcpArgs["script"] == nil {
		return fmt.Errorf("interact execute_js: --script is required")
	}
	return nil
}

// hasTargetingParam checks if at least one element targeting param is present.
func hasTargetingParam(mcpArgs map[string]any) bool {
	for _, key := range []string{"selector", "element_id", "index", "x", "y"} {
		if mcpArgs[key] != nil {
			return true
		}
	}
	return false
}
