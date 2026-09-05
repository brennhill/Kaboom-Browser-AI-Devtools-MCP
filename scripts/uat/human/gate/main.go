// main.go — The release gate: refuses a tag whose build nobody judged.
//
// Usage:
//   go run ./scripts/uat/human/gate --log uat-runs/2026-09-05.jsonl [--build <sha>]
//
// Exit 0 only when every case in the inventory was answered against THIS build
// and none of them failed, counting a recorded waiver as an answer. Exit 1
// otherwise, naming what is missing.
//
// Answered against this build is the point. A run against yesterday's binary
// says nothing about the one being published, and "we ran it last week" is
// exactly how a release ships a regression that a person had already seen.

package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/scripts/uat/human/inventory"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/scripts/uat/human/runlog"
)

func main() {
	logPath := flag.String("log", "", "run log to check (default: the newest under uat-runs/)")
	casesPath := flag.String("cases", inventory.RelativePath, "case inventory the run is checked against")
	waiversPath := flag.String("waivers", defaultWaiversPath, "recorded waivers")
	build := flag.String("build", "", "build SHA the release is cut from (default: git HEAD)")
	flag.Parse()

	if *logPath == "" {
		newest, err := newestRun(defaultRunDir)
		if err != nil {
			// No run at all is the commonest way this gate fires, and the message
			// has to say what to do rather than name a missing file.
			fmt.Fprintf(os.Stderr, "human-uat-gate: %v\nRun `make uat-human` against this build first.\n", err)
			os.Exit(1)
		}
		*logPath = newest
	}
	if *build == "" {
		*build = headSHA()
	}

	verdict, err := check(*casesPath, *logPath, *waiversPath, *build)
	if err != nil {
		fmt.Fprintf(os.Stderr, "human-uat-gate: %v\n", err)
		os.Exit(2)
	}
	fmt.Print(verdict.report())
	if !verdict.Passed {
		os.Exit(1)
	}
}

// check reads the three inputs and produces the verdict.
func check(casesPath, logPath, waiversPath, build string) (gateVerdict, error) {
	loaded, err := inventory.Load(casesPath)
	if err != nil {
		return gateVerdict{}, err
	}
	log, err := runlog.OpenLog(logPath)
	if err != nil {
		return gateVerdict{}, err
	}
	defer log.Close()

	waivers, err := loadWaivers(waiversPath)
	if err != nil {
		return gateVerdict{}, err
	}
	return judge(loaded.Cases, log, waivers, build), nil
}

// headSHA is the build being released when the caller did not name one.
func headSHA() string {
	out, err := exec.Command("git", "rev-parse", "--short", "HEAD").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// defaultWaiversPath is where accepted risks are recorded, from the repo root.
const defaultWaiversPath = "scripts/uat/human/waivers.json"

// defaultRunDir is where the runner writes its logs.
const defaultRunDir = "uat-runs"

// newestRun returns the most recent run log in dir.
//
// Most recent by modification time rather than by filename: a log is appended to
// across sittings, so the one last written to is the one that describes the
// build in hand, whatever date is in its name.
func newestRun(dir string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", fmt.Errorf("no human UAT runs found in %s/", dir)
	}
	var newest string
	var newestAt int64
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".jsonl") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if at := info.ModTime().UnixNano(); at > newestAt {
			newest, newestAt = filepath.Join(dir, entry.Name()), at
		}
	}
	if newest == "" {
		return "", fmt.Errorf("no human UAT run log in %s/", dir)
	}
	return newest, nil
}
