// frame.go — RFC 6455 WebSocket frame codec (read, write, handshake accept key).
// Why: This is shared wire infrastructure, not a self-test fixture. It was named
// testpages_websocket_codec.go, but the production terminal relay consumes the same
// three functions through terminal.Deps, so it lives in a package with exactly one
// job: turning bytes into frames and back.
// Docs: docs/features/feature/self-testing/index.md

package wsframe

import (
	"bufio"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"io"
)

// MaxPayload caps incoming frame payloads to prevent DoS via oversized allocation.
const MaxPayload = 1 << 20 // 1 MiB

// ReadFrame reads one complete WebSocket frame, handling masking.
// Returns the FIN bit, opcode, unmasked payload, and any I/O error.
// Payloads larger than MaxPayload are rejected to prevent DoS.
func ReadFrame(r io.Reader) (fin bool, opcode byte, payload []byte, err error) {
	header := make([]byte, 2)
	if _, err = io.ReadFull(r, header); err != nil {
		return
	}
	fin = header[0]&0x80 != 0
	opcode = header[0] & 0x0F
	masked := header[1]&0x80 != 0
	length := uint64(header[1] & 0x7F)

	switch length {
	case 126:
		ext := make([]byte, 2)
		if _, err = io.ReadFull(r, ext); err != nil {
			return
		}
		length = uint64(binary.BigEndian.Uint16(ext))
	case 127:
		ext := make([]byte, 8)
		if _, err = io.ReadFull(r, ext); err != nil {
			return
		}
		length = binary.BigEndian.Uint64(ext)
	}

	if length > MaxPayload {
		err = fmt.Errorf("ws: frame payload %d bytes exceeds limit %d", length, uint64(MaxPayload))
		return
	}

	var mask [4]byte
	if masked {
		if _, err = io.ReadFull(r, mask[:]); err != nil {
			return
		}
	}

	payload = make([]byte, length)
	if _, err = io.ReadFull(r, payload); err != nil {
		return
	}
	if masked {
		for i := range payload {
			payload[i] ^= mask[i%4]
		}
	}
	return
}

// WriteFrame writes one unmasked WebSocket frame (FIN=1, server→client).
// Payload length is encoded per RFC 6455 §5.2, including the full 8-byte
// big-endian form for payloads ≥ 65536 bytes.
func WriteFrame(w *bufio.ReadWriter, opcode byte, payload []byte) error {
	length := uint64(len(payload))
	header := []byte{0x80 | opcode}
	switch {
	case length < 126:
		header = append(header, byte(length))
	case length < 65536:
		var encoded [2]byte
		binary.BigEndian.PutUint16(encoded[:], uint16(length))
		header = append(header, 126)
		header = append(header, encoded[:]...)
	default:
		var encoded [8]byte
		binary.BigEndian.PutUint64(encoded[:], length)
		header = append(header, 127)
		header = append(header, encoded[:]...)
	}
	if _, err := w.Write(append(header, payload...)); err != nil {
		return err
	}
	return w.Flush()
}

// AcceptKey computes the Sec-WebSocket-Accept value per RFC 6455.
func AcceptKey(key string) string {
	const guid = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"
	h := sha1.New()
	h.Write([]byte(key + guid))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}
