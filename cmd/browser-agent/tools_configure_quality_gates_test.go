// tools_configure_quality_gates_test.go — Tests for configure(what="setup_quality_gates") handler.

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSetupQualityGates_CreatesConfigAndStandards(t *testing.T) {
	t.Parallel()
	h, server, _ := makeToolHandler(t)

	dir := t.TempDir()
	server.activeCodebase.SetActiveCodebase(dir)

	resp := callConfigureRaw(h, `{"what":"setup_quality_gates"}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("expected success, got error: %s", firstText(result))
	}

	data := extractResultJSON(t, result)

	// Verify .kaboom.json was created.
	configPath := filepath.Join(dir, ".kaboom.json")
	if _, err := os.Stat(configPath); err != nil {
		t.Fatalf(".kaboom.json not created: %v", err)
	}

	// Verify kaboom-code-standards.md was created.
	standardsPath := filepath.Join(dir, "kaboom-code-standards.md")
	if _, err := os.Stat(standardsPath); err != nil {
		t.Fatalf("kaboom-code-standards.md not created: %v", err)
	}

	// Verify response contains created paths.
	if data["config_path"] == nil {
		t.Fatal("response missing config_path")
	}
	if data["standards_path"] == nil {
		t.Fatal("response missing standards_path")
	}

	// Verify config content is valid JSON with expected defaults.
	configBytes, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("failed to read .kaboom.json: %v", err)
	}
	var config map[string]any
	if err := json.Unmarshal(configBytes, &config); err != nil {
		t.Fatalf(".kaboom.json is not valid JSON: %v", err)
	}
	if config["code_standards"] != "kaboom-code-standards.md" {
		t.Fatalf("code_standards = %v, want kaboom-code-standards.md", config["code_standards"])
	}

	// Verify standards file has content.
	standardsBytes, err := os.ReadFile(standardsPath)
	if err != nil {
		t.Fatalf("failed to read kaboom-code-standards.md: %v", err)
	}
	if len(standardsBytes) < 100 {
		t.Fatal("kaboom-code-standards.md is too short — missing starter content")
	}
}

func TestSetupQualityGates_DoesNotOverwriteExistingConfig(t *testing.T) {
	t.Parallel()
	h, server, _ := makeToolHandler(t)

	dir := t.TempDir()
	server.activeCodebase.SetActiveCodebase(dir)

	// Pre-create .kaboom.json with custom content.
	existing := `{"code_standards":"my-custom-rules.md","file_size_limit":500}`
	if err := os.WriteFile(filepath.Join(dir, ".kaboom.json"), []byte(existing), 0644); err != nil {
		t.Fatal(err)
	}

	resp := callConfigureRaw(h, `{"what":"setup_quality_gates"}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("expected success, got error: %s", firstText(result))
	}

	// Verify existing config was NOT overwritten.
	configBytes, _ := os.ReadFile(filepath.Join(dir, ".kaboom.json"))
	if string(configBytes) != existing {
		t.Fatalf(".kaboom.json was overwritten: got %q", string(configBytes))
	}

	data := extractResultJSON(t, result)
	if data["config_existed"] != true {
		t.Fatal("response should indicate config_existed=true")
	}
}

func TestSetupQualityGates_DoesNotOverwriteExistingStandards(t *testing.T) {
	t.Parallel()
	h, server, _ := makeToolHandler(t)

	dir := t.TempDir()
	server.activeCodebase.SetActiveCodebase(dir)

	// Pre-create standards file.
	existing := "# My Custom Standards\n\nDo not overwrite me."
	if err := os.WriteFile(filepath.Join(dir, "kaboom-code-standards.md"), []byte(existing), 0644); err != nil {
		t.Fatal(err)
	}

	resp := callConfigureRaw(h, `{"what":"setup_quality_gates"}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("expected success, got error: %s", firstText(result))
	}

	// Verify standards file was NOT overwritten.
	standardsBytes, _ := os.ReadFile(filepath.Join(dir, "kaboom-code-standards.md"))
	if string(standardsBytes) != existing {
		t.Fatalf("kaboom-code-standards.md was overwritten")
	}
}

func TestSetupQualityGates_CustomTargetDir(t *testing.T) {
	t.Parallel()
	h, server, _ := makeToolHandler(t)

	dir := t.TempDir()
	server.activeCodebase.SetActiveCodebase(dir)

	subdir := filepath.Join(dir, "subproject")
	if err := os.MkdirAll(subdir, 0755); err != nil {
		t.Fatal(err)
	}

	resp := callConfigureRaw(h, `{"what":"setup_quality_gates","target_dir":"`+subdir+`"}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("expected success, got error: %s", firstText(result))
	}

	// Verify files created in subdir.
	if _, err := os.Stat(filepath.Join(subdir, ".kaboom.json")); err != nil {
		t.Fatal(".kaboom.json not created in target_dir")
	}
}

func TestSetupQualityGates_RejectsTargetDirOutsideProject(t *testing.T) {
	t.Parallel()
	h, server, _ := makeToolHandler(t)

	dir := t.TempDir()
	server.activeCodebase.SetActiveCodebase(dir)

	outsideDir := t.TempDir() // Different temp dir — outside project.

	resp := callConfigureRaw(h, `{"what":"setup_quality_gates","target_dir":"`+outsideDir+`"}`)
	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("expected error for target_dir outside project")
	}
	text := firstText(result)
	if !strings.Contains(text, "outside") {
		t.Fatalf("error should mention 'outside', got: %s", text)
	}
}

func TestSetupQualityGates_RequiresActiveCodebase(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	// No active codebase set.
	resp := callConfigureRaw(h, `{"what":"setup_quality_gates"}`)
	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("expected error when no active codebase is set")
	}
	text := firstText(result)
	if !strings.Contains(text, "codebase") || !strings.Contains(text, "active") {
		t.Fatalf("error should mention active codebase, got: %s", text)
	}
}

func TestSetupQualityGates_CustomStandardsPathInExistingConfig(t *testing.T) {
	t.Parallel()
	h, server, _ := makeToolHandler(t)

	dir := t.TempDir()
	server.activeCodebase.SetActiveCodebase(dir)

	// Pre-create config pointing to custom standards file.
	config := `{"code_standards":"docs/my-patterns.md"}`
	if err := os.WriteFile(filepath.Join(dir, ".kaboom.json"), []byte(config), 0644); err != nil {
		t.Fatal(err)
	}

	resp := callConfigureRaw(h, `{"what":"setup_quality_gates"}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("expected success, got error: %s", firstText(result))
	}

	// Should NOT create kaboom-code-standards.md because config points elsewhere.
	if _, err := os.Stat(filepath.Join(dir, "kaboom-code-standards.md")); err == nil {
		t.Fatal("should not create default standards file when config points to a custom path")
	}
}

func TestSetupQualityGates_SnakeCaseResponse(t *testing.T) {
	t.Parallel()
	h, server, _ := makeToolHandler(t)

	dir := t.TempDir()
	server.activeCodebase.SetActiveCodebase(dir)

	resp := callConfigureRaw(h, `{"what":"setup_quality_gates"}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("expected success, got error: %s", firstText(result))
	}

	jsonPart := extractJSONFromText(firstText(result))
	assertSnakeCaseFields(t, jsonPart)
}

func TestSetupQualityGates_ResponseContainsSuggestions(t *testing.T) {
	t.Parallel()
	h, server, _ := makeToolHandler(t)

	dir := t.TempDir()
	server.activeCodebase.SetActiveCodebase(dir)

	resp := callConfigureRaw(h, `{"what":"setup_quality_gates"}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("expected success, got error: %s", firstText(result))
	}

	data := extractResultJSON(t, result)
	suggestions, ok := data["suggestions"].([]any)
	if !ok || len(suggestions) == 0 {
		t.Fatal("response should include suggestions array")
	}
}

func TestSetupQualityGates_ResponseContainsDefaults(t *testing.T) {
	t.Parallel()
	h, server, _ := makeToolHandler(t)

	dir := t.TempDir()
	server.activeCodebase.SetActiveCodebase(dir)

	resp := callConfigureRaw(h, `{"what":"setup_quality_gates"}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("expected success, got error: %s", firstText(result))
	}

	data := extractResultJSON(t, result)
	defaults, ok := data["defaults"].(map[string]any)
	if !ok {
		t.Fatal("response should include defaults object")
	}
	if defaults["file_size_limit"] == nil {
		t.Fatal("defaults should include file_size_limit")
	}
	if defaults["code_standards"] == nil {
		t.Fatal("defaults should include code_standards")
	}
}

func TestSetupQualityGates_InstallsHooks(t *testing.T) {
	t.Parallel()
	h, server, _ := makeToolHandler(t)

	dir := t.TempDir()
	server.activeCodebase.SetActiveCodebase(dir)

	resp := callConfigureRaw(h, `{"what":"setup_quality_gates"}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("expected success, got error: %s", firstText(result))
	}

	data := extractResultJSON(t, result)
	if data["hooks_installed"] != true {
		t.Fatal("hooks_installed should be true on first run")
	}
	if data["settings_path"] == nil {
		t.Fatal("response should include settings_path")
	}

	// Verify .claude/settings.json was created with hooks.
	settingsPath := filepath.Join(dir, ".claude", "settings.json")
	settingsBytes, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf(".claude/settings.json not created: %v", err)
	}

	var settings map[string]any
	if err := json.Unmarshal(settingsBytes, &settings); err != nil {
		t.Fatalf("settings.json is not valid JSON: %v", err)
	}

	hooks, _ := settings["hooks"].(map[string]any)
	if hooks == nil {
		t.Fatal("settings.json missing hooks key")
	}
	postToolUse, _ := hooks["PostToolUse"].([]any)
	if len(postToolUse) != 3 {
		t.Fatalf("expected 3 PostToolUse entries (Edit|Write, Read, Bash), got %d", len(postToolUse))
	}

	// Verify all hook commands are present.
	settingsStr := string(settingsBytes)
	for _, cmd := range []string{
		"kaboom-hooks quality-gate",
		"kaboom-hooks compress-output",
		"kaboom-hooks session-track",
		"kaboom-hooks blast-radius",
		"kaboom-hooks decision-guard",
	} {
		if !strings.Contains(settingsStr, cmd) {
			t.Errorf("settings.json missing %s hook command", cmd)
		}
	}
}

func TestSetupQualityGates_DoesNotDuplicateHooks(t *testing.T) {
	t.Parallel()
	h, server, _ := makeToolHandler(t)

	dir := t.TempDir()
	server.activeCodebase.SetActiveCodebase(dir)

	// Run setup twice.
	callConfigureRaw(h, `{"what":"setup_quality_gates"}`)
	resp := callConfigureRaw(h, `{"what":"setup_quality_gates"}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("expected success, got error: %s", firstText(result))
	}

	data := extractResultJSON(t, result)
	if data["hooks_installed"] != false {
		t.Fatal("hooks_installed should be false on second run")
	}

	// Verify only 3 PostToolUse entries (not 6).
	settingsBytes, _ := os.ReadFile(filepath.Join(dir, ".claude", "settings.json"))
	var settings map[string]any
	if err := json.Unmarshal(settingsBytes, &settings); err != nil {
		t.Fatalf("invalid settings JSON: %v", err)
	}
	hooks, _ := settings["hooks"].(map[string]any)
	postToolUse, _ := hooks["PostToolUse"].([]any)
	if len(postToolUse) != 3 {
		t.Fatalf("expected 3 PostToolUse entries after double-run, got %d", len(postToolUse))
	}
}

func TestSetupQualityGates_MergesWithExistingSettings(t *testing.T) {
	t.Parallel()
	h, server, _ := makeToolHandler(t)

	dir := t.TempDir()
	server.activeCodebase.SetActiveCodebase(dir)

	// Pre-create .claude/settings.json with existing settings.
	settingsDir := filepath.Join(dir, ".claude")
	os.MkdirAll(settingsDir, 0755)
	existing := `{"permissions":{"allow":["Read","Write"]},"model":"sonnet"}`
	os.WriteFile(filepath.Join(settingsDir, "settings.json"), []byte(existing), 0644)

	resp := callConfigureRaw(h, `{"what":"setup_quality_gates"}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("expected success, got error: %s", firstText(result))
	}

	// Verify existing settings were preserved.
	settingsBytes, _ := os.ReadFile(filepath.Join(settingsDir, "settings.json"))
	var settings map[string]any
	if err := json.Unmarshal(settingsBytes, &settings); err != nil {
		t.Fatalf("invalid settings JSON: %v", err)
	}

	if settings["model"] != "sonnet" {
		t.Fatal("existing model setting was lost")
	}
	permissions, _ := settings["permissions"].(map[string]any)
	if permissions == nil {
		t.Fatal("existing permissions were lost")
	}

	// Verify hooks were added.
	hooks, _ := settings["hooks"].(map[string]any)
	if hooks == nil {
		t.Fatal("hooks were not added")
	}
}
