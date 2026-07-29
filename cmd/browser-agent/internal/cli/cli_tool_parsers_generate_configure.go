// cli_tool_parsers_generate_configure.go — Tool-specific CLI flag-to-MCP argument mapping for generate/configure.
// Why: Keeps generate/configure parser contracts isolated from observe/analyze/interact parsers.
// Docs: docs/features/feature/enhanced-cli-config/index.md

package cli

// ParseGenerateArgs parses CLI flags for the generate tool into MCP arguments.
func ParseGenerateArgs(format string, args []string) (map[string]any, error) {
	mcpArgs := map[string]any{"what": format}
	parsed, err := ParseFlagsBySpec(args, map[string]CLIFlagSpec{
		"--telemetry-mode":        {MCPKey: "telemetry_mode", Kind: FlagString},
		"--error-message":         {MCPKey: "error_message", Kind: FlagString},
		"--last-n":                {MCPKey: "last_n", Kind: FlagInt},
		"--base-url":              {MCPKey: "base_url", Kind: FlagString},
		"--include-screenshots":   {MCPKey: "include_screenshots", Kind: FlagBool},
		"--generate-fixtures":     {MCPKey: "generate_fixtures", Kind: FlagBool},
		"--visual-assertions":     {MCPKey: "visual_assertions", Kind: FlagBool},
		"--test-name":             {MCPKey: "test_name", Kind: FlagString},
		"--assert-network":        {MCPKey: "assert_network", Kind: FlagBool},
		"--assert-no-errors":      {MCPKey: "assert_no_errors", Kind: FlagBool},
		"--assert-response-shape": {MCPKey: "assert_response_shape", Kind: FlagBool},
		"--scope":                 {MCPKey: "scope", Kind: FlagString},
		"--include-passes":        {MCPKey: "include_passes", Kind: FlagBool},
		"--save-to":               {MCPKey: "save_to", Kind: FlagString},
		"--url":                   {MCPKey: "url", Kind: FlagString},
		"--method":                {MCPKey: "method", Kind: FlagString},
		"--status-min":            {MCPKey: "status_min", Kind: FlagInt},
		"--status-max":            {MCPKey: "status_max", Kind: FlagInt},
		"--mode":                  {MCPKey: "mode", Kind: FlagString},
		"--include-report-uri":    {MCPKey: "include_report_uri", Kind: FlagBool},
		"--exclude-origins":       {MCPKey: "exclude_origins", Kind: FlagStringList},
		"--resource-types":        {MCPKey: "resource_types", Kind: FlagStringList},
		"--origins":               {MCPKey: "origins", Kind: FlagStringList},
		"--annot-session":         {MCPKey: "annot_session", Kind: FlagString},
		"--context":               {MCPKey: "context", Kind: FlagString},
		"--action":                {MCPKey: "action", Kind: FlagString},
		"--test-file":             {MCPKey: "test_file", Kind: FlagString},
		"--test-dir":              {MCPKey: "test_dir", Kind: FlagString},
		"--broken-selectors":      {MCPKey: "broken_selectors", Kind: FlagStringList},
		"--auto-apply":            {MCPKey: "auto_apply", Kind: FlagBool},
		"--failure":               {MCPKey: "failure", Kind: FlagJSON},
		"--failures":              {MCPKey: "failures", Kind: FlagJSON},
		"--error-id":              {MCPKey: "error_id", Kind: FlagString},
		"--include-mocks":         {MCPKey: "include_mocks", Kind: FlagBool},
		"--output-format":         {MCPKey: "output_format", Kind: FlagString},
	})
	if err != nil {
		return nil, err
	}
	for k, v := range parsed {
		mcpArgs[k] = v
	}
	return mcpArgs, nil
}

// ParseConfigureArgs parses CLI flags for the configure tool into MCP arguments.
func ParseConfigureArgs(action string, args []string) (map[string]any, error) {
	mcpArgs := map[string]any{"what": action}
	parsed, err := ParseFlagsBySpec(args, map[string]CLIFlagSpec{
		// Cross-cutting
		"--telemetry-mode": {MCPKey: "telemetry_mode", Kind: FlagString},
		"--mode":           {MCPKey: "mode", Kind: FlagString},
		"--tool":           {MCPKey: "tool", Kind: FlagString},
		"--confirm":        {MCPKey: "confirm", Kind: FlagBool},
		// Store / persistence
		"--store-action": {MCPKey: "store_action", Kind: FlagString},
		"--namespace":    {MCPKey: "namespace", Kind: FlagString},
		"--key":          {MCPKey: "key", Kind: FlagString},
		"--data":         {MCPKey: "data", Kind: FlagJSONOrString},
		"--value":        {MCPKey: "value", Kind: FlagJSONOrString},
		// Noise filtering
		"--noise-action":   {MCPKey: "noise_action", Kind: FlagString},
		"--rules":          {MCPKey: "rules", Kind: FlagJSON},
		"--rule-id":        {MCPKey: "rule_id", Kind: FlagString},
		"--pattern":        {MCPKey: "pattern", Kind: FlagString},
		"--category":       {MCPKey: "category", Kind: FlagString},
		"--reason":         {MCPKey: "reason", Kind: FlagString},
		"--classification": {MCPKey: "classification", Kind: FlagString},
		"--message-regex":  {MCPKey: "message_regex", Kind: FlagString},
		"--source-regex":   {MCPKey: "source_regex", Kind: FlagString},
		"--url-regex":      {MCPKey: "url_regex", Kind: FlagString},
		"--method":         {MCPKey: "method", Kind: FlagString},
		"--domain":         {MCPKey: "domain", Kind: FlagString},
		"--status-min":     {MCPKey: "status_min", Kind: FlagInt},
		"--status-max":     {MCPKey: "status_max", Kind: FlagInt},
		"--level":          {MCPKey: "level", Kind: FlagString},
		// Recording / playback
		"--buffer":                 {MCPKey: "buffer", Kind: FlagString},
		"--tab-id":                 {MCPKey: "tab_id", Kind: FlagInt},
		"--recording-id":           {MCPKey: "recording_id", Kind: FlagString},
		"--sensitive-data-enabled": {MCPKey: "sensitive_data_enabled", Kind: FlagBool},
		// Audit / diagnostics
		"--audit-session-id": {MCPKey: "audit_session_id", Kind: FlagString},
		"--tool-name":        {MCPKey: "tool_name", Kind: FlagString},
		"--since":            {MCPKey: "since", Kind: FlagString},
		"--limit":            {MCPKey: "limit", Kind: FlagInt},
		"--operation":        {MCPKey: "operation", Kind: FlagString},
		// Report issue
		"--template":     {MCPKey: "template", Kind: FlagString},
		"--title":        {MCPKey: "title", Kind: FlagString},
		"--user-context": {MCPKey: "user_context", Kind: FlagString},
		// Streaming
		"--streaming-action": {MCPKey: "streaming_action", Kind: FlagString},
		"--events":           {MCPKey: "events", Kind: FlagStringList},
		"--throttle-seconds": {MCPKey: "throttle_seconds", Kind: FlagInt},
		// Action jitter
		"--action-jitter-ms": {MCPKey: "action_jitter_ms", Kind: FlagInt},
		// Diff sessions / verification
		"--verif-session-action": {MCPKey: "verif_session_action", Kind: FlagString},
		"--name":                 {MCPKey: "name", Kind: FlagString},
		"--compare-a":            {MCPKey: "compare_a", Kind: FlagString},
		"--compare-b":            {MCPKey: "compare_b", Kind: FlagString},
		"--url":                  {MCPKey: "url", Kind: FlagString},
		// Testing
		"--severity-min": {MCPKey: "severity_min", Kind: FlagString},
		"--test-id":      {MCPKey: "test_id", Kind: FlagString},
		"--label":        {MCPKey: "label", Kind: FlagString},
		"--original-id":  {MCPKey: "original_id", Kind: FlagString},
		"--replay-id":    {MCPKey: "replay_id", Kind: FlagString},
		// Sequences
		"--steps":             {MCPKey: "steps", Kind: FlagJSON},
		"--tags":              {MCPKey: "tags", Kind: FlagStringList},
		"--override-steps":    {MCPKey: "override_steps", Kind: FlagJSON},
		"--step-timeout-ms":   {MCPKey: "step_timeout_ms", Kind: FlagInt},
		"--continue-on-error": {MCPKey: "continue_on_error", Kind: FlagBool},
		"--stop-after-step":   {MCPKey: "stop_after_step", Kind: FlagInt},
		"--description":       {MCPKey: "description", Kind: FlagString},
		// Quality gates
		"--target-dir": {MCPKey: "target_dir", Kind: FlagString},
		// Network recording
		"--network-action": {MCPKey: "network_action", Kind: FlagString},
	})
	if err != nil {
		return nil, err
	}
	for k, v := range parsed {
		mcpArgs[k] = v
	}
	return mcpArgs, nil
}
