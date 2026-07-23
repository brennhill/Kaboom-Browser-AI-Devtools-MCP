// Purpose: Classifies startup launch mode (persistent vs likely transient) and enforces strict policy when configured.
// Why: Reduces setup churn by warning users when runtime context is likely to disconnect after process exit.

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
)

const (
	launchModePersistent      = "persistent"
	launchModeLikelyTransient = "likely_transient"
)

type launchModeInfo struct {
	Mode            string
	Reason          string
	ParentProcess   string
	IsTTY           bool
	StrictRequired  bool
	UnderSupervisor bool
}

var (
	launchModeMu      sync.RWMutex
	currentLaunchMode = launchModeInfo{Mode: launchModePersistent, Reason: "default"}
)

var lookupParentProcessName = detectParentProcessName

func setCurrentLaunchMode(info launchModeInfo) {
	launchModeMu.Lock()
	defer launchModeMu.Unlock()
	currentLaunchMode = info
}

func getCurrentLaunchMode() launchModeInfo {
	launchModeMu.RLock()
	defer launchModeMu.RUnlock()
	return currentLaunchMode
}

func classifyLaunchMode(cfg *serverConfig, isTTY bool) launchModeInfo {
	parent := lookupParentProcessName()
	supervised := isSupervisedLaunch()
	strict := isPersistentModeRequired()

	if cfg != nil && cfg.daemonMode {
		return launchModeInfo{
			Mode:            launchModePersistent,
			Reason:          "daemon_flag_enabled",
			ParentProcess:   parent,
			IsTTY:           isTTY,
			StrictRequired:  strict,
			UnderSupervisor: supervised,
		}
	}

	if supervised {
		return launchModeInfo{
			Mode:            launchModePersistent,
			Reason:          "supervisor_detected",
			ParentProcess:   parent,
			IsTTY:           isTTY,
			StrictRequired:  strict,
			UnderSupervisor: true,
		}
	}

	if isTTY {
		reason := "interactive_tty_without_daemon_flag"
		if isLikelyAdHocShell(parent) {
			reason = "interactive_shell_parent"
		}
		return launchModeInfo{
			Mode:            launchModeLikelyTransient,
			Reason:          reason,
			ParentProcess:   parent,
			IsTTY:           true,
			StrictRequired:  strict,
			UnderSupervisor: false,
		}
	}

	return launchModeInfo{
		Mode:            launchModePersistent,
		Reason:          "non_interactive_stdio",
		ParentProcess:   parent,
		IsTTY:           false,
		StrictRequired:  strict,
		UnderSupervisor: false,
	}
}

func buildLaunchModeWarning(info launchModeInfo, port int) string {
	if info.Mode != launchModeLikelyTransient {
		return ""
	}
	reason := info.Reason
	if reason == "" {
		reason = "unknown"
	}
	return fmt.Sprintf("launch_mode_warning: detected %s (%s). This session may disconnect when the process exits. Start persistently: kaboom-agentic-browser --daemon --port %d", info.Mode, reason, port)
}

func enforcePersistentMode(info launchModeInfo) error {
	if !info.StrictRequired || info.Mode != launchModeLikelyTransient {
		return nil
	}
	return fmt.Errorf("KABOOM_REQUIRE_PERSISTENT is enabled and launch mode is %s (%s). Start persistently: kaboom-agentic-browser --daemon --port %d", info.Mode, info.Reason, defaultPort)
}

func isPersistentModeRequired() bool {
	return truthyEnv("KABOOM_REQUIRE_PERSISTENT")
}

func isSupervisedLaunch() bool {
	if truthyEnv(supervisedEnvVar) {
		return true
	}
	for _, key := range supervisorEnvVars {
		if strings.TrimSpace(os.Getenv(key)) != "" {
			return true
		}
	}
	return false
}

// supervisedEnvVar force-marks the process as supervised.
const supervisedEnvVar = "KABOOM_SUPERVISED"

// supervisorEnvVars are the environment markers that imply something is
// supervising this process. Exported to the package so tests can neutralise the
// whole set: a test that clears only some of them silently depends on the CI
// runner's environment. GitHub's Linux runners are started by systemd and carry
// JOURNAL_STREAM, which is what made TestClassifyLaunchMode_* fail there while
// passing on macOS.
var supervisorEnvVars = []string{
	"INVOCATION_ID",      // systemd
	"JOURNAL_STREAM",     // systemd
	"LAUNCH_JOB_NAME",    // launchd
	"LAUNCH_JOB_KEY",     // launchd
	"RUNNING_AS_SERVICE", // explicit service marker
	"SERVICE_NAME",       // generic service marker
	"K_SERVICE",          // Cloud Run
	"K_REVISION",         // Cloud Run
	"CONTAINER",          // container supervisors
	"KABOOM_DAEMONIZED",  // project-local explicit marker
	"KABOOM_PERSISTENT",  // project-local explicit marker
}

func truthyEnv(key string) bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	switch v {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func isLikelyAdHocShell(parent string) bool {
	parent = strings.ToLower(strings.TrimSpace(parent))
	if parent == "" {
		return false
	}
	base := filepath.Base(parent)
	switch base {
	case "bash", "zsh", "sh", "dash", "fish", "nu", "pwsh", "powershell", "cmd", "npm", "npx", "yarn", "pnpm", "bun":
		return true
	default:
		return false
	}
}

func detectParentProcessName() string {
	ppid := os.Getppid()
	if ppid <= 0 {
		return ""
	}
	switch runtime.GOOS {
	case "windows":
		// Keep detection conservative on Windows to avoid expensive shellouts in startup path.
		return ""
	default:
		cmd := exec.Command("ps", "-p", strconv.Itoa(ppid), "-o", "comm=") // #nosec G204 -- fixed command and args derived from numeric ppid
		out, err := cmd.Output()
		if err != nil {
			return ""
		}
		return strings.TrimSpace(string(out))
	}
}
