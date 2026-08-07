// pid.go — Detects browser process IDs for upload automation targeting.
package osauto

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

type pidCommandOutput func(context.Context, string, ...string) ([]byte, error)

func productionPIDCommandOutput(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).Output()
}

func detectBrowserPIDDarwin(run pidCommandOutput) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := run(ctx, "pgrep", "-x", "Google Chrome")
	if err != nil {
		return 0, fmt.Errorf("Cannot detect Chrome: pgrep -x 'Google Chrome' found no process. Launch Google Chrome first")
	}

	lines := strings.SplitN(strings.TrimSpace(string(out)), "\n", 2)
	if len(lines) == 0 || lines[0] == "" {
		return 0, fmt.Errorf("Cannot detect Chrome: pgrep -x 'Google Chrome' found no process. Launch Google Chrome first")
	}
	var pid int
	if _, err := fmt.Sscanf(lines[0], "%d", &pid); err != nil {
		return 0, fmt.Errorf("Cannot detect Chrome: pgrep -x 'Google Chrome' returned non-numeric PID %q", lines[0])
	}
	return pid, nil
}

func detectBrowserPIDLinux(run pidCommandOutput) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for _, name := range []string{"chrome", "chromium", "google-chrome", "chromium-browser"} {
		out, err := run(ctx, "pgrep", "-x", name)
		if err != nil {
			continue
		}
		pid, ok := parseFirstPIDLine(string(out))
		if ok {
			return pid, nil
		}
	}
	return 0, fmt.Errorf("Cannot detect Chrome: pgrep found none of 'chrome', 'chromium', 'google-chrome', 'chromium-browser'. Launch Chrome/Chromium first")
}

func detectBrowserPIDWindows(run pidCommandOutput) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := run(ctx, "tasklist", "/FI", "IMAGENAME eq chrome.exe", "/FO", "CSV", "/NH")
	if err != nil {
		return 0, fmt.Errorf("Cannot detect Chrome: tasklist found no chrome.exe. Launch Google Chrome first")
	}

	lines := strings.SplitN(strings.TrimSpace(string(out)), "\n", 2)
	if len(lines) == 0 || strings.Contains(lines[0], "No tasks") {
		return 0, fmt.Errorf("Cannot detect Chrome: tasklist found no chrome.exe. Launch Google Chrome first")
	}

	fields := strings.Split(lines[0], ",")
	if len(fields) < 2 {
		return 0, fmt.Errorf("Cannot detect Chrome: tasklist returned unexpected format")
	}

	pidStr := strings.Trim(fields[1], "\" ")
	var pid int
	if _, err := fmt.Sscanf(pidStr, "%d", &pid); err != nil {
		return 0, fmt.Errorf("Cannot detect Chrome: tasklist returned non-numeric PID %q", pidStr)
	}
	return pid, nil
}

func parseFirstPIDLine(output string) (int, bool) {
	lines := strings.SplitN(strings.TrimSpace(output), "\n", 2)
	if len(lines) == 0 || lines[0] == "" {
		return 0, false
	}

	var pid int
	if _, err := fmt.Sscanf(lines[0], "%d", &pid); err != nil {
		return 0, false
	}
	return pid, true
}
