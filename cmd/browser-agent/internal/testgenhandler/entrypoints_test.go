// entrypoints_test.go — covers the three generate(test_*) entry points in this package.
// Why: these were previously exercised only through package main's generate-tool
// dispatch tests. Go measures coverage per package, so once the handler moved here
// those tests stopped counting and every entry point read 0.0% despite still being
// driven end to end. These tests assert the same contracts at the package boundary.

package testgenhandler

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

func req() mcp.JSONRPCRequest { return mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1} }

func toolResult(t *testing.T, resp mcp.JSONRPCResponse) mcp.MCPToolResult {
	t.Helper()
	var r mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &r); err != nil {
		t.Fatalf("response is not an MCPToolResult: %v; raw=%s", err, string(resp.Result))
	}
	return r
}

func TestHandleGenerateTestFromContext(t *testing.T) {
	t.Parallel()

	t.Run("malformed JSON fails", func(t *testing.T) {
		t.Parallel()
		env := newTestEnv()
		if !toolResult(t, env.h.HandleGenerateTestFromContext(req(), json.RawMessage(`{bad`))).IsError {
			t.Fatal("expected an error result for malformed JSON")
		}
	})

	t.Run("missing context fails", func(t *testing.T) {
		t.Parallel()
		env := newTestEnv()
		r := toolResult(t, env.h.HandleGenerateTestFromContext(req(), json.RawMessage(`{}`)))
		if !r.IsError {
			t.Fatal("expected an error result when 'context' is missing")
		}
		if !strings.Contains(r.Content[0].Text, "context") {
			t.Fatalf("error should name the missing param: %s", r.Content[0].Text)
		}
	})

	t.Run("unknown context fails", func(t *testing.T) {
		t.Parallel()
		env := newTestEnv()
		r := toolResult(t, env.h.HandleGenerateTestFromContext(req(), json.RawMessage(`{"context":"nope"}`)))
		if !r.IsError {
			t.Fatal("expected an error result for an unknown context value")
		}
	})

	t.Run("error context routes to the error generator", func(t *testing.T) {
		t.Parallel()
		// context="error" must reach generateTestFromError, which reads the log
		// buffer (not the action buffer) to find the error to reproduce. With no
		// error-level entry it reports no error context; with one, it generates.
		const errorTS = "2026-01-01T00:00:00Z"
		errorMillis := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli()

		env := newTestEnv()
		// The action must fall inside the generator's 5s window around the error.
		env.cap.AddEnhancedActionsForTest([]capture.EnhancedAction{
			{Type: "click", URL: "https://example.com", Timestamp: errorMillis},
		})

		noErrors := toolResult(t, env.h.HandleGenerateTestFromContext(req(), json.RawMessage(`{"context":"error"}`)))
		if !noErrors.IsError {
			t.Fatal("expected an error result when no error-level log entry exists")
		}

		env.deps.entries = []mcp.LogEntry{
			{"level": "error", "message": "TypeError: undefined is not a function", "ts": errorTS},
		}
		withError := toolResult(t, env.h.HandleGenerateTestFromContext(req(), json.RawMessage(`{"context":"error"}`)))
		if withError.IsError {
			t.Fatalf("unexpected error result once an error entry exists: %s", withError.Content[0].Text)
		}
		test := decodedTest(t, withError)
		// The reproduction script is built around the captured error message.
		if content, _ := test["content"].(string); !strings.Contains(content, "undefined is not a function") {
			t.Fatalf("generated test should reproduce the captured error; got %q", content)
		}
	})

	t.Run("interaction context generates a test and defaults the framework", func(t *testing.T) {
		t.Parallel()
		env := newTestEnv()
		env.cap.AddEnhancedActionsForTest([]capture.EnhancedAction{
			{Type: "click", URL: "https://example.com"},
		})

		resp := env.h.HandleGenerateTestFromContext(req(), json.RawMessage(`{"context":"interaction"}`))
		r := toolResult(t, resp)
		if r.IsError {
			t.Fatalf("unexpected error result: %s", r.Content[0].Text)
		}

		// Assert on the decoded test object, not on the response text: the
		// generated script body literally contains the word "playwright", so a
		// substring check here passes even when the default is removed.
		test := decodedTest(t, r)
		if test["framework"] != "playwright" {
			t.Fatalf("framework = %v, want playwright (the handler's default for an omitted framework)", test["framework"])
		}
	})
}

func TestHandleGenerateTestHeal(t *testing.T) {
	t.Parallel()

	t.Run("malformed JSON fails", func(t *testing.T) {
		t.Parallel()
		if !toolResult(t, newTestEnv().h.HandleGenerateTestHeal(req(), json.RawMessage(`{bad`))).IsError {
			t.Fatal("expected an error result for malformed JSON")
		}
	})

	t.Run("missing action fails", func(t *testing.T) {
		t.Parallel()
		r := toolResult(t, newTestEnv().h.HandleGenerateTestHeal(req(), json.RawMessage(`{}`)))
		if !r.IsError {
			t.Fatal("expected an error result when 'action' is missing")
		}
		if !strings.Contains(r.Content[0].Text, "action") {
			t.Fatalf("error should name the missing param: %s", r.Content[0].Text)
		}
	})

	t.Run("unknown action fails", func(t *testing.T) {
		t.Parallel()
		if !toolResult(t, newTestEnv().h.HandleGenerateTestHeal(req(), json.RawMessage(`{"action":"nope"}`))).IsError {
			t.Fatal("expected an error result for an unknown action value")
		}
	})

}

// Deliberately not parallel, and deliberately top level: heal resolves test_file
// against the process working directory and rejects anything that escapes it, so
// this case needs t.Chdir, which the testing package forbids under t.Parallel.
func TestHandleGenerateTestHealAnalyzeReadsTheFile(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	if err := os.WriteFile(filepath.Join(dir, "login.spec.ts"), []byte("await page.click('#login-btn')\n"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	args, _ := json.Marshal(map[string]any{"action": "analyze", "test_file": "login.spec.ts"})

	r := toolResult(t, newTestEnv().h.HandleGenerateTestHeal(req(), args))
	if r.IsError {
		t.Fatalf("unexpected error result: %s", r.Content[0].Text)
	}
}

func TestHandleGenerateTestClassify(t *testing.T) {
	t.Parallel()

	t.Run("malformed JSON fails", func(t *testing.T) {
		t.Parallel()
		if !toolResult(t, newTestEnv().h.HandleGenerateTestClassify(req(), json.RawMessage(`{bad`))).IsError {
			t.Fatal("expected an error result for malformed JSON")
		}
	})

	t.Run("missing action fails", func(t *testing.T) {
		t.Parallel()
		r := toolResult(t, newTestEnv().h.HandleGenerateTestClassify(req(), json.RawMessage(`{}`)))
		if !r.IsError {
			t.Fatal("expected an error result when 'action' is missing")
		}
		if !strings.Contains(r.Content[0].Text, "action") {
			t.Fatalf("error should name the missing param: %s", r.Content[0].Text)
		}
	})

	t.Run("unknown action fails", func(t *testing.T) {
		t.Parallel()
		if !toolResult(t, newTestEnv().h.HandleGenerateTestClassify(req(), json.RawMessage(`{"action":"nope"}`))).IsError {
			t.Fatal("expected an error result for an unknown action value")
		}
	})

	t.Run("failure action classifies", func(t *testing.T) {
		t.Parallel()
		args := json.RawMessage(`{"action":"failure","failure":{"test_name":"login test","error":"Timeout waiting for selector \"#login-btn\""}}`)
		r := toolResult(t, newTestEnv().h.HandleGenerateTestClassify(req(), args))
		if r.IsError {
			t.Fatalf("unexpected error result: %s", r.Content[0].Text)
		}
	})
}

func TestTestGenErrorToResponseMapsKnownCodes(t *testing.T) {
	t.Parallel()

	if len(testGenErrorMappings) == 0 {
		t.Fatal("no error mappings registered — this test would vacuously pass")
	}

	// A mapped error must produce that mapping's curated message and hint. Asserting
	// on the code alone is useless: the fallback branch embeds err.Error(), so the
	// code appears in the payload either way.
	for _, m := range testGenErrorMappings {
		resp := testGenErrorToResponse(1, errors.New("wrapped: "+m.code))
		got := string(resp.Result)
		if !strings.Contains(got, m.message) {
			t.Fatalf("error %q should carry its mapped message %q; got %s", m.code, m.message, got)
		}
		if m.hint != "" && !strings.Contains(got, m.hint) {
			t.Fatalf("error %q should carry its mapped hint %q; got %s", m.code, m.hint, got)
		}
	}

	// An unmapped error falls through to the generic internal error.
	resp := testGenErrorToResponse(1, errors.New("something else entirely"))
	got := string(resp.Result)
	if !strings.Contains(got, "Failed to generate test") {
		t.Fatalf("unmapped error should use the generic fallback message; got %s", got)
	}
	if !strings.Contains(got, "something else entirely") {
		t.Fatalf("fallback should surface the original error text; got %s", got)
	}
}

// decodedTest pulls the "test" object out of a successful generate response.
func decodedTest(t *testing.T, r mcp.MCPToolResult) map[string]any {
	t.Helper()
	if len(r.Content) == 0 {
		t.Fatal("response has no content blocks")
	}
	text := r.Content[0].Text
	for i, ch := range text {
		if ch == '{' || ch == '[' {
			text = text[i:]
			break
		}
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(text), &payload); err != nil {
		t.Fatalf("response payload is not JSON: %v; text=%q", err, text)
	}
	test, ok := payload["test"].(map[string]any)
	if !ok {
		t.Fatalf("payload has no 'test' object: %#v", payload)
	}
	return test
}
