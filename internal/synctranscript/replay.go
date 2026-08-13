// replay.go — A fake extension that answers daemon commands from a transcript.
//
// PURPOSE: the connected UAT categories need a browser, an extension and a
// person, so they run nowhere automated and nothing notices when a browser
// feature breaks. This client speaks the extension's half of /sync from a
// recording, which turns those categories into a job CI can run.
//
// CONTRACT: it identifies as the extension because a probe is deliberately
// never adopted as the session and never receives commands (see
// internal/extclient). It answers only what the transcript recorded, and
// reports anything else as a failed command rather than an empty success.

package synctranscript

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture/syncruntime"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/commandcontract"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/wirecodec"
)

const (
	// clientIdentity is what the daemon's extension-only guard admits, and what
	// the sync runtime will adopt as the authoritative session.
	clientIdentity = "kaboom-extension"
	// replayVersion tells anyone reading daemon diagnostics that the session is
	// a recording, not a browser.
	replayVersion = "0.0.0-replay"
	// requestTimeout exceeds the daemon's 5s long-poll hold with margin.
	requestTimeout = 8 * time.Second
	// idlePoll is the pause between syncs when the daemon asks for none.
	idlePoll = 50 * time.Millisecond
)

// TrackedTab is the page the fake extension claims to be tracking. Most
// commands are gated on a tracked tab, so without one every connected category
// fails on "no tab is being tracked" rather than exercising anything.
type TrackedTab struct {
	ID    int
	URL   string
	Title string
}

// Options configures a replay client.
type Options struct {
	Endpoint   string
	Transcript *Transcript
	TrackedTab TrackedTab
	SessionID  string
	HTTPClient *http.Client
}

// Client plays the extension's half of the sync protocol.
type Client struct {
	endpoint   string
	http       *http.Client
	transcript *Transcript
	tab        TrackedTab
	session    string

	generation uint64
	pending    []syncruntime.SyncCommandResult
	answered   int
}

func NewClient(options Options) *Client {
	client := options.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: requestTimeout}
	}
	session := options.SessionID
	if session == "" {
		session = "replay-session"
	}
	transcript := options.Transcript
	if transcript == nil {
		transcript = NewTranscript(nil)
	}
	return &Client{
		endpoint:   options.Endpoint,
		http:       client,
		transcript: transcript,
		tab:        options.TrackedTab,
		session:    session,
	}
}

// Answered reports how many commands were served from the transcript.
func (c *Client) Answered() int { return c.answered }

// Run polls until the context is cancelled.
func (c *Client) Run(ctx context.Context) error {
	for {
		if err := ctx.Err(); err != nil {
			return nil
		}
		if err := c.SyncOnce(ctx); err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(idlePoll):
		}
	}
}

// SyncOnce performs one exchange: send queued results, take the next batch of
// commands, and answer them from the transcript.
func (c *Client) SyncOnce(ctx context.Context) error {
	response, err := c.post(ctx, c.buildRequest())
	if err != nil {
		return err
	}
	// Results were accepted with this request, so they must not be sent again;
	// the daemon would see duplicate terminal outcomes for a closed command.
	c.pending = nil
	c.generation = response.ConnectionGeneration

	for _, command := range response.Commands {
		c.pending = append(c.pending, c.answer(command))
	}
	return nil
}

func (c *Client) buildRequest() syncruntime.SyncRequest {
	tracking := c.tab.ID != 0
	return syncruntime.SyncRequest{
		ExtSessionID:         c.session,
		ConnectionGeneration: c.generation,
		ExtensionVersion:     replayVersion,
		// Taken from the generated constant rather than pinned here: the daemon
		// refuses commands to an extension whose contract it does not recognise,
		// and a hardcoded copy would go stale on the next contract change and
		// fail every replayed category with a mismatch nobody expected.
		CommandContractID: commandcontract.ID,
		Settings: &syncruntime.SyncSettings{
			TrackingEnabled: tracking,
			TrackedTabID:    c.tab.ID,
			TrackedTabURL:   c.tab.URL,
			TrackedTabTitle: c.tab.Title,
			TabStatus:       "complete",
			CaptureLogs:     true,
			CaptureNetwork:  true,
			CaptureActions:  true,
		},
		CommandResults: c.pending,
	}
}

// answer resolves one command against the transcript.
func (c *Client) answer(command syncruntime.SyncCommand) syncruntime.SyncCommandResult {
	outcome := syncruntime.SyncCommandResult{
		ID:                   command.ID,
		CorrelationID:        command.CorrelationID,
		ConnectionGeneration: command.ConnectionGeneration,
	}
	record, matched := c.transcript.Match(Command{Type: command.Type, Params: command.Params})
	if !matched {
		// Rule 25: a missing recording is a real failure. Answering "complete"
		// with an empty result would let a category pass on work that never
		// happened — the exact defect this harness exists to catch.
		outcome.Status = "error"
		outcome.Error = fmt.Sprintf(
			"no recorded answer for %s %s; re-record the transcript with KABOOM_SYNC_TRANSCRIPT",
			command.Type, canonical(command.Params))
		return outcome
	}
	c.answered++
	outcome.Status = record.Status
	outcome.Result = record.Result
	outcome.Error = record.Error
	return outcome
}

func (c *Client) post(ctx context.Context, payload syncruntime.SyncRequest) (syncruntime.SyncResponse, error) {
	var empty syncruntime.SyncResponse
	body, err := json.Marshal(payload)
	if err != nil {
		return empty, fmt.Errorf("encode sync request: %w", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint+"/sync", bytes.NewReader(body))
	if err != nil {
		return empty, err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Kaboom-Client", clientIdentity)

	response, err := c.http.Do(request)
	if err != nil {
		return empty, fmt.Errorf("sync request: %w", err)
	}
	defer func() { _ = response.Body.Close() }()

	raw, err := io.ReadAll(response.Body)
	if err != nil {
		return empty, fmt.Errorf("read sync response: %w", err)
	}
	if response.StatusCode != http.StatusOK {
		return empty, fmt.Errorf("sync returned HTTP %d: %s", response.StatusCode, bytes.TrimSpace(raw))
	}
	decoded, err := wirecodec.Decode[syncruntime.SyncResponse](raw)
	if err != nil {
		return empty, fmt.Errorf("sync response: %w", err)
	}
	return decoded, nil
}
