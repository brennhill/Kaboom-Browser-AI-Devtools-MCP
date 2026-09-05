// browser_launch_test.go — Holds the UAT browser launcher to refusing a shared daemon.
//
// PURPOSE: the daemon keeps ONE extension slot. extension_connected and
// command_contract_id belong to whichever browser checked in last, so two
// browsers on one port produce command_contract_mismatch at random — which is
// exactly what a recording run looked like on 2026-09-05: a freshly compiled
// extension was loaded and every connected category still failed, because the
// developer's own Chrome was polling the same port.
//
// CONTRACT: the launcher refuses to start a second browser against a daemon that
// already reports an extension, and it says why rather than racing.

package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const launcherFile = "scripts/uat/orchestration/uat-browser-launch.sh"

// healthServer stands in for a daemon reporting whether an extension is attached.
func healthServer(t *testing.T, extensionConnected bool) string {
	t.Helper()
	body := `{"status":"ok","capture":{"extension_connected":false}}`
	if extensionConnected {
		body = `{"status":"ok","capture":{"extension_connected":true}}`
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/health" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)
	return strings.TrimPrefix(server.URL, "http://127.0.0.1:")
}

// runLauncherFunc sources the launcher and calls one of its functions.
func runLauncherFunc(t *testing.T, env []string, args ...string) (combined string, exitCode int) {
	t.Helper()
	script := ". " + filepath.Join(repoRoot(t), launcherFile) + "\n\"$@\""
	cmd := exec.Command("bash", append([]string{"-c", script, "bash"}, args...)...)
	cmd.Env = append(os.Environ(), env...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return string(out), exitErr.ExitCode()
		}
		t.Fatalf("running the launcher failed for a reason other than its exit status: %v", err)
	}
	return string(out), 0
}

func TestTheLauncherRefusesADaemonThatAlreadyHasAnExtension(t *testing.T) {
	t.Parallel()
	port := healthServer(t, true)

	out, code := runLauncherFunc(t, nil, "uat_assert_sole_extension", port)
	if code == 0 {
		t.Fatal("the launcher accepted a daemon another browser is already attached to; the two would race for its single extension slot and fail at random")
	}
	if !strings.Contains(out, "already attached") {
		t.Errorf("the refusal does not say what is wrong, so the operator cannot fix it: %q", out)
	}
}

func TestTheLauncherProceedsWhenNothingIsAttached(t *testing.T) {
	t.Parallel()
	// Control: without this, a check that always refused would satisfy the test
	// above and no browser could ever be launched.
	port := healthServer(t, false)

	if out, code := runLauncherFunc(t, nil, "uat_assert_sole_extension", port); code != 0 {
		t.Errorf("the launcher refused an unattached daemon (exit %d): %q", code, out)
	}
}

func TestTheLauncherProceedsWhenNoDaemonIsListening(t *testing.T) {
	t.Parallel()
	// A closed port means nothing is attached. Treating an unreachable daemon as
	// "occupied" would make the launcher unusable in the one case it exists for:
	// bringing up a browser before the categories start their own daemons.
	if out, code := runLauncherFunc(t, nil, "uat_assert_sole_extension", "1"); code != 0 {
		t.Errorf("the launcher refused a port with no daemon on it (exit %d): %q", code, out)
	}
}

func TestChromeDiscoveryRefusesStableChrome(t *testing.T) {
	t.Parallel()
	// Stable Chrome has ignored --load-extension since 137. Measured on 152: a
	// browser launched with it sends no request to the daemon port in 120s,
	// exposes no service worker, and records no extension in its profile. If
	// discovery returned one, the caller would wait out its whole timeout with
	// nothing to look at, which is how this was first mistaken for a flake.
	out, code := runLauncherFunc(t, []string{"KABOOM_UAT_CHROME="}, "uat_find_chrome")
	if code == 0 && strings.Contains(out, "/Applications/Google Chrome.app/") {
		t.Errorf("discovery returned stable Chrome (%q); it loads no extension and the launch would time out with no visible cause", strings.TrimSpace(out))
	}
	if code != 0 && !strings.Contains(out, "--load-extension") {
		t.Errorf("the refusal does not say why the browser is unusable: %q", out)
	}
}

func TestChromeDiscoveryHonoursTheOverrideAndRefusesAMissingBinary(t *testing.T) {
	t.Parallel()
	fake := filepath.Join(t.TempDir(), "chrome")
	if err := os.WriteFile(fake, []byte("#!/bin/bash\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	out, code := runLauncherFunc(t, []string{"KABOOM_UAT_CHROME=" + fake}, "uat_find_chrome")
	if code != 0 || strings.TrimSpace(out) != fake {
		t.Errorf("KABOOM_UAT_CHROME=%s resolved to %q (exit %d); a CI runner cannot name its browser", fake, strings.TrimSpace(out), code)
	}

	missing := filepath.Join(t.TempDir(), "not-here")
	out, code = runLauncherFunc(t, []string{"KABOOM_UAT_CHROME=" + missing}, "uat_find_chrome")
	if code == 0 {
		t.Error("a KABOOM_UAT_CHROME that does not exist was accepted; the launch would fail later with no explanation")
	}
	if !strings.Contains(out, "not executable") {
		t.Errorf("the refusal does not name the problem: %q", out)
	}
}

func TestTheLauncherSurvivesSetU(t *testing.T) {
	t.Parallel()
	// The recorder runs under `set -euo pipefail`, and bash 3.2 — what macOS
	// ships — treats "${empty[@]}" as an unbound variable. An empty headless
	// array therefore aborted the recorder before it launched anything, with a
	// message about an array rather than about the browser.
	script := "set -euo pipefail\n. " + filepath.Join(repoRoot(t), launcherFile) +
		"\nuat_launch_extension_browser \"$1\" \"$2\" 65535 2"
	cmd := exec.Command("bash", "-c", script, "bash",
		filepath.Join(repoRoot(t), "extension"), t.TempDir())
	cmd.Env = append(os.Environ(), "KABOOM_UAT_CHROME=/bin/echo")
	out, _ := cmd.CombinedOutput()

	if strings.Contains(string(out), "unbound variable") {
		t.Errorf("the launcher aborted on an unbound variable under set -u: %q", out)
	}
	// It must get far enough to report the real problem rather than a shell one.
	if !strings.Contains(string(out), "never reported in") {
		t.Errorf("the launcher did not reach its own timeout report: %q", out)
	}
}
