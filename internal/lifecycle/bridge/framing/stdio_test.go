// Purpose: Tests for bridge stdio transport framing and isolation.
// Docs: docs/features/feature/bridge-restart/index.md

// stdio_test.go — Tests for ReadStdioMessage.
package bridge

import (
	"bufio"
	"fmt"
	"strings"
	"testing"
)

const testMaxBodySize = 10 * 1024 * 1024

func frameMessage(payload string) string {
	return fmt.Sprintf("Content-Length: %d\r\nContent-Type: application/json\r\n\r\n%s", len(payload), payload)
}

func frameMessageContentTypeFirst(payload string) string {
	return fmt.Sprintf("Content-Type: application/json\r\nContent-Length: %d\r\n\r\n%s", len(payload), payload)
}
func TestReadStdioMessageWithMode_LineDelimitedJSON(t *testing.T) {
	input := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}` + "\n"
	r := bufio.NewReader(strings.NewReader(input))

	msg, framing, err := ReadStdioMessageWithMode(r, testMaxBodySize)
	if err != nil {
		t.Fatalf("ReadStdioMessageWithMode returned error: %v", err)
	}
	if framing != StdioFramingLine {
		t.Fatalf("framing = %v, want StdioFramingLine", framing)
	}
	if got, want := string(msg), `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`; got != want {
		t.Fatalf("message = %q, want %q", got, want)
	}
}

func TestReadStdioMessageWithMode_ContentLengthFramedJSON(t *testing.T) {
	payload := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05"}}`
	r := bufio.NewReader(strings.NewReader(frameMessage(payload)))

	msg, framing, err := ReadStdioMessageWithMode(r, testMaxBodySize)
	if err != nil {
		t.Fatalf("ReadStdioMessageWithMode returned error: %v", err)
	}
	if framing != StdioFramingContentLength {
		t.Fatalf("framing = %v, want StdioFramingContentLength", framing)
	}
	if got := string(msg); got != payload {
		t.Fatalf("message = %q, want %q", got, payload)
	}
}

func TestReadStdioMessageWithMode_ContentLengthFramedJSON_ContentTypeFirst(t *testing.T) {
	payload := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05"}}`
	r := bufio.NewReader(strings.NewReader(frameMessageContentTypeFirst(payload)))

	msg, framing, err := ReadStdioMessageWithMode(r, testMaxBodySize)
	if err != nil {
		t.Fatalf("ReadStdioMessageWithMode returned error: %v", err)
	}
	if framing != StdioFramingContentLength {
		t.Fatalf("framing = %v, want StdioFramingContentLength", framing)
	}
	if got := string(msg); got != payload {
		t.Fatalf("message = %q, want %q", got, payload)
	}
}
