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
	flag.IntVar(&opts.tabID, "tab-id", 1, "tab id to report as tracked")
	flag.StringVar(&opts.tabURL, "tab-url", "http://127.0.0.1:7890/testpages/interact.html", "URL to report as tracked")
	flag.StringVar(&opts.tabTitle, "tab-title", "kaboom replay", "title to report as tracked")
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
	if err := guardAgainstRealExtension(endpoint, opts.force); err != nil {
		return err
	}

	transcript := synctranscript.NewTranscript(records)
	client := synctranscript.NewClient(synctranscript.Options{
		Endpoint:   endpoint,
		Transcript: transcript,
		TrackedTab: synctranscript.TrackedTab{ID: opts.tabID, URL: opts.tabURL, Title: opts.tabTitle},
		SessionID:  fmt.Sprintf("replay-%d", os.Getpid()),
	})

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	fmt.Fprintf(os.Stderr, "kaboom-replay-extension: %d exchange(s) loaded, polling %s/sync\n", len(records), endpoint)
	if err := client.Run(ctx); err != nil {
		return err
	}
	return report(transcript, client, opts.strict)
}

// guardAgainstRealExtension refuses to displace a live browser session.
//
// The replay client identifies as the extension because a probe is never
// adopted as the session and never receives commands. That means starting it
// against a developer's daemon would take over their browser's session and
// silently break their tooling.
func guardAgainstRealExtension(endpoint string, force bool) error {
	if force {
		return nil
	}
	client := &http.Client{Timeout: healthTimeout}
	response, err := client.Get(endpoint + "/health")
	if err != nil {
		// EXPECTED_ABSENCE: the daemon may not be up yet when the harness
		// starts both together, and the sync loop retries on its own.
		return nil
	}
	defer func() { _ = response.Body.Close() }()

	health, err := decodeHealth(response)
	if err != nil {
		// EXPECTED_ABSENCE: an unreadable health body is not evidence that a
		// real extension is attached, and blocking on it would make the guard
		// more disruptive than the case it protects against.
		return nil
	}
	if health {
		return fmt.Errorf("a real extension is already connected to %s; replaying would take over its session. Pass --force if that is intended", endpoint)
	}
	return nil
}

func decodeHealth(response *http.Response) (bool, error) {
	// extension_connected is nested under capture. Reading the top level
	// would make this guard always pass, so it would never protect the
	// developer session it exists to protect.
	var payload struct {
		Capture struct {
			ExtensionConnected bool `json:"extension_connected"`
		} `json:"capture"`
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, maxHealthBody))
	if err != nil {
		return false, err
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return false, err
	}
	return payload.Capture.ExtensionConnected, nil
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
