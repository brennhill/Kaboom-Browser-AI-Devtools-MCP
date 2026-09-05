// main.go — Runs the human UAT rig: one case, one call, one person's runlog.Verdict.
//
// Usage:
//   go run ./scripts/uat/human/runner [flags]
//
//   --cases   path to the inventory (default scripts/uat/human/cases.json)
//   --log     run log to append to (default uat-runs/<date>.jsonl)
//   --filter  substring; only cases whose id contains it are presented
//   --redo    present cases that already have an answer
//   --server  MCP binary to drive (default kaboom-mcp from PATH)
//   --dry-run print the cases that would run, make no calls, ask nothing
//
// The run log is append-only and resumable: rerunning the same command picks up
// where the last sitting stopped.

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/scripts/uat/human/inventory"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/scripts/uat/human/runlog"
)

type options struct {
	casesPath string
	logPath   string
	filter    string
	redo      bool
	server    string
	dryRun    bool
	evidence  bool
}

func main() {
	opts := parseFlags()
	if err := run(opts); err != nil {
		fmt.Fprintf(os.Stderr, "human-uat: %v\n", err)
		os.Exit(1)
	}
}

func parseFlags() options {
	var opts options
	flag.StringVar(&opts.casesPath, "cases", inventory.RelativePath, "case inventory to run")
	flag.StringVar(&opts.logPath, "log", "", "run log to append to (default uat-runs/<date>.jsonl)")
	flag.StringVar(&opts.filter, "filter", "", "only present cases whose id contains this substring")
	flag.BoolVar(&opts.redo, "redo", false, "present cases that already have an answer")
	flag.StringVar(&opts.server, "server", "kaboom-mcp", "MCP server binary to drive")
	flag.BoolVar(&opts.dryRun, "dry-run", false, "list the cases that would run and exit")
	evidenceOff := flag.Bool("no-evidence", false, "skip the screenshot/console/network capture around each case")
	flag.Parse()
	opts.evidence = !*evidenceOff
	if opts.logPath == "" {
		opts.logPath = filepath.Join("uat-runs", time.Now().UTC().Format("2006-01-02")+".jsonl")
	}
	return opts
}

func run(opts options) error {
	loaded, err := inventory.Load(opts.casesPath)
	if err != nil {
		return err
	}
	selected := selectCases(loaded.Cases, opts.filter)
	if len(selected) == 0 {
		return fmt.Errorf("no case id contains %q", opts.filter)
	}
	if err := os.MkdirAll(filepath.Dir(opts.logPath), 0o755); err != nil {
		return err
	}
	log, err := runlog.OpenLog(opts.logPath)
	if err != nil {
		return err
	}
	defer log.Close()

	if opts.dryRun {
		return listCases(selected, log, opts.redo)
	}

	mcpSession, err := spawn(opts.server, []string{"KABOOM_TELEMETRY=off"})
	if err != nil {
		return fmt.Errorf("start %s (is it on PATH? `npm i -g kaboom-mcp`): %w", opts.server, err)
	}
	defer mcpSession.shutdown()
	if err := mcpSession.initialize(); err != nil {
		return err
	}

	runID := time.Now().UTC().Format("20060102T150405Z")
	session := &session{
		log:         log,
		mcpSession:  mcpSession,
		prompt:      newPrompter(os.Stdin, os.Stdout),
		runID:       runID,
		buildSHA:    buildSHA(),
		evidenceDir: evidenceDir(opts, runID),
	}
	if err := session.presentAll(selected, opts.redo); err != nil {
		return err
	}
	reportTally(loaded.Cases, log, opts.logPath)
	return nil
}

// evidenceDir is a directory beside the run log, named for the run.
//
// Beside the log and not inside a temp dir: the log records the paths, and a
// reader opening a FAIL a week later has to be able to find them.
func evidenceDir(opts options, runID string) string {
	if !opts.evidence {
		return ""
	}
	return filepath.Join(filepath.Dir(opts.logPath), "evidence", runID)
}

// selectCases applies the id filter, preserving inventory order so two runs
// present the same cases in the same sequence.
func selectCases(cases []inventory.Case, filter string) []inventory.Case {
	if filter == "" {
		return cases
	}
	var selected []inventory.Case
	for _, c := range cases {
		if strings.Contains(c.ID, filter) {
			selected = append(selected, c)
		}
	}
	return selected
}

func listCases(cases []inventory.Case, log *runlog.Log, redo bool) error {
	shown := 0
	for _, c := range cases {
		if _, answered := log.Answered(c.ID); answered && !redo {
			continue
		}
		fmt.Printf("%-40s %s\n", c.ID, c.Question)
		shown++
	}
	fmt.Printf("\n%d case(s) to run.\n", shown)
	return nil
}

func reportTally(cases []inventory.Case, log *runlog.Log, logPath string) {
	tally := runlog.Summarize(cases, log)
	fmt.Printf("\n────────────────────────────────────────────────────────\n")
	fmt.Printf("PASS %d   FAIL %d   BLOCKED %d   SKIPPED %d   UNANSWERED %d   (of %d)\n",
		tally.Pass, tally.Fail, tally.Blocked, tally.Skipped, tally.Unanswered, len(cases))
	if failed := runlog.FailedCases(log); len(failed) > 0 {
		fmt.Printf("\nFAILED:\n  %s\n", strings.Join(failed, "\n  "))
	}
	fmt.Printf("\nRun log: %s\n", logPath)
}

// buildSHA records which build was judged.
//
// A runlog.Verdict without it cannot be compared across runs: "screenshot failed" means
// nothing if nobody can tell whether it was judged before or after a fix.
func buildSHA() string {
	out, err := exec.Command("git", "rev-parse", "--short", "HEAD").Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(out))
}

// marshalRequest renders the call for the record and for the person to read.
func marshalRequest(tool string, arguments map[string]any) json.RawMessage {
	encoded, err := json.Marshal(map[string]any{"name": tool, "arguments": arguments})
	if err != nil {
		return nil
	}
	return encoded
}
