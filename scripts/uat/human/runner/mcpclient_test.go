// mcpclient_test.go — Proves one call's reply is never recorded against another
// case, which is the only way this client can quietly corrupt a run.

package main

import (
	"bytes"
	"io"
	"strings"
	"testing"
	"time"
)

// blockingReader never yields, standing in for a server that stopped answering.
type blockingReader struct{ release chan struct{} }

func (b *blockingReader) Read(p []byte) (int, error) {
	<-b.release
	return 0, io.EOF
}

func TestARepliesToItsOwnIDNotTheNextLine(t *testing.T) {
	t.Parallel()
	// The server emits a notification and a progress message before the answer.
	// Taking the next line would record a notification as the tool's response and
	// a person would judge the wrong thing.
	server := strings.NewReader(strings.Join([]string{
		`{"jsonrpc":"2.0","method":"notifications/message","params":{"level":"info"}}`,
		`not json at all`,
		`{"jsonrpc":"2.0","id":1,"result":{"content":[{"type":"text","text":"the answer"}]}}`,
	}, "\n") + "\n")
	var sent bytes.Buffer
	mcpSession := newClient(server, &sent, nil)

	caseRecord, err := mcpSession.call("observe", map[string]any{"what": "page"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(caseRecord), "the answer") {
		t.Errorf("result = %s, want the reply matching the request id", caseRecord)
	}
	if !strings.Contains(sent.String(), `"method":"tools/call"`) {
		t.Errorf("the request was not sent as a tools/call: %s", sent.String())
	}
}

func TestAServerErrorIsReportedNotReturnedAsAResult(t *testing.T) {
	t.Parallel()
	server := strings.NewReader(`{"jsonrpc":"2.0","id":1,"error":{"code":-32602,"message":"unknown mode"}}` + "\n")
	mcpSession := newClient(server, &bytes.Buffer{}, nil)

	caseRecord, err := mcpSession.call("observe", map[string]any{"what": "nonsense"})
	if err == nil {
		t.Fatalf("an error envelope came back as a result: %s", caseRecord)
	}
	if !strings.Contains(err.Error(), "unknown mode") {
		t.Errorf("error = %v, want the server's own message", err)
	}
}

func TestAClosedServerIsAnErrorNotAnEmptyResult(t *testing.T) {
	t.Parallel()
	// An empty result forwarded as success is exactly the failure this rig
	// exists to end: the tester would be asked to judge nothing at all.
	mcpSession := newClient(strings.NewReader(""), &bytes.Buffer{}, nil)

	if caseRecord, err := mcpSession.call("observe", map[string]any{"what": "page"}); err == nil {
		t.Fatalf("a dead server produced a result: %s", caseRecord)
	}
}

func TestATimedOutCallPoisonsTheSessionInsteadOfDesyncing(t *testing.T) {
	t.Parallel()
	blocked := &blockingReader{release: make(chan struct{})}
	defer close(blocked.release)
	mcpSession := newClient(blocked, &bytes.Buffer{}, nil)
	mcpSession.deadline = 20 * time.Millisecond

	if _, err := mcpSession.call("observe", map[string]any{"what": "page"}); err == nil {
		t.Fatal("a hung server returned success")
	}
	// The abandoned reply is still on the stream. Making another call would read
	// it as the new call's answer and record it against the wrong case.
	_, err := mcpSession.call("observe", map[string]any{"what": "logs"})
	if err == nil {
		t.Fatal("a second call went out on a stream carrying an abandoned reply")
	}
	if !strings.Contains(err.Error(), "timed out") {
		t.Errorf("error = %v; it must say why the session stopped", err)
	}
}

func TestInitializeHappensBeforeAnyToolCall(t *testing.T) {
	t.Parallel()
	server := strings.NewReader(`{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2024-11-05"}}` + "\n")
	var sent bytes.Buffer
	mcpSession := newClient(server, &sent, nil)

	if err := mcpSession.initialize(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sent.String(), `"method":"initialize"`) {
		t.Errorf("no handshake was sent: %s", sent.String())
	}
	if !strings.Contains(sent.String(), "kaboom-human-uat") {
		t.Error("the server cannot tell which client is driving it")
	}
}
