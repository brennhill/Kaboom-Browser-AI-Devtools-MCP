// test_support_test.go — Shared deterministic WebSocket transport test collaborators.

package wstransport

import (
	"bufio"
	"encoding/json"
	"io"
	"net/http"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/wsframe"
)

func testDeps() Deps {
	return Deps{
		JSONResponse: func(w http.ResponseWriter, status int, data any) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(status)
			_ = json.NewEncoder(w).Encode(data)
		},
		Stderrf:      func(string, ...any) {},
		WSReadFrame:  wsframe.ReadFrame,
		WSWriteFrame: wsframe.WriteFrame,
		WSAcceptKey:  wsframe.AcceptKey,
	}
}

func testWSWriteFrame(writer *bufio.ReadWriter, opcode byte, payload []byte) error {
	return wsframe.WriteFrame(writer, opcode, payload)
}

func testWSReadFrame(reader io.Reader) (bool, byte, []byte, error) {
	return wsframe.ReadFrame(reader)
}
