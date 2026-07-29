// websocket_test.go — Exercises the WebSocket test harness at frame level.

package testpages

import (
	"bufio"
	"encoding/json"
	"net"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/wsframe"
)

func TestWSEchoLoopDataControlAndFragments(t *testing.T) {
	t.Parallel()
	server, client := net.Pipe()
	done := make(chan struct{})
	go func() {
		defer close(done)
		rw := bufio.NewReadWriter(bufio.NewReader(server), bufio.NewWriter(server))
		wsEchoLoop(server, rw)
	}()
	defer client.Close()
	rw := bufio.NewReadWriter(bufio.NewReader(client), bufio.NewWriter(client))

	writeRawFrame(t, rw, false, 0x1, []byte("hel"))
	writeRawFrame(t, rw, true, 0x0, []byte("lo"))
	_, opcode, payload, err := wsframe.ReadFrame(rw)
	if err != nil {
		t.Fatal(err)
	}
	if opcode != 0x1 {
		t.Fatalf("text opcode = %d", opcode)
	}
	var envelope map[string]any
	if err := json.Unmarshal(payload, &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope["echo"] != "hello" || envelope["server"] != "kaboom-test-harness" {
		t.Fatalf("text envelope = %v", envelope)
	}

	writeRawFrame(t, rw, true, 0x9, []byte("ping"))
	_, opcode, payload, err = wsframe.ReadFrame(rw)
	if err != nil || opcode != 0xA || string(payload) != "ping" {
		t.Fatalf("pong = opcode %d payload %q err %v", opcode, payload, err)
	}

	writeRawFrame(t, rw, true, 0x2, []byte{1, 2, 3})
	_, opcode, payload, err = wsframe.ReadFrame(rw)
	if err != nil || opcode != 0x2 || string(payload) != string([]byte{1, 2, 3}) {
		t.Fatalf("binary echo = opcode %d payload %v err %v", opcode, payload, err)
	}

	writeRawFrame(t, rw, true, 0x8, nil)
	_, opcode, _, err = wsframe.ReadFrame(rw)
	if err != nil || opcode != 0x8 {
		t.Fatalf("close echo = opcode %d err %v", opcode, err)
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("echo loop did not stop after close")
	}
}

func TestWSEchoLoopStopsOnReadError(t *testing.T) {
	t.Parallel()
	server, client := net.Pipe()
	done := make(chan struct{})
	go func() {
		defer close(done)
		wsEchoLoop(server, bufio.NewReadWriter(bufio.NewReader(server), bufio.NewWriter(server)))
	}()
	_ = client.Close()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("echo loop did not stop after peer close")
	}
}

func writeRawFrame(t *testing.T, rw *bufio.ReadWriter, fin bool, opcode byte, payload []byte) {
	t.Helper()
	first := opcode
	if fin {
		first |= 0x80
	}
	if len(payload) >= 126 {
		t.Fatalf("test helper only supports short payloads")
	}
	frame := append([]byte{first, byte(len(payload))}, payload...)
	if _, err := rw.Write(frame); err != nil {
		t.Fatal(err)
	}
	if err := rw.Flush(); err != nil {
		t.Fatal(err)
	}
}
