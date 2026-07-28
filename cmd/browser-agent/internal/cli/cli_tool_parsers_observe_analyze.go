// cli_tool_parsers_observe_analyze.go — Tool-specific CLI flag-to-MCP argument mapping for observe/analyze.
// Why: Keeps observe/analyze parser contracts isolated from other tool parser families.
// Docs: docs/features/feature/enhanced-cli-config/index.md

package cli

// ParseObserveArgs parses CLI flags for the observe tool into MCP arguments.
func ParseObserveArgs(mode string, args []string) (map[string]any, error) {
	mcpArgs := map[string]any{"what": mode}
	parsed, err := ParseFlagsBySpec(args, map[string]CLIFlagSpec{
		// Cross-cutting
		"--telemetry-mode": {MCPKey: "telemetry_mode", Kind: FlagString},
		"--limit":          {MCPKey: "limit", Kind: FlagInt},
		"--summary":        {MCPKey: "summary", Kind: FlagBool},
		"--scope":          {MCPKey: "scope", Kind: FlagString},
		// Pagination
		"--after-cursor":        {MCPKey: "after_cursor", Kind: FlagString},
		"--before-cursor":       {MCPKey: "before_cursor", Kind: FlagString},
		"--since-cursor":        {MCPKey: "since_cursor", Kind: FlagString},
		"--restart-on-eviction": {MCPKey: "restart_on_eviction", Kind: FlagBool},
		// Filtering
		"--min-level":      {MCPKey: "min_level", Kind: FlagString},
		"--source":         {MCPKey: "source", Kind: FlagString},
		"--url":            {MCPKey: "url", Kind: FlagString},
		"--method":         {MCPKey: "method", Kind: FlagString},
		"--status-min":     {MCPKey: "status_min", Kind: FlagInt},
		"--status-max":     {MCPKey: "status_max", Kind: FlagInt},
		"--body-path":      {MCPKey: "body_path", Kind: FlagString},
		"--connection-id":  {MCPKey: "connection_id", Kind: FlagString},
		"--direction":      {MCPKey: "direction", Kind: FlagString},
		"--last-n":         {MCPKey: "last_n", Kind: FlagInt},
		"--include":        {MCPKey: "include", Kind: FlagStringList},
		"--correlation-id": {MCPKey: "correlation_id", Kind: FlagString},
		"--recording-id":   {MCPKey: "recording_id", Kind: FlagString},
		"--window-seconds": {MCPKey: "window_seconds", Kind: FlagInt},
		"--original-id":    {MCPKey: "original_id", Kind: FlagString},
		"--replay-id":      {MCPKey: "replay_id", Kind: FlagString},
		// Log detail
		"--include-internal":       {MCPKey: "include_internal", Kind: FlagBool},
		"--include-extension-logs": {MCPKey: "include_extension_logs", Kind: FlagBool},
		"--extension-limit":        {MCPKey: "extension_limit", Kind: FlagInt},
		"--min-group-size":         {MCPKey: "min_group_size", Kind: FlagInt},
		// Screenshot
		"--format":          {MCPKey: "format", Kind: FlagString},
		"--quality":         {MCPKey: "quality", Kind: FlagInt},
		"--full-page":       {MCPKey: "full_page", Kind: FlagBool},
		"--selector":        {MCPKey: "selector", Kind: FlagString},
		"--wait-for-stable": {MCPKey: "wait_for_stable", Kind: FlagBool},
		"--save-to":         {MCPKey: "save_to", Kind: FlagString},
		// Storage / IndexedDB
		"--storage-type": {MCPKey: "storage_type", Kind: FlagString},
		"--key":          {MCPKey: "key", Kind: FlagString},
		"--database":     {MCPKey: "database", Kind: FlagString},
		"--store":        {MCPKey: "store", Kind: FlagString},
		// Transients / Page inventory
		"--classification": {MCPKey: "classification", Kind: FlagString},
		"--visible-only":   {MCPKey: "visible_only", Kind: FlagBool},
	})
	if err != nil {
		return nil, err
	}
	for k, v := range parsed {
		mcpArgs[k] = v
	}
	return mcpArgs, nil
}

// ParseAnalyzeArgs parses CLI flags for the analyze tool into MCP arguments.
func ParseAnalyzeArgs(what string, args []string) (map[string]any, error) {
	mcpArgs := map[string]any{"what": what}
	parsed, err := ParseFlagsBySpec(args, map[string]CLIFlagSpec{
		// Cross-cutting
		"--telemetry-mode": {MCPKey: "telemetry_mode", Kind: FlagString},
		"--background":     {MCPKey: "background", Kind: FlagBool},
		"--summary":        {MCPKey: "summary", Kind: FlagBool},
		"--limit":          {MCPKey: "limit", Kind: FlagInt},
		// Element targeting
		"--selector": {MCPKey: "selector", Kind: FlagString},
		"--frame":    {MCPKey: "frame", Kind: FlagIntOrString},
		"--tab-id":   {MCPKey: "tab_id", Kind: FlagInt},
		// Analysis control
		"--operation":           {MCPKey: "operation", Kind: FlagString},
		"--ignore-endpoints":    {MCPKey: "ignore_endpoints", Kind: FlagStringList},
		"--scope":               {MCPKey: "scope", Kind: FlagString},
		"--tags":                {MCPKey: "tags", Kind: FlagStringList},
		"--force-refresh":       {MCPKey: "force_refresh", Kind: FlagBool},
		"--domain":              {MCPKey: "domain", Kind: FlagString},
		"--timeout-ms":          {MCPKey: "timeout_ms", Kind: FlagInt},
		"--world":               {MCPKey: "world", Kind: FlagString},
		"--max-workers":         {MCPKey: "max_workers", Kind: FlagInt},
		"--checks":              {MCPKey: "checks", Kind: FlagStringList},
		"--severity-min":        {MCPKey: "severity_min", Kind: FlagString},
		"--first-party-origins": {MCPKey: "first_party_origins", Kind: FlagStringList},
		"--include-static":      {MCPKey: "include_static", Kind: FlagBool},
		"--custom-lists":        {MCPKey: "custom_lists", Kind: FlagJSON},
		"--correlation-id":      {MCPKey: "correlation_id", Kind: FlagString},
		"--annot-session":       {MCPKey: "annot_session", Kind: FlagString},
		"--urls":                {MCPKey: "urls", Kind: FlagStringList},
		"--file":                {MCPKey: "file", Kind: FlagString},
		// Annotation URL filtering
		"--url": {MCPKey: "url", Kind: FlagString},
		// Data table
		"--max-rows": {MCPKey: "max_rows", Kind: FlagInt},
		"--max-cols": {MCPKey: "max_cols", Kind: FlagInt},
		// Visual regression
		"--name":      {MCPKey: "name", Kind: FlagString},
		"--baseline":  {MCPKey: "baseline", Kind: FlagString},
		"--threshold": {MCPKey: "threshold", Kind: FlagInt},
		// Audit
		"--categories": {MCPKey: "categories", Kind: FlagStringList},
	})
	if err != nil {
		return nil, err
	}
	for k, v := range parsed {
		mcpArgs[k] = v
	}
	return mcpArgs, nil
}
