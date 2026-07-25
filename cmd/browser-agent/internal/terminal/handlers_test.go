// handlers_test.go -- Tests for terminal HTTP handlers: auth, session lifecycle, static serving.

package terminal

import (
	"bufio"
	"bytes"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/pty"
)

// testDeps returns a Deps instance for testing with real WS codec functions.
func testDeps() Deps {
	return Deps{
		JSONResponse:   testJSONResponse,
		CORSMiddleware: func(next http.HandlerFunc) http.HandlerFunc { return next },
		Stderrf:        func(format string, args ...any) {},
		MaxPostBody:    10 * 1024 * 1024,
		WSReadFrame:    testWSReadFrame,
		WSWriteFrame:   testWSWriteFrame,
		WSAcceptKey:    testWSAcceptKey,
	}
}

func testJSONResponse(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func testWSReadFrame(r io.Reader) (fin bool, opcode byte, payload []byte, err error) {
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

func testWSWriteFrame(w *bufio.ReadWriter, opcode byte, payload []byte) error {
	length := uint64(len(payload))
	header := []byte{0x80 | opcode}
	switch {
	case length < 126:
		header = append(header, byte(length))
	case length < 65536:
		header = append(header, 126,
			byte(length>>8), byte(length))
	default:
		header = append(header, 127,
			byte(length>>56), byte(length>>48), byte(length>>40), byte(length>>32),
			byte(length>>24), byte(length>>16), byte(length>>8), byte(length))
	}
	if _, err := w.Write(append(header, payload...)); err != nil {
		return err
	}
	return w.Flush()
}

func testWSAcceptKey(key string) string {
	const guid = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"
	h := sha1.New()
	h.Write([]byte(key + guid))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

func TestNewFrameWriter_SerializesConcurrentWrites(t *testing.T) {
	t.Parallel()

	const frameCount = 64

	var wire bytes.Buffer
	rw := bufio.NewReadWriter(bufio.NewReader(&wire), bufio.NewWriter(&wire))
	deps := testDeps()
	writeFrame := NewFrameWriter(nil, rw, deps)

	var wg sync.WaitGroup
	for i := 0; i < frameCount; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			payload := []byte(fmt.Sprintf("msg-%02d", i))
			if err := writeFrame(0x1, payload); err != nil {
				t.Errorf("writeFrame(%d): %v", i, err)
			}
		}(i)
	}
	wg.Wait()

	seen := make(map[string]bool, frameCount)
	reader := bytes.NewReader(wire.Bytes())
	for {
		_, _, payload, err := testWSReadFrame(reader)
		if err != nil {
			if err == io.EOF {
				break
			}
			t.Fatalf("wsReadFrame failed: %v", err)
		}
		seen[string(payload)] = true
	}

	if len(seen) != frameCount {
		t.Fatalf("decoded frames = %d, want %d", len(seen), frameCount)
	}
	for i := 0; i < frameCount; i++ {
		expected := fmt.Sprintf("msg-%02d", i)
		if !seen[expected] {
			t.Fatalf("missing payload %q", expected)
		}
	}
}

func TestHandleTerminalPage_ServesHTML(t *testing.T) {
	deps := testDeps()
	req := httptest.NewRequest("GET", "/terminal", nil)
	rec := httptest.NewRecorder()
	HandleTerminalPage(rec, req, deps)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	ct := rec.Header().Get("Content-Type")
	if ct != "text/html; charset=utf-8" {
		t.Fatalf("expected text/html, got %s", ct)
	}
	body := rec.Body.String()
	if len(body) < 100 {
		t.Fatal("expected substantial HTML content")
	}
}

func TestHandleTerminalPage_RejectsNonGET(t *testing.T) {
	deps := testDeps()
	req := httptest.NewRequest("POST", "/terminal", nil)
	rec := httptest.NewRecorder()
	HandleTerminalPage(rec, req, deps)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rec.Code)
	}
}

func TestHandleTerminalStart_CreatesSession(t *testing.T) {
	mgr := pty.NewManager()
	defer mgr.StopAll()
	deps := testDeps()

	body, _ := json.Marshal(map[string]any{
		"cmd":  "/bin/sh",
		"args": []string{"-c", "exec cat"},
	})
	req := httptest.NewRequest("POST", "/terminal/start", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	relays := NewMap()
	HandleTerminalStart(rec, req, deps, nil, mgr, nil, relays)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["session_id"] != "default" {
		t.Fatalf("expected session_id 'default', got: %v", resp["session_id"])
	}
	if resp["token"] == nil || resp["token"] == "" {
		t.Fatal("expected non-empty token")
	}
	if resp["pid"] == nil {
		t.Fatal("expected pid in response")
	}
}

func TestHandleTerminalStart_DuplicateReturnsConflict(t *testing.T) {
	mgr := pty.NewManager()
	defer mgr.StopAll()
	deps := testDeps()

	body, _ := json.Marshal(map[string]any{
		"cmd":  "/bin/sh",
		"args": []string{"-c", "exec cat"},
	})

	relays := NewMap()

	// First start.
	req := httptest.NewRequest("POST", "/terminal/start", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	HandleTerminalStart(rec, req, deps, nil, mgr, nil, relays)
	if rec.Code != http.StatusOK {
		t.Fatalf("first start: expected 200, got %d", rec.Code)
	}

	// Second start with same ID.
	req = httptest.NewRequest("POST", "/terminal/start", bytes.NewReader(body))
	rec = httptest.NewRecorder()
	HandleTerminalStart(rec, req, deps, nil, mgr, nil, relays)
	if rec.Code != http.StatusConflict {
		t.Fatalf("duplicate start: expected 409, got %d", rec.Code)
	}
}

// A real start failure (here: a non-existent working directory) must surface as a
// distinct 4xx error, NOT the 409-with-token "session exists" reconnect — otherwise
// the client silently reconnects to the old cwd and the user sees no reason for the
// failure. Regression for the "terminal failed a few times, no visible reason" bug.
func TestHandleTerminalStart_BadDirSurfacesDistinctError(t *testing.T) {
	mgr := pty.NewManager()
	defer mgr.StopAll()
	deps := testDeps()

	body, _ := json.Marshal(map[string]any{
		"cmd":  "/bin/sh",
		"args": []string{"-c", "exec cat"},
		"dir":  "/no/such/kaboom-dir-xyz-does-not-exist",
	})
	req := httptest.NewRequest("POST", "/terminal/start", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	HandleTerminalStart(rec, req, deps, nil, mgr, nil, NewMap())

	if rec.Code == http.StatusConflict {
		t.Fatalf("bad cwd must NOT return 409 (silent reconnect); body: %s", rec.Body.String())
	}
	if rec.Code < 400 || rec.Code >= 500 {
		t.Fatalf("bad cwd should surface a 4xx error, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	_ = json.NewDecoder(rec.Body).Decode(&resp)
	if tok, ok := resp["token"]; ok && tok != nil && tok != "" {
		t.Fatalf("a real start failure must not hand back a reconnect token, got: %v", tok)
	}
	if msg, _ := resp["error"].(string); msg == "" {
		t.Fatal("expected a non-empty error message surfaced to the client")
	}
}

func TestHandleTerminalStart_DefaultsToShell(t *testing.T) {
	mgr := pty.NewManager()
	defer mgr.StopAll()
	deps := testDeps()

	body, _ := json.Marshal(map[string]any{})
	req := httptest.NewRequest("POST", "/terminal/start", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	relays := NewMap()
	HandleTerminalStart(rec, req, deps, nil, mgr, nil, relays)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleTerminalStop_DestroysSession(t *testing.T) {
	mgr := pty.NewManager()
	defer mgr.StopAll()
	deps := testDeps()

	// Start a session first.
	startBody, _ := json.Marshal(map[string]any{
		"cmd":  "/bin/sh",
		"args": []string{"-c", "exec cat"},
	})
	relays := NewMap()
	req := httptest.NewRequest("POST", "/terminal/start", bytes.NewReader(startBody))
	rec := httptest.NewRecorder()
	HandleTerminalStart(rec, req, deps, nil, mgr, nil, relays)

	// Stop it.
	stopBody, _ := json.Marshal(map[string]any{"id": "default"})
	req = httptest.NewRequest("POST", "/terminal/stop", bytes.NewReader(stopBody))
	rec = httptest.NewRecorder()
	HandleTerminalStop(rec, req, deps, mgr, relays)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// Verify session is gone.
	if mgr.Count() != 0 {
		t.Fatal("expected 0 sessions after stop")
	}
}

func TestHandleTerminalStop_NotFound(t *testing.T) {
	mgr := pty.NewManager()
	deps := testDeps()

	body, _ := json.Marshal(map[string]any{"id": "nonexistent"})
	req := httptest.NewRequest("POST", "/terminal/stop", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	relays := NewMap()
	HandleTerminalStop(rec, req, deps, mgr, relays)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestHandleTerminalConfig_ListsSessions(t *testing.T) {
	mgr := pty.NewManager()
	defer mgr.StopAll()
	deps := testDeps()

	// Start a session.
	_, err := mgr.Start(pty.StartConfig{
		ID:   "test",
		Cmd:  "/bin/sh",
		Args: []string{"-c", "exec cat"},
	})
	if err != nil {
		t.Fatalf("start: %v", err)
	}

	req := httptest.NewRequest("GET", "/terminal/config", nil)
	rec := httptest.NewRecorder()
	relays := NewMap()
	HandleTerminalConfig(rec, req, deps, mgr, relays)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	count := resp["count"].(float64)
	if count != 1 {
		t.Fatalf("expected count 1, got %v", count)
	}

	// Validate the new rich session objects.
	sessions, ok := resp["sessions"].([]any)
	if !ok {
		t.Fatalf("expected sessions array, got %T", resp["sessions"])
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	sess := sessions[0].(map[string]any)
	if sess["id"] != "test" {
		t.Fatalf("expected id 'test', got %v", sess["id"])
	}
	if sess["alive"] != true {
		t.Fatalf("expected alive=true, got %v", sess["alive"])
	}
	if sess["pid"] == nil {
		t.Fatal("expected pid in session info")
	}
	if _, hasAlt := sess["alt_screen"]; !hasAlt {
		t.Fatal("expected alt_screen field in session info")
	}
}

func TestHandleTerminalValidate_ValidToken(t *testing.T) {
	mgr := pty.NewManager()
	defer mgr.StopAll()
	deps := testDeps()

	result, err := mgr.Start(pty.StartConfig{
		Cmd:  "/bin/sh",
		Args: []string{"-c", "exec cat"},
	})
	if err != nil {
		t.Fatalf("start: %v", err)
	}

	req := httptest.NewRequest("GET", "/terminal/validate?token="+result.Token, nil)
	rec := httptest.NewRecorder()
	HandleTerminalValidate(rec, req, deps, mgr)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var resp map[string]bool
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !resp["valid"] {
		t.Fatal("expected valid=true for live session")
	}
}

func TestHandleTerminalValidate_InvalidToken(t *testing.T) {
	mgr := pty.NewManager()
	deps := testDeps()

	req := httptest.NewRequest("GET", "/terminal/validate?token=bogus", nil)
	rec := httptest.NewRecorder()
	HandleTerminalValidate(rec, req, deps, mgr)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var resp map[string]bool
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["valid"] {
		t.Fatal("expected valid=false for bogus token")
	}
}

func TestHandleTerminalValidate_EmptyToken(t *testing.T) {
	mgr := pty.NewManager()
	deps := testDeps()

	req := httptest.NewRequest("GET", "/terminal/validate", nil)
	rec := httptest.NewRecorder()
	HandleTerminalValidate(rec, req, deps, mgr)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var resp map[string]bool
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["valid"] {
		t.Fatal("expected valid=false for empty token")
	}
}

func TestHandleTerminalValidate_StaleToken(t *testing.T) {
	mgr := pty.NewManager()
	deps := testDeps()

	// Start and immediately stop to create a stale token.
	result, err := mgr.Start(pty.StartConfig{
		Cmd:  "/bin/sh",
		Args: []string{"-c", "exec cat"},
	})
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	token := result.Token
	if err := mgr.Stop("default"); err != nil {
		t.Fatalf("stop: %v", err)
	}

	req := httptest.NewRequest("GET", "/terminal/validate?token="+token, nil)
	rec := httptest.NewRecorder()
	HandleTerminalValidate(rec, req, deps, mgr)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var resp map[string]bool
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["valid"] {
		t.Fatal("expected valid=false for stale token (session stopped)")
	}
}

func TestHandleTerminalWS_MissingToken(t *testing.T) {
	mgr := pty.NewManager()
	deps := testDeps()

	req := httptest.NewRequest("GET", "/terminal/ws", nil)
	rec := httptest.NewRecorder()
	relays := NewMap()
	HandleTerminalWS(rec, req, deps, mgr, relays)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestHandleTerminalWS_InvalidToken(t *testing.T) {
	mgr := pty.NewManager()
	deps := testDeps()

	req := httptest.NewRequest("GET", "/terminal/ws?token=bogus", nil)
	rec := httptest.NewRecorder()
	relays := NewMap()
	HandleTerminalWS(rec, req, deps, mgr, relays)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestHandleTerminalWS_NoUpgradeHeader(t *testing.T) {
	mgr := pty.NewManager()
	defer mgr.StopAll()
	deps := testDeps()

	// Start a session to get a valid token.
	result, err := mgr.Start(pty.StartConfig{
		Cmd:  "/bin/sh",
		Args: []string{"-c", "exec cat"},
	})
	if err != nil {
		t.Fatalf("start: %v", err)
	}

	req := httptest.NewRequest("GET", "/terminal/ws?token="+result.Token, nil)
	rec := httptest.NewRecorder()
	relays := NewMap()
	HandleTerminalWS(rec, req, deps, mgr, relays)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestHandleTerminalUpload_Success(t *testing.T) {
	mgr := pty.NewManager()
	defer mgr.StopAll()
	deps := testDeps()

	_, err := mgr.Start(pty.StartConfig{
		ID:   "upload-test",
		Cmd:  "/bin/sh",
		Args: []string{"-c", "exec cat"},
	})
	if err != nil {
		t.Fatalf("start: %v", err)
	}

	sess, _ := mgr.Get("upload-test")
	relays := NewMap()
	relays.GetOrCreate("upload-test", sess, t.TempDir())

	imgData := bytes.Repeat([]byte{0xFF, 0xD8, 0xFF}, 10)
	req := httptest.NewRequest("POST", "/terminal/upload?session_id=upload-test&filename=test.png", bytes.NewReader(imgData))
	req.Header.Set("Content-Type", "image/png")
	rec := httptest.NewRecorder()
	HandleTerminalUpload(rec, req, deps, mgr, relays)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["path"] == nil || resp["path"] == "" {
		t.Fatal("expected non-empty path in response")
	}
}

func TestHandleTerminalUpload_InvalidContentType(t *testing.T) {
	mgr := pty.NewManager()
	defer mgr.StopAll()
	deps := testDeps()

	_, err := mgr.Start(pty.StartConfig{
		ID:   "upload-bad",
		Cmd:  "/bin/sh",
		Args: []string{"-c", "exec cat"},
	})
	if err != nil {
		t.Fatalf("start: %v", err)
	}

	sess, _ := mgr.Get("upload-bad")
	relays := NewMap()
	relays.GetOrCreate("upload-bad", sess, t.TempDir())

	req := httptest.NewRequest("POST", "/terminal/upload?session_id=upload-bad&filename=test.txt", bytes.NewReader([]byte("not an image")))
	req.Header.Set("Content-Type", "text/plain")
	rec := httptest.NewRecorder()
	HandleTerminalUpload(rec, req, deps, mgr, relays)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleTerminalUpload_SessionNotFound(t *testing.T) {
	mgr := pty.NewManager()
	relays := NewMap()
	deps := testDeps()

	req := httptest.NewRequest("POST", "/terminal/upload?session_id=nonexistent", bytes.NewReader([]byte("data")))
	req.Header.Set("Content-Type", "image/png")
	rec := httptest.NewRecorder()
	HandleTerminalUpload(rec, req, deps, mgr, relays)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}
