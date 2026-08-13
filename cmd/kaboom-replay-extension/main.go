// main.go — Plays the browser extension from a recorded transcript.
//
// PURPOSE: the connected UAT categories are the only tests that prove a browser
// feature still works, and they need Chrome, the extension and a person — so
// they run nowhere automated and nothing notices when one breaks. This binary
// answers the daemon's commands from a recording, which lets those categories
// run as a headless job.
//
// Record once against a real browser:
//
//	KABOOM_SYNC_TRANSCRIPT=fixtures/connected.jsonl kaboom-agentic-browser --daemon
//
// Replay:
//
//	kaboom-replay-extension --port 7890 --transcript fixtures/connected.jsonl

package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/synctranscript"
)

const (
	// healthTimeout bounds the pre-flight check against the daemon.
	healthTimeout = 3 * time.Second
	// maxHealthBody bounds the pre-flight read.
	maxHealthBody = 1 << 20
)

type options struct {
	port       int
	transcript string
	tabID      int
	tabURL     string
	tabTitle   string
	force      bool
	strict     bool
}

func main() {
	opts := parseFlags()
	if err := run(opts); err != nil {
		fmt.Fprintln(os.Stderr, "kaboom-replay-extension:", err)
		os.Exit(1)
	}
}

func parseFlags() options {
	var opts options
	flag.IntVar(&opts.port, "port", 7890, "daemon port")
	flag.StringVar(&opts.transcript, "transcript", "", "path to a recorded command transcript (required)")
	flag.IntVar(&opts.tabID, "tab-id", 0, "tab id to report as tracked (default: the one the transcript recorded)")
	flag.StringVar(&opts.tabURL, "tab-url", "", "URL to report as tracked (default: the one the transcript recorded)")
	flag.StringVar(&opts.tabTitle, "tab-title", "", "title to report as tracked")
	flag.BoolVar(&opts.force, "force", false, "replay even if a real extension is already connected")
	flag.BoolVar(&opts.strict, "strict", true, "exit non-zero if any command had no recorded answer")
	flag.Parse()
	return opts
}

func run(opts options) error {
	if opts.transcript == "" {
		return fmt.Errorf("--transcript is required; record one with KABOOM_SYNC_TRANSCRIPT=<path> on the daemon")
	}
	file, err := os.Open(opts.transcript)
	if err != nil {
		return fmt.Errorf("open transcript: %w", err)
	}
	records, decodeErr := synctranscript.Decode(file)
	_ = file.Close()
	if decodeErr != nil {
		return decodeErr
	}
	if len(records) == 0 {
		// A transcript that answers nothing is indistinguishable from a browser
		// that never responded, and every category would fail on a timeout with
		// no indication that the fixture was the problem.
		return fmt.Errorf("%s contains no exchanges; re-record it", opts.transcript)
	}

	endpoint := fmt.Sprintf("http://127.0.0.1:%d", opts.port)
	daemon := probeDaemon(endpoint)
	if daemon.extensionConnected && !opts.force {
		return fmt.Errorf("a real extension is already connected to %s; replaying would take over its session. Pass --force if that is intended", endpoint)
	}

	tab := resolveTrackedTab(opts, records)
	transcript := synctranscript.NewTranscript(records)
	client := synctranscript.NewClient(synctranscript.Options{
		Endpoint:   endpoint,
		Transcript: transcript,
		TrackedTab: tab,
		SessionID:  fmt.Sprintf("replay-%d", os.Getpid()),
		// Report the daemon's own version. Anything else makes the daemon
		// prepend a version-mismatch warning to every tool response, and the
		// categories assert on response text — so a cosmetic mismatch would
		// show up as content failures that have nothing to do with the feature.
		Version: daemon.version,
	})
	fmt.Fprintf(os.Stderr, "kaboom-replay-extension: reporting tracked tab %d (%s)\n", tab.ID, tab.URL)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	fmt.Fprintf(os.Stderr, "kaboom-replay-extension: %d exchange(s) loaded, polling %s/sync\n", len(records), endpoint)
	if err := client.Run(ctx); err != nil {
		return err
	}
	return report(transcript, client, opts.strict)
}

// resolveTrackedTab prefers the tab the transcript recorded, because the UAT
// harness compares the tracked id against what `observe tabs` returns — and
// that answer comes from the same transcript. A hand-set id that disagrees with
// the recording makes readiness fail for a reason nobody would guess.
func resolveTrackedTab(opts options, records []synctranscript.Record) synctranscript.TrackedTab {
	tab, found := synctranscript.TrackedTabFromRecords(records)
	if !found {
		// EXPECTED_ABSENCE: a transcript covering only offline commands never
		// observed the tab list, so the flags are the only source.
		tab = synctranscript.TrackedTab{ID: 1, URL: "http://127.0.0.1/replay", Title: "kaboom replay"}
	}
	if opts.tabID != 0 {
		tab.ID = opts.tabID
	}
	if opts.tabURL != "" {
		tab.URL = opts.tabURL
	}
	if opts.tabTitle != "" {
		tab.Title = opts.tabTitle
	}
	return tab
}

// daemonState is what the pre-flight learns about the daemon being replayed to.
type daemonState struct {
	extensionConnected bool
	version            string
}

// probeDaemon reads /health once, for two decisions: whether a real browser is
// already attached, and which version to impersonate.
//
// The replay client identifies as the extension because a probe is never
// adopted as the session and never receives commands. That means starting it
// against a developer's daemon would take over their browser's session and
// silently break their tooling.
//
// A daemon that cannot be read yields the zero value: the harness starts the
// daemon and the client together, so "not up yet" is normal and the sync loop
// retries on its own. Absence of an answer is not evidence of a real browser.
func probeDaemon(endpoint string) daemonState {
	client := &http.Client{Timeout: healthTimeout}
	response, err := client.Get(endpoint + "/health")
	if err != nil {
		// EXPECTED_ABSENCE: see above — the daemon may not be listening yet.
		return daemonState{}
	}
	defer func() { _ = response.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(response.Body, maxHealthBody))
	if err != nil {
		// EXPECTED_ABSENCE: an unreadable health body is not evidence that a
		// real extension is attached, and blocking on it would make the guard
		// more disruptive than the case it protects against.
		return daemonState{}
	}
	// extension_connected is nested under capture. Reading the top level
	// would make the guard always pass, so it would never protect the
	// developer session it exists to protect.
	var payload struct {
		Version string `json:"version"`
		Capture struct {
			ExtensionConnected bool `json:"extension_connected"`
		} `json:"capture"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		// EXPECTED_ABSENCE: as above.
		return daemonState{}
	}
	return daemonState{extensionConnected: payload.Capture.ExtensionConnected, version: payload.Version}
}

// report summarises coverage. Under --strict an unanswered command fails the
// run: a category that silently received "no recorded answer" for everything
// would otherwise look like a browser problem rather than a stale fixture.
func report(transcript *synctranscript.Transcript, client *synctranscript.Client, strict bool) error {
	misses := transcript.Misses()
	unused := transcript.Unused()
	fmt.Fprintf(os.Stderr, "kaboom-replay-extension: answered %d command(s), %d unmatched, %d recording(s) unused\n",
		client.Answered(), totalMisses(misses), len(unused))

	for kind, count := range misses {
		fmt.Fprintf(os.Stderr, "  no recorded answer: %s x%d\n", kind, count)
	}
	for _, record := range unused {
		fmt.Fprintf(os.Stderr, "  never requested: %s\n", record.Type)
	}
	if strict && len(misses) > 0 {
		return fmt.Errorf("%d command(s) had no recorded answer; re-record the transcript", totalMisses(misses))
	}
	return nil
}

func totalMisses(misses map[string]int) int {
	total := 0
	for _, count := range misses {
		total += count
	}
	return total
}
