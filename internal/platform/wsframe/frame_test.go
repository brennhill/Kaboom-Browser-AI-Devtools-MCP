package wsframe

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"strings"
	"testing"
)

func TestAcceptKeyKnownVector(t *testing.T) {
	t.Parallel()

	got := AcceptKey("dGhlIHNhbXBsZSBub25jZQ==")
	want := "s3pPLMBiTxaQ9kYGzzhZRbK+xOo="
	if got != want {
		t.Fatalf("AcceptKey mismatch: got %q want %q", got, want)
	}
}

func TestWriteFrameLengthEncoding(t *testing.T) {
	t.Parallel()

	t.Run("short payload", func(t *testing.T) {
		t.Parallel()

		var b bytes.Buffer
		rw := bufio.NewReadWriter(bufio.NewReader(&b), bufio.NewWriter(&b))
		if err := WriteFrame(rw, 0x1, []byte("hi")); err != nil {
			t.Fatalf("WriteFrame failed: %v", err)
		}
		got := b.Bytes()
		if len(got) != 4 {
			t.Fatalf("unexpected frame length: got %d", len(got))
		}
		if got[0] != 0x81 || got[1] != 0x02 || got[2] != 'h' || got[3] != 'i' {
			t.Fatalf("unexpected frame bytes: %v", got)
		}
	})

	t.Run("extended 16-bit payload", func(t *testing.T) {
		t.Parallel()

		payload := bytes.Repeat([]byte{'a'}, 126)
		var b bytes.Buffer
		rw := bufio.NewReadWriter(bufio.NewReader(&b), bufio.NewWriter(&b))
		if err := WriteFrame(rw, 0x2, payload); err != nil {
			t.Fatalf("WriteFrame failed: %v", err)
		}
		got := b.Bytes()
		if len(got) != 4+len(payload) {
			t.Fatalf("unexpected frame length: got %d want %d", len(got), 4+len(payload))
		}
		if got[0] != 0x82 || got[1] != 126 || got[2] != 0x00 || got[3] != 126 {
			t.Fatalf("unexpected frame header: %v", got[:4])
		}
	})

	t.Run("extended 64-bit payload", func(t *testing.T) {
		t.Parallel()

		payload := bytes.Repeat([]byte{'b'}, 65536)
		var b bytes.Buffer
		rw := bufio.NewReadWriter(bufio.NewReader(&b), bufio.NewWriter(&b))
		if err := WriteFrame(rw, 0x2, payload); err != nil {
			t.Fatalf("WriteFrame failed: %v", err)
		}
		got := b.Bytes()
		if len(got) != 10+len(payload) {
			t.Fatalf("unexpected frame length: got %d want %d", len(got), 10+len(payload))
		}
		if got[0] != 0x82 || got[1] != 127 {
			t.Fatalf("unexpected first header bytes: %v", got[:2])
		}
		if n := binary.BigEndian.Uint64(got[2:10]); n != uint64(len(payload)) {
			t.Fatalf("unexpected encoded payload length: got %d want %d", n, len(payload))
		}
	})
}

func TestReadFrame(t *testing.T) {
	t.Parallel()

	t.Run("reads unmasked short frame", func(t *testing.T) {
		t.Parallel()

		r := bytes.NewReader([]byte{0x81, 0x02, 'h', 'i'})
		fin, opcode, payload, err := ReadFrame(r)
		if err != nil {
			t.Fatalf("ReadFrame failed: %v", err)
		}
		if !fin {
			t.Fatal("expected FIN=true")
		}
		if opcode != 0x1 {
			t.Fatalf("unexpected opcode: got %d want %d", opcode, 0x1)
		}
		if string(payload) != "hi" {
			t.Fatalf("unexpected payload: got %q", string(payload))
		}
	})

	t.Run("reads masked short frame", func(t *testing.T) {
		t.Parallel()

		mask := [4]byte{0x01, 0x02, 0x03, 0x04}
		payload := []byte("hello")
		masked := make([]byte, len(payload))
		for i := range payload {
			masked[i] = payload[i] ^ mask[i%4]
		}

		frame := []byte{0x81, 0x80 | byte(len(payload))}
		frame = append(frame, mask[:]...)
		frame = append(frame, masked...)

		fin, opcode, gotPayload, err := ReadFrame(bytes.NewReader(frame))
		if err != nil {
			t.Fatalf("ReadFrame failed: %v", err)
		}
		if !fin {
			t.Fatal("expected FIN=true")
		}
		if opcode != 0x1 {
			t.Fatalf("unexpected opcode: got %d want %d", opcode, 0x1)
		}
		if string(gotPayload) != "hello" {
			t.Fatalf("unexpected payload: got %q", string(gotPayload))
		}
	})

	t.Run("rejects oversize payload", func(t *testing.T) {
		t.Parallel()

		tooLarge := uint64(MaxPayload + 1)
		frame := []byte{0x81, 127, 0, 0, 0, 0}
		frame = append(frame, byte(tooLarge>>24), byte(tooLarge>>16), byte(tooLarge>>8), byte(tooLarge))
		_, _, _, err := ReadFrame(bytes.NewReader(frame))
		if err == nil {
			t.Fatal("expected payload-size error")
		}
		if !strings.Contains(err.Error(), "exceeds limit") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}
