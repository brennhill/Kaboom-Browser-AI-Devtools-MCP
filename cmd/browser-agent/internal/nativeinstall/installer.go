// Purpose: Auto-detects and configures MCP client integrations (Claude Code, Cursor, Windsurf, etc.) during --install.
// Why: Provides zero-config onboarding by writing the correct JSON config for each supported MCP client.

package nativeinstall

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/diag"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/identity"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/telemetry"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/util"
)

// installerLegacyServerKeys are historical MCP config IDs that are migrated
// to the canonical identity.MCPServerName during install.
var installerLegacyServerKeys = []string{
	"kaboom-agentic-browser",
	"kaboom",
	"gasoline-browser-devtools",
	"gasoline-agentic-browser",
	"gasoline",
	"strum-browser-devtools",
	"strum-agentic-browser",
	"strum",
}

func extensionInstallDir(home string) string {
	if override := strings.TrimSpace(os.Getenv("KABOOM_EXTENSION_DIR")); override != "" {
		return override
	}
	return filepath.Join(home, "KaboomAgenticDevtoolExtension")
}

func manualExtensionSetupChecklist(extDir string) []string {
	return []string{
		"BROWSER EXTENSION (MANUAL STEP REQUIRED):",
		"   The installer staged extension files, but it cannot click browser UI controls for you.",
		"   1) Open chrome://extensions (or brave://extensions)",
		"   2) Enable Developer mode",
		"   3) Click Load unpacked and select:",
		fmt.Sprintf("      %s", extDir),
		"   4) Pin Kaboom in the browser toolbar (recommended)",
		"   5) Open the Kaboom popup and click Track This Tab",
	}
}

func printManualExtensionSetupChecklist(extDir string) {
	lines := manualExtensionSetupChecklist(extDir)
	if len(lines) == 0 {
		return
	}
	diag.Printf("\033[1;33m%s\033[0m\n", lines[0])
	for _, line := range lines[1:] {
		diag.Printf("%s\n", line)
	}
}

// fileManagerOpenCommand returns the command that reveals dir in goos's desktop
// file manager, or ok=false when the platform has none. Pure so the mapping can
// be unit-tested without launching anything.
func fileManagerOpenCommand(goos, dir string) (name string, args []string, ok bool) {
	switch goos {
	case "darwin":
		return "open", []string{dir}, true
	case "windows":
		return "explorer", []string{dir}, true
	case "linux":
		return "xdg-open", []string{dir}, true
	default:
		return "", nil, false
	}
}

// envFlagEnabled reports whether any of the named env vars is set to a truthy
// opt-in value. Unset/empty/"0"/"false"/"no" all count as off. Shared source of
// truth for the install-time opt-outs so their accepted values never drift.
func envFlagEnabled(keys ...string) bool {
	for _, key := range keys {
		v := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
		if v != "" && v != "0" && v != "false" && v != "no" {
			return true
		}
	}
	return false
}

// extensionAutoOpenDisabled reports whether the user opted out of the
// install-time folder auto-open (headless installs, CI, scripted runs).
func extensionAutoOpenDisabled() bool {
	return envFlagEnabled("KABOOM_NO_OPEN", "KABOOM_INSTALL_NO_OPEN")
}

// openExtensionFolder best-effort reveals extDir in the file manager so Load
// unpacked is one selection away. Never fatal — the checklist still shows the
// path when this cannot run (no opener, headless, opted out, dir absent).
func openExtensionFolder(extDir string) bool {
	if extDir == "" || extensionAutoOpenDisabled() {
		return false
	}
	if info, err := os.Stat(extDir); err != nil || !info.IsDir() {
		return false
	}
	name, args, ok := fileManagerOpenCommand(runtime.GOOS, extDir)
	if !ok {
		return false
	}
	if _, err := exec.LookPath(name); err != nil {
		return false
	}
	cmd := exec.Command(name, args...)
	cmd.Stdout, cmd.Stderr, cmd.Stdin = nil, nil, nil
	util.SetDetachedProcess(cmd)
	if err := cmd.Start(); err != nil {
		return false
	}
	go func() { _ = cmd.Wait() }() // lint:allow-bare-goroutine — reap the detached opener
	return true
}

func printInstallerPanel(title string, lines []string) {
	const border = "+----------------------------------------------------------+"
	diag.Printf("\033[1;36m%s\033[0m\n", border)
	diag.Printf("\033[1;36m| \033[1m%-56s\033[1;36m |\033[0m\n", title)
	diag.Printf("\033[1;36m%s\033[0m\n", border)
	for _, line := range lines {
		diag.Printf("\033[1;36m|\033[0m %-58s \033[1;36m|\033[0m\n", line)
	}
	diag.Printf("\033[1;36m%s\033[0m\n", border)
}

// mcpFileConfig describes one file-based MCP client config target.
type mcpFileConfig struct {
	name     string
	path     string
	key      string
	isCustom bool
}

// fileConfigTargets returns the file-based MCP client config targets for the
// current OS. VS Code reads MCP servers from a top-level "servers" key in
// mcp.json (not "mcpServers" like Claude Desktop/Cursor).
func fileConfigTargets(home string) []mcpFileConfig {
	configs := []mcpFileConfig{
		{"Cursor", "~/.cursor/mcp.json", "mcpServers", false},
		{"Windsurf", "~/.codeium/windsurf/mcp_config.json", "mcpServers", false},
		{"Gemini CLI", "~/.gemini/settings.json", "mcpServers", false},
		{"Antigravity", "~/.gemini/antigravity/mcp_config.json", "mcpServers", false},
		{"OpenCode", "~/.config/opencode/opencode.json", "mcp", true},
		{"Zed", "~/.config/zed/settings.json", "context_servers", true},
	}

	switch runtime.GOOS {
	case "darwin":
		configs = append(configs,
			mcpFileConfig{"Claude Desktop", "Library/Application Support/Claude/claude_desktop_config.json", "mcpServers", false},
			mcpFileConfig{"VS Code", "Library/Application Support/Code/User/mcp.json", "servers", false},
		)
	case "linux":
		configs = append(configs,
			mcpFileConfig{"VS Code", ".config/Code/User/mcp.json", "servers", false},
		)
	case "windows":
		appData := os.Getenv("APPDATA")
		if appData == "" {
			appData = filepath.Join(home, "AppData", "Roaming")
		}
		configs = append(configs,
			mcpFileConfig{"Claude Desktop", filepath.Join(appData, "Claude", "claude_desktop_config.json"), "mcpServers", false},
			mcpFileConfig{"VS Code", filepath.Join(appData, "Code", "User", "mcp.json"), "servers", false},
		)
	}
	return configs
}

// Run detects and configures all supported MCP clients.
func Run(forceCleanup func() error) {
	// 1. Silent Reset (Kill stale instances)
	// We do this first to ensure config files aren't being held open
	// and no old versions are interfering.
	if forceCleanup != nil {
		_ = forceCleanup()
	}

	exe, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Error: Could not determine kaboom binary path: %v\n", err)
		os.Exit(1)
	}

	home, homeErr := os.UserHomeDir()
	if homeErr != nil {
		home = ""
	}
	extDir := extensionInstallDir(home)

	// 2. Claude Code
	if err := installClaudeCode(exe); err != nil {
		diag.Printf("  ⚠️  Claude Code: %v\n", err)
	}

	// 3. File-based configs
	clientsConfigured := 0
	if home == "" {
		// Without a home directory every relative target would resolve to
		// cwd-relative junk paths, so skip the file-based configs entirely.
		diag.Printf("  ⚠️  Could not determine home directory (%v); skipping file-based MCP client configs\n", homeErr)
	} else {
		for _, cfg := range fileConfigTargets(home) {
			path := cfg.path
			if strings.HasPrefix(path, "~/") {
				path = filepath.Join(home, path[2:])
			} else if !filepath.IsAbs(path) {
				path = filepath.Join(home, path)
			}

			if _, err := os.Stat(filepath.Dir(path)); os.IsNotExist(err) {
				continue // Client directory doesn't exist, skip
			}

			if err := mergeJSONConfig(path, cfg.key, exe, cfg.isCustom); err != nil {
				telemetry.AppError("install_config_error", nil)
				diag.Printf("  ⚠️  %s: %v\n", cfg.name, err)
			} else {
				clientsConfigured++
			}
		}
	}

	// NOTE: install_complete lifecycle event not yet in Counterscale contract.
	// Add to LIFECYCLE_EVENTS in counterscale ingest before re-enabling.

	// 4. Start the Daemon
	// We start the daemon so the extension works immediately and the user
	// can verify the install with a health check.
	diag.Printf("🚀 Starting Kaboom server...")
	startDaemonSilently(exe)

	// 5. BIG SUCCESS MESSAGE
	diag.Printf("\n\033[1;32m✅ KABOOM INSTALLED & RUNNING!\033[0m\n")
	printInstallerPanel("INSTALL SUMMARY", []string{
		"Kaboom server started in background on port 7890.",
		"MCP clients are configured with direct binary path (no npx).",
		fmt.Sprintf("Binary path: %s", exe),
	})
	diag.Printf("\n")
	printManualExtensionSetupChecklist(extDir)
	if openExtensionFolder(extDir) {
		diag.Printf("   \033[1;32m📂 Opened the extension folder for you — select it in Load unpacked.\033[0m\n")
	}
	// Confirm the extension actually connects to the daemon we just started.
	runExtensionConnectWait(7890, extDir)
	diag.Printf("\033[1;33mREADY TO COOK:\033[0m\n")
	diag.Printf("   The Kaboom server is active on port 7890.\n")
	diag.Printf("   Your AI tool (Claude, Cursor, etc.) is now configured.\n")
	diag.Printf("\033[1;36m+----------------------------------------------------------+\033[0m\n")
}

func startDaemonSilently(exe string) {
	// Standard daemon flags
	args := []string{"--daemon", "--port", "7890"}
	cmd := exec.Command(exe, args...)

	// Ensure it's detached
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Stdin = nil

	// Platform-specific detachment (Unix/Windows)
	util.SetDetachedProcess(cmd)

	if err := cmd.Start(); err != nil {
		diag.Printf(" ⚠️  (could not start background server: %v)\n", err)
	} else {
		diag.Printf(" ✅\n")
	}
}

func installClaudeCode(exePath string) error {
	if _, err := exec.LookPath("claude"); err != nil {
		return nil // Claude Code not installed, skip silently
	}

	env := envWithout("CLAUDECODE")

	// add-json fails if the server is already registered; remove any previous
	// registration first so reinstalls stay idempotent. Failures are ignored:
	// "not registered" is the expected first-install outcome.
	removeCmd := exec.Command("claude", "mcp", "remove", "--scope", "user", identity.MCPServerName)
	removeCmd.Env = env
	_ = removeCmd.Run() //nolint:errcheck // best-effort cleanup before re-adding

	entry := map[string]any{
		"command": exePath,
		"args":    []string{},
	}
	data, _ := json.Marshal(entry)

	cmd := exec.Command("claude", "mcp", "add-json", "--scope", "user", identity.MCPServerName)
	cmd.Stdin = strings.NewReader(string(data))
	cmd.Env = env

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("claude mcp add-json failed: %v (output: %s)", err, strings.TrimSpace(string(output)))
	}
	return nil
}

// envWithout returns the current environment minus the named variable.
// Unsetting (rather than appending NAME= present-but-empty) matters for
// present/absent checks like the claude CLI's CLAUDECODE nesting guard.
func envWithout(name string) []string {
	prefix := name + "="
	environ := os.Environ()
	env := make([]string, 0, len(environ))
	for _, kv := range environ {
		if strings.HasPrefix(kv, prefix) {
			continue
		}
		env = append(env, kv)
	}
	return env
}

func mergeJSONConfig(path, key, exePath string, isCustom bool) error {
	data := make(map[string]any)
	if bytes, err := os.ReadFile(path); err == nil {
		if len(bytes) > 0 {
			if err := json.Unmarshal(bytes, &data); err != nil {
				return fmt.Errorf("refusing to overwrite %s: existing file has invalid JSON (%v). Fix the file manually or back it up before retrying", path, err)
			}
		}
	}

	if _, ok := data[key]; !ok {
		data[key] = make(map[string]any)
	}

	servers, ok := data[key].(map[string]any)
	if !ok {
		return fmt.Errorf("unexpected format for key %q", key)
	}
	for _, legacy := range installerLegacyServerKeys {
		delete(servers, legacy)
	}

	if isCustom {
		if key == "mcp" { // OpenCode
			servers[identity.MCPServerName] = map[string]any{
				"type":    "local",
				"command": []string{exePath},
				"enabled": true,
			}
		} else if key == "context_servers" { // Zed
			servers[identity.MCPServerName] = map[string]any{
				"source":  "custom",
				"command": exePath,
				"args":    []string{},
			}
		}
	} else {
		servers[identity.MCPServerName] = map[string]any{
			"command": exePath,
			"args":    []string{},
		}
	}

	out, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}

	// Back up existing file before overwriting.
	if existing, err := os.ReadFile(path); err == nil && len(existing) > 0 {
		_ = os.WriteFile(path+".bak", existing, 0600)
	}

	// Write to a temp file in the same directory and rename into place so a
	// crash mid-write can never leave a truncated client config behind.
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, append(out, '\n'), 0600); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	return os.Rename(tmpPath, path)
}

const (
	connectWaitTimeout = 30 * time.Second
	connectPollEvery   = 750 * time.Millisecond
	connectHealthRead  = 800 * time.Millisecond
)

type Health struct {
	Reachable          bool
	ExtensionConnected bool
	Version            string
	Refused            bool
}

func isConnRefused(err error) bool {
	return errors.Is(err, syscall.ECONNREFUSED)
}

func connectPhase(h Health) string {
	if h.ExtensionConnected {
		return "connected"
	}
	if h.Reachable {
		return "waiting_extension"
	}
	return "daemon_unreachable"
}

func connectProgressLine(phase string) string {
	switch phase {
	case "daemon_unreachable":
		return "   … waiting for the Kaboom server to come up"
	case "waiting_extension":
		return "   … server is up — load the extension in your browser to finish"
	default:
		return ""
	}
}

func connectHintLine(lastPhase string, port int, extDir string) string {
	if lastPhase == "waiting_extension" {
		return fmt.Sprintf(
			"The Kaboom server is running on port %d, but the extension has not connected yet.\n"+
				"   Open chrome://extensions, enable Developer mode, click Load unpacked, and select:\n   %s",
			port, extDir)
	}
	return fmt.Sprintf(
		"The Kaboom server is not answering on port %d yet.\n"+
			"   Re-run the installer, or start Kaboom manually, then load the extension.",
		port)
}

type connectWaitDeps struct {
	fetch func(ctx context.Context, port int) Health
	now   func() time.Time
	after func(time.Duration) <-chan time.Time
	sink  func(string)
}

type connectResult struct {
	connected bool
	aborted   bool
	lastPhase string
}

func waitForExtensionConnected(ctx context.Context, port int, timeout, poll time.Duration, deps connectWaitDeps) connectResult {
	start := deps.now()
	rendered := ""
	for {
		if ctx.Err() != nil {
			return connectResult{aborted: true, lastPhase: rendered}
		}
		phase := connectPhase(deps.fetch(ctx, port))
		if phase != rendered {
			rendered = phase
			if deps.sink != nil {
				if line := connectProgressLine(phase); line != "" {
					deps.sink(line)
				}
			}
		}
		if phase == "connected" {
			return connectResult{connected: true, lastPhase: phase}
		}
		if deps.now().Sub(start) >= timeout {
			return connectResult{lastPhase: phase}
		}
		select {
		case <-ctx.Done():
			return connectResult{aborted: true, lastPhase: rendered}
		case <-deps.after(poll):
		}
	}
}

func FetchHealth(ctx context.Context, port int, timeout time.Duration) Health {
	client := &http.Client{Timeout: timeout}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("http://127.0.0.1:%d/health", port), nil)
	if err != nil {
		return Health{}
	}
	resp, err := client.Do(req)
	if err != nil {
		return Health{Refused: isConnRefused(err)}
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return Health{Reachable: true}
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 256*1024))
	if err != nil {
		return Health{Reachable: true}
	}
	var parsed struct {
		Version string `json:"version"`
		Capture struct {
			ExtensionConnected bool `json:"extension_connected"`
		} `json:"capture"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return Health{Reachable: true}
	}
	return Health{Reachable: true, ExtensionConnected: parsed.Capture.ExtensionConnected, Version: parsed.Version}
}

func installWaitDisabled() bool {
	return envFlagEnabled("KABOOM_NO_WAIT", "KABOOM_INSTALL_NO_WAIT")
}

func isTerminal(f *os.File) bool {
	if f == nil {
		return false
	}
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

func runExtensionConnectWait(port int, extDir string) {
	if installWaitDisabled() || !isTerminal(os.Stderr) {
		return
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	diag.Printf("\n\033[1;33m⏳ Waiting for the browser extension to connect (Ctrl-C to skip)…\033[0m\n")
	res := waitForExtensionConnected(ctx, port, connectWaitTimeout, connectPollEvery, connectWaitDeps{
		fetch: func(c context.Context, p int) Health { return FetchHealth(c, p, connectHealthRead) },
		now:   time.Now,
		after: time.After,
		sink:  func(line string) { diag.Printf("%s\n", line) },
	})
	if res.connected {
		diag.Printf("\033[1;32m✅ Extension connected — Kaboom is fully wired up!\033[0m\n")
	} else if res.aborted {
		diag.Printf("\033[1;33m⏭️  Skipped waiting — the Kaboom server is running; load the extension and it will connect.\033[0m\n")
	} else {
		diag.Printf("\033[1;33m⚠️  %s\033[0m\n", connectHintLine(res.lastPhase, port, extDir))
	}
}
