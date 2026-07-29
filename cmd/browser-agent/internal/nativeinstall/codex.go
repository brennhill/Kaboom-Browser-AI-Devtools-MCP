// codex.go — Comment-preserving native Codex MCP configuration.
package nativeinstall

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const codexServerName = "kaboom-browser-devtools"

var tomlTableHeader = regexp.MustCompile(`^\s*\[\[?\s*([^\]]+?)\s*\]\]?\s*(?:#.*)?$`)

type installTargets struct {
	codexOnly bool
}

func parseInstallTargets(args []string) (installTargets, error) {
	if len(args) == 0 {
		return installTargets{}, nil
	}
	if len(args) == 1 && strings.EqualFold(strings.TrimSpace(args[0]), "codex") {
		return installTargets{codexOnly: true}, nil
	}
	return installTargets{}, fmt.Errorf("unknown install target %q (supported: codex)", strings.Join(args, " "))
}

func codexConfigPath(home string) string {
	if override := strings.TrimSpace(os.Getenv("CODEX_HOME")); override != "" {
		return filepath.Join(override, "config.toml")
	}
	return filepath.Join(home, ".codex", "config.toml")
}

func tomlString(value string) string {
	escaped := strings.ReplaceAll(value, `\`, `\\`)
	escaped = strings.ReplaceAll(escaped, `"`, `\"`)
	return `"` + escaped + `"`
}

func codexTableTargetsManagedServer(path string) bool {
	const prefix = "mcp_servers."
	if !strings.HasPrefix(path, prefix) {
		return false
	}
	rest := strings.TrimPrefix(path, prefix)
	name := strings.SplitN(rest, ".", 2)[0]
	name = strings.Trim(name, `"`)
	return name == codexServerName
}

func stripManagedCodexBlock(content string) string {
	lines := strings.Split(content, "\n")
	kept := make([]string, 0, len(lines))
	skipping := false
	for _, line := range lines {
		if match := tomlTableHeader.FindStringSubmatch(line); match != nil {
			skipping = codexTableTargetsManagedServer(match[1])
		}
		if !skipping {
			kept = append(kept, line)
		}
	}
	return strings.TrimSpace(strings.Join(kept, "\n"))
}

func mergeCodexConfig(path, executable string) error {
	existing, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read Codex config: %w", err)
	}
	head := stripManagedCodexBlock(string(existing))
	block := strings.Join([]string{
		"[mcp_servers." + codexServerName + "]",
		"# Managed by Kaboom. This approves all five Kaboom MCP tools.",
		"command = " + tomlString(executable),
		`default_tools_approval_mode = "approve"`,
	}, "\n")
	next := block + "\n"
	if head != "" {
		next = head + "\n\n" + block + "\n"
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create Codex config directory: %w", err)
	}
	temp := path + ".tmp"
	if err := os.WriteFile(temp, []byte(next), 0o600); err != nil {
		return fmt.Errorf("write Codex config: %w", err)
	}
	if err := os.Rename(temp, path); err != nil {
		_ = os.Remove(temp)
		return fmt.Errorf("replace Codex config: %w", err)
	}
	return nil
}
