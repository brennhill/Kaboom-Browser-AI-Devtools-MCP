// Purpose: End-to-end proof that a recorded transcript can stand in for the browser.
// Docs: docs/features/feature/self-testing/index.md

//go:build integration

// replay_test.go — The connected suite's headless substitute, proven against a real daemon.
//
// The connected UAT categories are the only tests that prove a browser feature
// still works, and they need Chrome, the extension and a person — so they run
// nowhere automated. This test proves the substitute is real: a daemon with no
// browser attached, a fake extension answering from a recording, and an MCP
// tool call that comes back with the recorded payload.
//
// It also proves the recorder half, because a replay fixture nobody can
// regenerate is a fixture that rots.

package replayintegration

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/integrationtest"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/synctranscript"
)

const recordedDOM = `{"elements":[{"tag":"main","selector":"main.content"}],"count":1}`

func TestIntegration_ReplayedExtensionAnswersAnMCPToolCall(t *testing.T) {
	port, endpoint, transcriptPath := startDaemonWithRecording(t)

	transcript := synctranscript.NewTranscript(buildTranscript(t))
	client := synctranscript.NewClient(synctranscript.Options{
		Endpoint:   endpoint,
		Transcript: transcript,
		TrackedTab: synctranscript.TrackedTab{ID: 7, URL: "http://127.0.0.1/fixture.html", Title: "fixture"},
		SessionID:  "replay-integration",
	})
	stopReplay := runReplay(t, client)

	waitForExtensionConnected(t, endpoint)

	body := callTool(t, endpoint, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":`+
		`{"name":"analyze","arguments":{"what":"dom","selector":"main"}}}`)

	// The recorded selector is the discriminating detail: it exists nowhere but
	// the transcript, so finding it proves the answer travelled the whole path
	// rather than being synthesised by the daemon.
	if !strings.Contains(body, "main.content") {
		t.Fatalf("tool response did not carry the recorded payload.\nresponse: %s", truncate(body))
	}
	if client.Answered() == 0 {
		t.Error("the replay client answered no commands, so the daemon never asked it anything")
	}

	stopReplay()
	assertRecorded(t, transcriptPath)
	_ = port
}

// A command with no recording must surface as a failure. If it came back as an
// empty success, every category would pass against a stale transcript — the
// exact defect the whole harness exists to prevent.
func TestIntegration_UnrecordedCommandDoesNotLookLikeACleanResult(t *testing.T) {
	_, endpoint, _ := startDaemonWithRecording(t)

	transcript := synctranscript.NewTranscript(buildTranscript(t))
	client := synctranscript.NewClient(synctranscript.Options{
		Endpoint:   endpoint,
		Transcript: transcript,
		TrackedTab: synctranscript.TrackedTab{ID: 7, URL: "http://127.0.0.1/fixture.html", Title: "fixture"},
		SessionID:  "replay-miss",
	})
	stopReplay := runReplay(t, client)
	defer stopReplay()

	waitForExtensionConnected(t, endpoint)

	body := callTool(t, endpoint, `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":`+
		`{"name":"analyze","arguments":{"what":"dom","selector":".never-recorded"}}}`)

	if strings.Contains(body, "main.content") {
		t.Errorf("an unrecorded selector was answered with another command's payload: %s", truncate(body))
	}
	if misses := transcript.Misses(); len(misses) == 0 {
		t.Error("an unrecorded command was not counted as a miss")
	}
}

func buildTranscript(t *testing.T) []synctranscript.Record {
	t.Helper()
	recorder := synctranscript.NewRecorder()
	// The command type is the daemon's internal name for the work, not the MCP
	// mode the caller asked for: analyze/what=dom is dispatched as "dom".
	recorder.Observe(
		synctranscript.Command{Type: "dom", Params: json.RawMessage(`{"what":"dom","selector":"main"}`)},
		synctranscript.Result{Status: "complete", Result: json.RawMessage(recordedDOM)},
	)
	return recorder.Records()
}

// startDaemonWithRecording brings up a real daemon with no browser attached and
// command recording enabled.
func startDaemonWithRecording(t *testing.T) (int, string, string) {
	t.Helper()
	binary := integrationtest.BuildBinary(t)
	port := integrationtest.FreePort(t)
	transcriptPath := filepath.Join(t.TempDir(), "recorded.jsonl")

	command := integrationtest.StartServer(t, binary, "--daemon", "--parallel", "--port", fmt.Sprint(port))
	command.Env = append(command.Env, "KABOOM_SYNC_TRANSCRIPT="+transcriptPath)
	logPath := filepath.Join(t.TempDir(), "daemon.log")
	logFile, logErr := os.Create(logPath)
	if logErr != nil {
		t.Fatalf("create daemon log: %v", logErr)
	}
	command.Stdout = logFile
	command.Stderr = logFile
	t.Cleanup(func() {
		_ = logFile.Close()
		if t.Failed() {
			if body, err := os.ReadFile(logPath); err == nil {
				t.Logf("daemon output:\n%s", body)
			}
		}
	})
	if err := command.Start(); err != nil {
		t.Fatalf("start daemon: %v", err)
	}
	t.Cleanup(func() {
		if command.Process != nil {
			_ = command.Process.Kill()
			_, _ = command.Process.Wait()
		}
	})

	endpoint := fmt.Sprintf("http://127.0.0.1:%d", port)
	waitForHealth(t, endpoint)
	return port, endpoint, transcriptPath
}

func runReplay(t *testing.T, client *synctranscript.Client) func() {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		if err := client.Run(ctx); err != nil {
			t.Errorf("replay client: %v", err)
		}
	}()
	return func() {
		cancel()
		<-done
	}
}

// pollUntil retries probe on a ticker until it succeeds or the budget expires.
// Ticker rather than Sleep: the repo bans wall-clock sleeps in tests, and a
// ticker keeps the cadence explicit and bounded.
func pollUntil(t *testing.T, budget time.Duration, describe string, probe func() bool) {
	t.Helper()
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	deadline := time.After(budget)
	for {
		if probe() {
			return
		}
		select {
		case <-deadline:
			t.Fatal(describe)
		case <-ticker.C:
		}
	}
}

func waitForHealth(t *testing.T, endpoint string) {
	t.Helper()
	pollUntil(t, integrationtest.StartTimeout(), "daemon never became healthy", func() bool {
		response, err := http.Get(endpoint + "/health") // #nosec G107 -- loopback test endpoint
		if err != nil {
			return false
		}
		_ = response.Body.Close()
		return response.StatusCode == http.StatusOK
	})
}

// waitForExtensionConnected is what proves the fake is admitted as the session.
// A probe is deliberately never adopted and never receives commands, so if this
// never flips, the client is talking but is not the extension.
func waitForExtensionConnected(t *testing.T, endpoint string) {
	t.Helper()
	pollUntil(t, integrationtest.ResponseTimeout(10*time.Second),
		"the daemon never reported the replayed extension as connected", func() bool {
			response, err := http.Get(endpoint + "/health") // #nosec G107 -- loopback test endpoint
			if err != nil {
				return false
			}
			body, _ := io.ReadAll(io.LimitReader(response.Body, 1<<20))
			_ = response.Body.Close()
			// extension_connected is nested under capture, not top level.
			// Reading the top level silently yields false forever, which reads
			// as "the browser never attached" rather than "wrong field".
			var health struct {
				Capture struct {
					ExtensionConnected bool `json:"extension_connected"`
				} `json:"capture"`
			}
			return json.Unmarshal(body, &health) == nil && health.Capture.ExtensionConnected
		})
}

func callTool(t *testing.T, endpoint, payload string) string {
	t.Helper()
	client := &http.Client{Timeout: integrationtest.ResponseTimeout(20 * time.Second)}
	response, err := client.Post(endpoint+"/mcp", "application/json", strings.NewReader(payload))
	if err != nil {
		t.Fatalf("tool call: %v", err)
	}
	defer func() { _ = response.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(response.Body, 8<<20))
	if err != nil {
		t.Fatalf("read tool response: %v", err)
	}
	return string(body)
}

// assertRecorded proves the other half: a transcript can be regenerated from a
// live run, so the fixture is reproducible rather than hand-maintained.
func assertRecorded(t *testing.T, path string) {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("the daemon recorded no transcript at %s: %v", path, err)
	}
	defer func() { _ = file.Close() }()

	records, err := synctranscript.Decode(file)
	if err != nil {
		t.Fatalf("recorded transcript did not decode: %v", err)
	}
	if len(records) == 0 {
		t.Fatal("the daemon wrote a transcript with no exchanges")
	}
	for _, record := range records {
		if record.Type == "dom" && strings.Contains(string(record.Result), "main.content") {
			return
		}
	}
	t.Errorf("the recorded transcript did not contain the exchange that just happened: %+v", records)
}

func truncate(body string) string {
	if len(body) <= 600 {
		return body
	}
	return body[:600] + "…"
}
