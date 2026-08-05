// Purpose: Defines hook protocol contracts, quality policy, and decision enforcement.
// Why: Keeps request decoding and the policies applied to that request under one owner.
// Docs: docs/features/feature/quality-gates/index.md

package hook

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// Agent identifies which AI coding agent is calling the hook.
type Agent string

const (
	AgentClaude Agent = "claude"
	AgentGemini Agent = "gemini"
	AgentCodex  Agent = "codex"
)

// DetectAgent identifies the calling agent from environment variables.
func DetectAgent() Agent {
	if os.Getenv("GEMINI_SESSION_ID") != "" {
		return AgentGemini
	}
	if os.Getenv("CODEX_SESSION_ID") != "" {
		return AgentCodex
	}
	return AgentClaude
}

// Input is the JSON structure Claude Code sends to PostToolUse hooks via stdin.
type Input struct {
	ToolName     string          `json:"tool_name"`
	ToolInput    json.RawMessage `json:"tool_input"`
	ToolResponse json.RawMessage `json:"tool_response"`
}

// ToolInputFields holds the commonly needed fields from tool_input.
type ToolInputFields struct {
	FilePath  string `json:"file_path"`
	Command   string `json:"command"`
	NewString string `json:"new_string"` // Edit tool: the replacement text
	Content   string `json:"content"`    // Write tool: the full file content
}

// Output is the JSON structure hooks write to stdout.
type Output struct {
	AdditionalContext string `json:"additionalContext"` // SPEC:claude-code-hooks (camelCase per protocol)
}

// ReadInput reads and parses hook input from a reader (typically os.Stdin).
func ReadInput(r io.Reader) (Input, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return Input{}, fmt.Errorf("ReadInput: cannot read stdin. %v", err)
	}
	var input Input
	if err := json.Unmarshal(data, &input); err != nil {
		return Input{}, fmt.Errorf("ReadInput: invalid JSON input. %v", err)
	}
	return input, nil
}

// ParseToolInput extracts common fields from the tool_input JSON.
func (in Input) ParseToolInput() ToolInputFields {
	var fields ToolInputFields
	if len(in.ToolInput) > 0 {
		// Best-effort parse — malformed tool_input falls back to zero-value fields,
		// causing the hook to silently do nothing (correct per hook protocol).
		_ = json.Unmarshal(in.ToolInput, &fields)
	}
	return fields
}

// ResponseText extracts the output text from tool_response.
// Handles both string responses and object responses with output/stdout/content fields.
func (in Input) ResponseText() string {
	if len(in.ToolResponse) == 0 {
		return ""
	}

	// Try as plain string first.
	var s string
	if err := json.Unmarshal(in.ToolResponse, &s); err == nil {
		return s
	}

	// Try as object with output/stdout/content fields.
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(in.ToolResponse, &obj); err != nil {
		return ""
	}

	for _, key := range []string{"output", "stdout", "content"} {
		if raw, ok := obj[key]; ok {
			var val string
			if json.Unmarshal(raw, &val) == nil {
				return val
			}
		}
	}
	return ""
}

// WriteOutput writes the hook output JSON to a writer (typically os.Stdout).
// Auto-detects the calling agent and adapts the JSON format accordingly.
// Returns nil if context is empty (nothing to output).
func WriteOutput(w io.Writer, context string) error {
	if context == "" {
		return nil
	}
	agent := DetectAgent()
	var out any
	switch agent {
	case AgentGemini:
		// SPEC:gemini-cli-hooks — nested under hookSpecificOutput.
		out = map[string]any{
			"hookSpecificOutput": map[string]string{
				"additionalContext": context,
			},
		}
	default:
		// SPEC:claude-code-hooks — flat additionalContext.
		out = Output{AdditionalContext: context}
	}
	return json.NewEncoder(w).Encode(out) //nolint: error returned to caller
}

const (
	KaboomConfigFile         = ".kaboom.json"
	DefaultCodeStandardsFile = "kaboom-code-standards.md"
	DefaultFileSizeLimit     = 800
	maxStandardsLines        = 150
)

// KaboomConfig is the structure of .kaboom.json.
type KaboomConfig struct {
	CodeStandards      string `json:"code_standards"`
	FileSizeLimit      int    `json:"file_size_limit"`
	DuplicateThreshold int    `json:"duplicate_threshold"`
}

// QualityGateResult holds the findings from the quality gate check.
type QualityGateResult struct {
	Context string
}

// FormatContext returns the additionalContext string for the hook output.
func (r *QualityGateResult) FormatContext() string {
	return r.Context
}

// RunQualityGate checks the edited/written file against project standards.
// Returns nil if no findings or if the file/config doesn't exist.
func RunQualityGate(input Input) *QualityGateResult {
	if input.ToolName != "Edit" && input.ToolName != "Write" {
		return nil
	}

	fields := input.ParseToolInput()
	filePath := fields.FilePath
	if filePath == "" {
		return nil
	}
	if _, err := os.Stat(filePath); err != nil {
		return nil
	}

	projectRoot := FindProjectRoot(filePath)
	if projectRoot == "" {
		return nil
	}

	cfg := loadKaboomConfig(filepath.Join(projectRoot, KaboomConfigFile))

	var parts []string

	// 1. Standards doc.
	standardsPath := filepath.Join(projectRoot, cfg.CodeStandards)
	if content, err := readHeadLines(standardsPath, maxStandardsLines); err == nil && content != "" {
		parts = append(parts,
			"=== PROJECT CODE STANDARDS ===",
			content,
			"=== END STANDARDS ===",
		)
	}

	// 2. File size check.
	if lineCount, err := countLines(filePath); err == nil {
		if lineCount > cfg.FileSizeLimit {
			parts = append(parts, fmt.Sprintf(
				"WARNING: %s is %d lines (limit: %d). This file must be split.",
				filePath, lineCount, cfg.FileSizeLimit))
		} else if lineCount > cfg.FileSizeLimit*90/100 {
			parts = append(parts, fmt.Sprintf(
				"NOTE: %s is %d lines (limit: %d). Approaching the limit — consider splitting.",
				filePath, lineCount, cfg.FileSizeLimit))
		}
	}

	// 3. Convention summary — always inject top discovered conventions so the
	//    LLM can judge drift even when the edit doesn't contain a matching pattern.
	ext := filepath.Ext(filePath)
	if summary := ConventionSummary(projectRoot, ext); summary != "" {
		parts = append(parts, summary)
	}

	// 4. Convention detection — reuse already-parsed fields to avoid double-unmarshal.
	//    If the edit contains a known pattern, show specific examples from the codebase.
	newContent := extractNewContent(input, fields)
	if conventions := DetectConventions(filePath, projectRoot, newContent); len(conventions) > 0 {
		parts = append(parts, FormatConventions(conventions))
	}

	// 5. Review instruction.
	if len(parts) > 0 {
		parts = append(parts,
			"QUALITY GATE: Review your change against the standards and conventions above. Fix any violations before proceeding.")
	}

	if len(parts) == 0 {
		return nil
	}

	return &QualityGateResult{
		Context: strings.Join(parts, "\n"),
	}
}

// FindProjectRoot walks up from filePath looking for .kaboom.json.
func FindProjectRoot(filePath string) string {
	dir := filepath.Dir(filePath)
	for {
		if _, err := os.Stat(filepath.Join(dir, KaboomConfigFile)); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return ""
}

// loadKaboomConfig reads and parses .kaboom.json with defaults.
func loadKaboomConfig(path string) KaboomConfig {
	cfg := KaboomConfig{
		CodeStandards: DefaultCodeStandardsFile,
		FileSizeLimit: DefaultFileSizeLimit,
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg
	}
	// Best-effort parse — malformed config falls back to defaults above.
	_ = json.Unmarshal(data, &cfg)
	if cfg.CodeStandards == "" {
		cfg.CodeStandards = DefaultCodeStandardsFile
	}
	if cfg.FileSizeLimit <= 0 {
		cfg.FileSizeLimit = DefaultFileSizeLimit
	}
	return cfg
}

// readHeadLines reads up to maxLines from a file.
func readHeadLines(path string, maxLines int) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	lines := strings.Split(string(data), "\n")
	if len(lines) > maxLines {
		lines = lines[:maxLines]
	}
	return strings.Join(lines, "\n"), nil
}

// countLines counts newlines in a file.
func countLines(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	if len(data) == 0 {
		return 0, nil
	}
	count := strings.Count(string(data), "\n")
	// If the file doesn't end with a newline, add 1.
	if data[len(data)-1] != '\n' {
		count++
	}
	return count, nil
}

// extractNewContent returns the newly introduced code from the hook input.
// For Edit: returns new_string (only the changed code).
// For Write: returns content (entire file is new).
// Falls back to reading the file from disk for Write if content is empty.
// Accepts pre-parsed fields to avoid re-parsing tool_input JSON.
func extractNewContent(input Input, fields ToolInputFields) string {
	if fields.NewString != "" {
		return fields.NewString
	}
	if fields.Content != "" {
		return fields.Content
	}
	// Fallback for Write: file was already written to disk.
	if input.ToolName == "Write" && fields.FilePath != "" {
		data, err := os.ReadFile(fields.FilePath)
		if err == nil {
			return string(data)
		}
	}
	return ""
}

const (
	decisionsDir  = ".kaboom"
	decisionsFile = "decisions.json"
)

// Decision represents a locked architectural decision.
type Decision struct {
	ID       string `json:"id"`
	Rule     string `json:"rule"`
	Pattern  string `json:"pattern"` // Literal substring match.
	Regex    string `json:"regex"`   // Regex match (prefix with re: in pattern field, or use this field).
	Reason   string `json:"reason"`
	Enforced string `json:"enforced"` // Date when decision was made.
	Expires  string `json:"expires"`  // Optional expiry date (YYYY-MM-DD).
}

// DecisionGuardResult holds the findings from decision guard analysis.
type DecisionGuardResult struct {
	Context   string
	Decisions []Decision
}

// FormatContext returns the additionalContext string for the hook output.
func (r *DecisionGuardResult) FormatContext() string {
	return r.Context
}

// RunDecisionGuard checks edited code against project decisions.
// Returns nil if no decisions match or no decision file exists.
func RunDecisionGuard(input Input, projectRoot string) *DecisionGuardResult {
	if !isEditTool(input.ToolName) {
		return nil
	}

	fields := input.ParseToolInput()
	filePath := fields.FilePath
	if filePath == "" || projectRoot == "" {
		return nil
	}

	newContent := extractNewContent(input, fields)
	if newContent == "" {
		return nil
	}

	decisions := loadDecisions(projectRoot)
	if len(decisions) == 0 {
		return nil
	}

	var matched []Decision
	for _, d := range decisions {
		if isExpired(d) {
			continue
		}
		if matchesDecision(d, newContent, filePath) {
			matched = append(matched, d)
		}
	}

	if len(matched) == 0 {
		return nil
	}

	return &DecisionGuardResult{
		Context:   formatDecisions(matched),
		Decisions: matched,
	}
}

func loadDecisions(projectRoot string) []Decision {
	path := filepath.Join(projectRoot, decisionsDir, decisionsFile)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	var decisions []Decision
	if json.Unmarshal(data, &decisions) != nil {
		return nil
	}
	return decisions
}

func isExpired(d Decision) bool {
	if d.Expires == "" {
		return false
	}
	expiry, err := time.Parse("2006-01-02", d.Expires)
	if err != nil {
		return false
	}
	return time.Now().After(expiry)
}

func matchesDecision(d Decision, content, filePath string) bool {
	// Check regex pattern.
	if d.Regex != "" {
		re, err := regexp.Compile(d.Regex)
		if err != nil {
			return false // Skip invalid regex.
		}
		if re.MatchString(content) {
			return true
		}
	}

	// Check pattern field.
	if d.Pattern != "" {
		// Support "re:" prefix for inline regex.
		if strings.HasPrefix(d.Pattern, "re:") {
			reStr := strings.TrimPrefix(d.Pattern, "re:")
			re, err := regexp.Compile(reStr)
			if err != nil {
				return false
			}
			return re.MatchString(content)
		}
		// Literal substring match.
		return strings.Contains(content, d.Pattern)
	}

	return false
}

func formatDecisions(decisions []Decision) string {
	var b strings.Builder
	b.WriteString("=== ARCHITECTURAL DECISIONS (do not violate) ===")
	for _, d := range decisions {
		fmt.Fprintf(&b, "\n[%s] %s", d.ID, d.Rule)
		if d.Reason != "" {
			fmt.Fprintf(&b, "\n  Reason: %s", d.Reason)
		}
		if d.Enforced != "" {
			fmt.Fprintf(&b, "\n  Enforced: %s", d.Enforced)
		}
	}
	b.WriteString("\n=== END DECISIONS ===")
	b.WriteString("\nDECISION GUARD: Your edit matches a locked architectural decision. Revise to comply.")
	return b.String()
}
