// Purpose: Implements the /tests/ws WebSocket handshake and frame-level echo harness.
// Why: Keeps protocol parsing/echo logic isolated from HTTP page routing for easier testing and maintenance.
// Docs: docs/features/feature/self-testing/index.md

package testpages

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/wsframe"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/util"
)

// wsIdleTimeout is the per-read deadline, reset after every successful frame.
// This is an idle timeout — an active connection that keeps sending frames will
// never be cut. An idle connection is closed after this duration.
const wsIdleTimeout = 60 * time.Second

// handleTestHarnessWS upgrades a GET /tests/ws request to a WebSocket echo
// server implemented with zero external dependencies (net/http hijacking).
// Note: CORS headers set by corsMiddleware are buffered in http.ResponseWriter
// but are not included in the 101 handshake written directly to the hijacked
// connection. This is intentional — WebSocket upgrade bypasses HTTP CORS.
func HandlerWS(w http.ResponseWriter, r *http.Request) {
	key := r.Header.Get("Sec-WebSocket-Key")
	if key == "" || strings.ToLower(r.Header.Get("Upgrade")) != "websocket" {
		util.JSONResponse(w, http.StatusBadRequest, map[string]string{
			"error":   "bad_request",
			"message": "websocket upgrade required",
		})
		return
	}

	hj, ok := w.(http.Hijacker)
	if !ok {
		util.JSONResponse(w, http.StatusInternalServerError, map[string]string{
			"error":   "internal_error",
			"message": "server does not support hijacking",
		})
		return
	}

	conn, bufrw, err := hj.Hijack()
	if err != nil {
		util.JSONResponse(w, http.StatusInternalServerError, map[string]string{
			"error":   "internal_error",
			"message": err.Error(),
		})
		return
	}
	defer conn.Close()

	// Send the 101 handshake.
	accept := wsframe.AcceptKey(key)
	handshake := fmt.Sprintf(
		"HTTP/1.1 101 Switching Protocols\r\n"+
			"Upgrade: websocket\r\n"+
			"Connection: Upgrade\r\n"+
			"Sec-WebSocket-Accept: %s\r\n\r\n",
		accept,
	)
	if _, err := bufrw.WriteString(handshake); err != nil {
		return
	}
	if err := bufrw.Flush(); err != nil {
		return
	}

	wsEchoLoop(conn, bufrw)
}

// wsEchoLoop reads WebSocket frames and echoes data frames.
// Text frames are echoed as a JSON envelope; binary frames are echoed as-is.
// The per-read deadline is refreshed after every successful frame, implementing
// an idle timeout: active connections are never cut; idle ones time out after
// wsIdleTimeout.
func wsEchoLoop(conn net.Conn, rw *bufio.ReadWriter) {
	// fragOpcode and fragBuf accumulate fragments for reassembly (RFC 6455 §5.4).
	var fragOpcode byte
	var fragBuf []byte

	for {
		// Reset the idle deadline before each read. A connection that sends
		// at least one frame per wsIdleTimeout period is never disconnected.
		_ = conn.SetReadDeadline(time.Now().Add(wsIdleTimeout))

		fin, opcode, payload, err := wsframe.ReadFrame(rw)
		if err != nil {
			return
		}

		// Control frames (0x8–0xF) must not be fragmented (RFC 6455 §5.5) and
		// are dispatched immediately regardless of any in-progress fragment sequence.
		if opcode >= 0x8 {
			switch opcode {
			case 0x8: // Close — echo close frame and exit.
				_ = wsframe.WriteFrame(rw, 0x8, nil)
				return
			case 0x9: // Ping → Pong
				if err := wsframe.WriteFrame(rw, 0xA, payload); err != nil {
					return
				}
			}
			continue
		}

		// Data frame fragmentation reassembly (RFC 6455 §5.4).
		if opcode != 0x0 {
			// Non-zero opcode = first (or only) frame of a data message.
			if !fin {
				// First fragment: record opcode and begin accumulation.
				fragOpcode = opcode
				fragBuf = append(fragBuf[:0], payload...)
				continue
			}
			// FIN=1 with non-continuation opcode = single unfragmented message.
		} else {
			// Continuation frame (opcode 0x0): append to accumulation buffer.
			fragBuf = append(fragBuf, payload...)
			if !fin {
				continue // More fragments incoming.
			}
			// Final fragment — reassemble the full message.
			opcode = fragOpcode
			payload = fragBuf
			fragBuf = fragBuf[:0]
		}

		// Dispatch complete (possibly reassembled) message.
		switch opcode {
		case 0x1: // Text → echo as JSON envelope
			reply := mcp.SafeMarshal(map[string]any{
				"type":   "echo",
				"echo":   string(payload),
				"server": "kaboom-test-harness",
				"ts":     time.Now().UnixMilli(),
			}, "{}")
			if err := wsframe.WriteFrame(rw, 0x1, reply); err != nil {
				return
			}
		case 0x2: // Binary → echo binary
			if err := wsframe.WriteFrame(rw, 0x2, payload); err != nil {
				return
			}
		}
	}
}
