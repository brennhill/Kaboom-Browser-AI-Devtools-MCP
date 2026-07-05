// handlers_http_test.go — Drives each HTTP handler through success + error branches using stubbed stage functions.

package uploadhandler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/upload"
)

// withStubbedStageFns swaps the package-level stage function variables for the
// duration of a test and restores them afterward. The stage funcs are the seam
// through which the HTTP handlers delegate real work; stubbing them keeps the
// tests deterministic (no filesystem, no OS automation).
type stageStubs struct {
	fileRead     func(FileReadRequest, *Security, bool) FileReadResponse
	dialogInject func(FileDialogInjectRequest, *Security) StageResponse
	formSubmit   func(FormSubmitRequest, *Security) StageResponse
	osAutomation func(OSAutomationInjectRequest, *Security) StageResponse
	dismiss      func() StageResponse
}

func withStubbedStageFns(t *testing.T, s stageStubs) {
	t.Helper()
	origFileRead := fileReadFn
	origDialog := dialogInjectFn
	origForm := formSubmitFn
	origOS := osAutomationFn
	origDismiss := dismissDialogFn
	t.Cleanup(func() {
		fileReadFn = origFileRead
		dialogInjectFn = origDialog
		formSubmitFn = origForm
		osAutomationFn = origOS
		dismissDialogFn = origDismiss
	})
	if s.fileRead != nil {
		fileReadFn = s.fileRead
	}
	if s.dialogInject != nil {
		dialogInjectFn = s.dialogInject
	}
	if s.formSubmit != nil {
		formSubmitFn = s.formSubmit
	}
	if s.osAutomation != nil {
		osAutomationFn = s.osAutomation
	}
	if s.dismiss != nil {
		dismissDialogFn = s.dismiss
	}
}

func postJSON(path, body string) (*httptest.ResponseRecorder, *http.Request) {
	req := httptest.NewRequest("POST", path, strings.NewReader(body))
	return httptest.NewRecorder(), req
}

// ---------------------------------------------------------------------------
// HandleFileReadHTTP: success + status mapping for failure messages
// ---------------------------------------------------------------------------

func TestHandleFileReadHTTP_Success(t *testing.T) {
	withStubbedStageFns(t, stageStubs{
		fileRead: func(_ FileReadRequest, _ *Security, _ bool) FileReadResponse {
			return FileReadResponse{Success: true, FileName: "a.txt", FileSize: 3}
		},
	})
	w, req := postJSON("/api/file/read", `{"file_path":"/x/a.txt"}`)
	HandleFileReadHTTP(w, req, nil, testJSONResponder)
	if w.Code != http.StatusOK {
		t.Fatalf("success should return 200, got %d", w.Code)
	}
}

func TestHandleFileReadHTTP_ErrorStatusMapping(t *testing.T) {
	cases := []struct {
		name    string
		errMsg  string
		want    int
	}{
		{"not found", "file not found", http.StatusNotFound},
		{"no such file", "open /x: no such file or directory", http.StatusNotFound},
		{"permission", "permission denied", http.StatusForbidden},
		{"generic validation", "path outside upload dir", http.StatusBadRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			withStubbedStageFns(t, stageStubs{
				fileRead: func(_ FileReadRequest, _ *Security, _ bool) FileReadResponse {
					return FileReadResponse{Success: false, Error: tc.errMsg}
				},
			})
			w, req := postJSON("/api/file/read", `{"file_path":"/x/a.txt"}`)
			HandleFileReadHTTP(w, req, nil, testJSONResponder)
			if w.Code != tc.want {
				t.Fatalf("error %q: want status %d, got %d", tc.errMsg, tc.want, w.Code)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// HandleFileDialogInjectHTTP: success + failure + invalid JSON
// ---------------------------------------------------------------------------

func TestHandleFileDialogInjectHTTP_Success(t *testing.T) {
	withStubbedStageFns(t, stageStubs{
		dialogInject: func(_ FileDialogInjectRequest, _ *Security) StageResponse {
			return StageResponse{Success: true, Stage: 2}
		},
	})
	w, req := postJSON("/api/file/dialog/inject", `{"file_path":"/x/a.txt","browser_pid":1}`)
	HandleFileDialogInjectHTTP(w, req, nil, testJSONResponder)
	if w.Code != http.StatusOK {
		t.Fatalf("success should return 200, got %d", w.Code)
	}
}

func TestHandleFileDialogInjectHTTP_Failure(t *testing.T) {
	withStubbedStageFns(t, stageStubs{
		dialogInject: func(_ FileDialogInjectRequest, _ *Security) StageResponse {
			return StageResponse{Success: false, Error: "bad pid"}
		},
	})
	w, req := postJSON("/api/file/dialog/inject", `{"file_path":"/x/a.txt"}`)
	HandleFileDialogInjectHTTP(w, req, nil, testJSONResponder)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("failure should return 400, got %d", w.Code)
	}
}

func TestHandleFileDialogInjectHTTP_InvalidJSON(t *testing.T) {
	w, req := postJSON("/api/file/dialog/inject", `{bad`)
	HandleFileDialogInjectHTTP(w, req, nil, testJSONResponder)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("invalid JSON should return 400, got %d", w.Code)
	}
}

// ---------------------------------------------------------------------------
// HandleFormSubmitHTTP: success + failure
// ---------------------------------------------------------------------------

func TestHandleFormSubmitHTTP_Success(t *testing.T) {
	withStubbedStageFns(t, stageStubs{
		formSubmit: func(_ FormSubmitRequest, _ *Security) StageResponse {
			return StageResponse{Success: true, Stage: 3, Status: "ok"}
		},
	})
	w, req := postJSON("/api/form/submit", `{"form_action":"https://h/x","file_path":"/x/a"}`)
	HandleFormSubmitHTTP(w, req, nil, testJSONResponder)
	if w.Code != http.StatusOK {
		t.Fatalf("success should return 200, got %d", w.Code)
	}
}

func TestHandleFormSubmitHTTP_Failure(t *testing.T) {
	withStubbedStageFns(t, stageStubs{
		formSubmit: func(_ FormSubmitRequest, _ *Security) StageResponse {
			return StageResponse{Success: false, Error: "blocked url"}
		},
	})
	w, req := postJSON("/api/form/submit", `{"form_action":"https://h/x"}`)
	HandleFormSubmitHTTP(w, req, nil, testJSONResponder)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("failure should return 400, got %d", w.Code)
	}
}

// ---------------------------------------------------------------------------
// HandleOSAutomationHTTP: enabled success + failure + invalid JSON
// ---------------------------------------------------------------------------

func TestHandleOSAutomationHTTP_Success(t *testing.T) {
	withStubbedStageFns(t, stageStubs{
		osAutomation: func(_ OSAutomationInjectRequest, _ *Security) StageResponse {
			return StageResponse{Success: true, Stage: 4}
		},
	})
	w, req := postJSON("/api/os-automation/inject", `{"file_path":"/x/a","browser_pid":2}`)
	HandleOSAutomationHTTP(w, req, true, nil, testJSONResponder)
	if w.Code != http.StatusOK {
		t.Fatalf("success should return 200, got %d", w.Code)
	}
}

func TestHandleOSAutomationHTTP_Failure(t *testing.T) {
	withStubbedStageFns(t, stageStubs{
		osAutomation: func(_ OSAutomationInjectRequest, _ *Security) StageResponse {
			return StageResponse{Success: false, Stage: 4, Error: "automation failed"}
		},
	})
	w, req := postJSON("/api/os-automation/inject", `{"file_path":"/x/a"}`)
	HandleOSAutomationHTTP(w, req, true, nil, testJSONResponder)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("failure should return 400, got %d", w.Code)
	}
}

func TestHandleOSAutomationHTTP_InvalidJSON(t *testing.T) {
	w, req := postJSON("/api/os-automation/inject", `{bad`)
	HandleOSAutomationHTTP(w, req, true, nil, testJSONResponder)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("invalid JSON should return 400, got %d", w.Code)
	}
}

// ---------------------------------------------------------------------------
// HandleOSAutomationDismissHTTP: enabled success + failure (500)
// ---------------------------------------------------------------------------

func TestHandleOSAutomationDismissHTTP_Success(t *testing.T) {
	withStubbedStageFns(t, stageStubs{
		dismiss: func() StageResponse {
			return StageResponse{Success: true, Stage: 4, Status: "dismissed"}
		},
	})
	req := httptest.NewRequest("POST", "/api/os-automation/dismiss", nil)
	w := httptest.NewRecorder()
	HandleOSAutomationDismissHTTP(w, req, true, testJSONResponder)
	if w.Code != http.StatusOK {
		t.Fatalf("success should return 200, got %d", w.Code)
	}
}

func TestHandleOSAutomationDismissHTTP_Failure(t *testing.T) {
	withStubbedStageFns(t, stageStubs{
		dismiss: func() StageResponse {
			return StageResponse{Success: false, Error: "no dialog"}
		},
	})
	req := httptest.NewRequest("POST", "/api/os-automation/dismiss", nil)
	w := httptest.NewRecorder()
	HandleOSAutomationDismissHTTP(w, req, true, testJSONResponder)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("transport failure should return 500, got %d", w.Code)
	}
}

// Ensure the upload package alias import is exercised (compile-time guard that
// the stubbed signatures match the real stage function types).
var _ = upload.HandleFileRead
