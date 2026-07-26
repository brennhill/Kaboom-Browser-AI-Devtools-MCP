// Purpose: Defines and evaluates the upload sensitive-path denylist.
// Why: Keeps denylist construction/matching isolated from request-level validation flow.
// Docs: docs/features/feature/file-upload/index.md

package uploadsec

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// ============================================
// Sensitive Path Denylist
// ============================================

// BuiltinDenyPatterns are hardcoded sensitive path patterns that cannot be removed.
// Patterns use filepath.Match syntax. Prefix ~ is expanded to the user's home directory.
var BuiltinDenyPatterns []DenyPattern

// DenyPattern represents a sensitive path pattern.
type DenyPattern struct {
	Prefix  string // resolved absolute path prefix to match
	Display string // display pattern (with ~ for error messages)
	Desc    string // human-readable description
}

// #lizard forgives
func init() {
	home, _ := os.UserHomeDir()

	if home != "" {
		homePatterns := []struct {
			pattern string
			desc    string
		}{
			{"~/.ssh", "SSH directory"},
			{"~/.gnupg", "GPG directory"},
			{"~/.aws", "AWS credentials"},
			{"~/.config/gcloud", "GCP credentials"},
			{"~/.azure", "Azure credentials"},
			{"~/.config/doctl", "DigitalOcean credentials"},
			{"~/.kube", "Kubernetes config"},
			{"~/.bash_history", "shell history"},
			{"~/.zsh_history", "shell history"},
			{"~/.node_repl_history", "shell history"},
			{"~/.python_history", "shell history"},
			{"~/Library/Application Support/Google/Chrome", "browser data"},
			{"~/.config/google-chrome", "browser data"},
			{"~/.config/chromium", "browser data"},
			{"~/Library/Application Support/Firefox", "browser data"},
			{"~/.mozilla/firefox", "browser data"},
			{"~/.npmrc", "npm credentials"},
			{"~/.pypirc", "PyPI credentials"},
			{"~/.docker/config.json", "Docker credentials"},
			{"~/.config/gh/hosts.yml", "GitHub CLI credentials"},
		}
		for _, rule := range homePatterns {
			expanded := strings.Replace(rule.pattern, "~", home, 1)
			expanded = filepath.Clean(expanded)
			BuiltinDenyPatterns = append(BuiltinDenyPatterns, DenyPattern{
				Prefix:  expanded,
				Display: rule.pattern,
				Desc:    rule.desc,
			})
		}
	}

	systemPaths := []struct {
		path string
		desc string
	}{
		{"/etc/shadow", "system password file"},
		{"/etc/passwd", "system user file"},
		{"/etc/sudoers", "sudoers file"},
		{"/proc", "proc filesystem"},
		{"/sys", "sys filesystem"},
		{"/root/.ssh", "root SSH directory"},
		{"/root/.gnupg", "root GPG directory"},
		{"/root/.aws", "root AWS credentials"},
		{"/root/.config/gcloud", "root GCP credentials"},
		{"/root/.azure", "root Azure credentials"},
		{"/root/.kube", "root Kubernetes config"},
		{"/root/.npmrc", "root npm credentials"},
		{"/root/.docker", "root Docker config"},
		{"/root/.bash_history", "root shell history"},
		{"/Library/Keychains", "system keychain"},
	}
	for _, path := range systemPaths {
		BuiltinDenyPatterns = append(BuiltinDenyPatterns, DenyPattern{
			Prefix:  filepath.Clean(path.path),
			Display: path.path,
			Desc:    path.desc,
		})
	}

	if runtime.GOOS == "windows" {
		winPaths := []struct {
			path string
			desc string
		}{
			{`C:\Windows\System32\config`, "Windows registry hives"},
			{`C:\Windows\System32\drivers\etc`, "Windows hosts file"},
		}
		for _, path := range winPaths {
			BuiltinDenyPatterns = append(BuiltinDenyPatterns, DenyPattern{
				Prefix:  filepath.Clean(path.path),
				Display: path.path,
				Desc:    path.desc,
			})
		}
	}
}

// PathsEqualFold returns true if a == b, using case-insensitive comparison
// on macOS and Windows (which have case-insensitive filesystems by default).
func PathsEqualFold(a, b string) bool {
	if runtime.GOOS == "darwin" || runtime.GOOS == "windows" {
		return strings.EqualFold(a, b)
	}
	return a == b
}

// PathHasPrefixFold returns true if s starts with prefix, using case-insensitive
// comparison on macOS and Windows.
func PathHasPrefixFold(s, prefix string) bool {
	if runtime.GOOS == "darwin" || runtime.GOOS == "windows" {
		return len(s) >= len(prefix) && strings.EqualFold(s[:len(prefix)], prefix)
	}
	return strings.HasPrefix(s, prefix)
}

// SensitiveExtensions maps file extensions to their denylist pattern display string.
var SensitiveExtensions = map[string]string{
	".pem":      "**/*.pem",
	".key":      "**/*.key",
	".p12":      "**/*.p12",
	".pfx":      "**/*.pfx",
	".keystore": "**/*.keystore",
}

// MatchesDenylist checks if a resolved path matches any built-in deny pattern.
// Returns the matching display pattern and true if denied.
func MatchesDenylist(resolvedPath string) (string, bool) {
	if pattern, matched := matchesPrefixDenylist(resolvedPath); matched {
		return pattern, true
	}
	return matchesBasenameDenylist(resolvedPath)
}

// matchesPrefixDenylist checks against directory-prefix-based deny patterns.
func matchesPrefixDenylist(resolvedPath string) (string, bool) {
	for _, pattern := range BuiltinDenyPatterns {
		if PathsEqualFold(resolvedPath, pattern.Prefix) || PathHasPrefixFold(resolvedPath, pattern.Prefix+string(filepath.Separator)) {
			return pattern.Display, true
		}
	}
	return "", false
}

// matchesBasenameDenylist checks against filename-based deny patterns.
func matchesBasenameDenylist(resolvedPath string) (string, bool) {
	baseLower := strings.ToLower(filepath.Base(resolvedPath))

	if baseLower == ".env" || strings.HasPrefix(baseLower, ".env.") {
		return "**/.env*", true
	}

	ext := strings.ToLower(filepath.Ext(resolvedPath))
	if pattern, ok := SensitiveExtensions[ext]; ok {
		return pattern, true
	}

	dir := filepath.Dir(resolvedPath)
	if strings.EqualFold(filepath.Base(dir), ".git") && baseLower == "config" {
		return "**/.git/config", true
	}

	return "", false
}

// MatchesUserDenylist checks if a resolved path matches any user-defined deny pattern.
func MatchesUserDenylist(resolvedPath string, patterns []string) (string, bool) {
	for _, pattern := range patterns {
		if matched, _ := filepath.Match(pattern, resolvedPath); matched {
			return pattern, true
		}
		if matched, _ := filepath.Match(pattern, filepath.Base(resolvedPath)); matched {
			return pattern, true
		}
	}
	return "", false
}
