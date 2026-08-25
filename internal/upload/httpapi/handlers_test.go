// handlers_test.go — Drives each upload HTTP endpoint through success and error branches.

package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/upload"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/upload/uploadsec"
)

// newStubbedHandlers builds one isolated handler instance with the supplied
// stage overrides; nil stage fields keep the production defaults.
func newStubbedHandlers(t *testing.T, s stageFunctions) *Handlers {
	t.Helper()
	stages := defaultStageFunctions()
	if s.fileRead != nil {
		stages.fileRead = s.fileRead
	}
	if s.dialogInject != nil {
		stages.dialogInject = s.dialogInject
	}
	if s.formSubmit != nil {
		stages.formSubmit = s.formSubmit
	}
	if s.osAutomation != nil {
		stages.osAutomation = s.osAutomation
	}
	if s.dismissDialog != nil {
		stages.dismissDialog = s.dismissDialog
	}
	return newHandlersWithStages(nil, true, testJSONResponder, stages)
}

func postJSON(path, body string) (*httptest.ResponseRecorder, *http.Request) {
	req := httptest.NewRequest("POST", path, strings.NewReader(body))
	return httptest.NewRecorder(), req
}

// ---------------------------------------------------------------------------
// Handlers.HandleFileRead: success + status mapping for failure messages
// ---------------------------------------------------------------------------

func TestHandlersHandleFileRead_Success(t *testing.T) {
	handlers := newStubbedHandlers(t, stageFunctions{
		fileRead: func(_ upload.FileReadRequest, _ *uploadsec.Security, _ bool) upload.FileReadResponse {
			return upload.FileReadResponse{Success: true, FileName: "a.txt", FileSize: 3}
		},
	})
	w, req := postJSON("/api/file/read", `{"file_path":"/x/a.txt"}`)
	handlers.HandleFileRead(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("success should return 200, got %d", w.Code)
	}
}

func TestHandlersHandleFileRead_ErrorStatusMapping(t *testing.T) {
	cases := []struct {
		name   string
		errMsg string
		want   int
	}{
		{"not found", "file not found", http.StatusNotFound},
		{"no such file", "open /x: no such file or directory", http.StatusNotFound},
		{"permission", "permission denied", http.StatusForbidden},
		{"generic validation", "path outside upload dir", http.StatusBadRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			handlers := newStubbedHandlers(t, stageFunctions{
				fileRead: func(_ upload.FileReadRequest, _ *uploadsec.Security, _ bool) upload.FileReadResponse {
					return upload.FileReadResponse{Success: false, Error: tc.errMsg}
				},
			})
			w, req := postJSON("/api/file/read", `{"file_path":"/x/a.txt"}`)
			handlers.HandleFileRead(w, req)
			if w.Code != tc.want {
				t.Fatalf("error %q: want status %d, got %d", tc.errMsg, tc.want, w.Code)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Handlers.HandleFileDialogInject: success + failure + invalid JSON
// ---------------------------------------------------------------------------

func TestHandlersHandleFileDialogInject_Success(t *testing.T) {
	handlers := newStubbedHandlers(t, stageFunctions{
		dialogInject: func(_ upload.FileDialogInjectRequest, _ *uploadsec.Security) upload.StageResponse {
			return upload.StageResponse{Success: true, Stage: 2}
		},
	})
	w, req := postJSON("/api/file/dialog/inject", `{"file_path":"/x/a.txt","browser_pid":1}`)
	handlers.HandleFileDialogInject(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("success should return 200, got %d", w.Code)
	}
}

func TestHandlersHandleFileDialogInject_Failure(t *testing.T) {
	handlers := newStubbedHandlers(t, stageFunctions{
		dialogInject: func(_ upload.FileDialogInjectRequest, _ *uploadsec.Security) upload.StageResponse {
			return upload.StageResponse{Success: false, Error: "bad pid"}
		},
	})
	w, req := postJSON("/api/file/dialog/inject", `{"file_path":"/x/a.txt"}`)
	handlers.HandleFileDialogInject(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("failure should return 400, got %d", w.Code)
	}
}

func TestHandlersHandleFileDialogInject_InvalidJSON(t *testing.T) {
	handlers := newStubbedHandlers(t, stageFunctions{})
	w, req := postJSON("/api/file/dialog/inject", `{bad`)
	handlers.HandleFileDialogInject(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("invalid JSON should return 400, got %d", w.Code)
	}
}

// ---------------------------------------------------------------------------
// Handlers.HandleFormSubmit: success + failure
// ---------------------------------------------------------------------------

func TestHandlersHandleFormSubmit_Success(t *testing.T) {
	handlers := newStubbedHandlers(t, stageFunctions{
		formSubmit: func(_ context.Context, _ upload.FormSubmitRequest, _ *uploadsec.Security) upload.StageResponse {
			return upload.StageResponse{Success: true, Stage: 3, Status: "ok"}
		},
	})
	w, req := postJSON("/api/form/submit", `{"form_action":"https://h/x","file_path":"/x/a"}`)
	handlers.HandleFormSubmit(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("success should return 200, got %d", w.Code)
	}
}

func TestHandlersHandleFormSubmit_Failure(t *testing.T) {
	handlers := newStubbedHandlers(t, stageFunctions{
		formSubmit: func(_ context.Context, _ upload.FormSubmitRequest, _ *uploadsec.Security) upload.StageResponse {
			return upload.StageResponse{Success: false, Error: "blocked url"}
		},
	})
	w, req := postJSON("/api/form/submit", `{"form_action":"https://h/x"}`)
	handlers.HandleFormSubmit(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("failure should return 400, got %d", w.Code)
	}
}

func TestHandlersHandleFormSubmitPropagatesRequestCancellation(t *testing.T) {
	handlers := newStubbedHandlers(t, stageFunctions{
		formSubmit: func(ctx context.Context, _ upload.FormSubmitRequest, _ *uploadsec.Security) upload.StageResponse {
			if !errors.Is(ctx.Err(), context.Canceled) {
				t.Fatalf("form submission context error = %v, want context canceled", ctx.Err())
			}
			return upload.StageResponse{Success: false, Error: ctx.Err().Error()}
		},
	})
	w, req := postJSON("/api/form/submit", `{"form_action":"https://h/x","file_path":"/x/a"}`)
	ctx, cancel := context.WithCancel(req.Context())
	cancel()
	handlers.HandleFormSubmit(w, req.WithContext(ctx))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("canceled submission should return 400, got %d", w.Code)
	}
}

// ---------------------------------------------------------------------------
// Handlers.HandleOSAutomation: enabled success + failure + invalid JSON
// ---------------------------------------------------------------------------

func TestHandlersHandleOSAutomation_Success(t *testing.T) {
	handlers := newStubbedHandlers(t, stageFunctions{
		osAutomation: func(_ upload.OSAutomationInjectRequest, _ *uploadsec.Security) upload.StageResponse {
			return upload.StageResponse{Success: true, Stage: 4}
		},
	})
	w, req := postJSON("/api/os-automation/inject", `{"file_path":"/x/a","browser_pid":2}`)
	handlers.HandleOSAutomation(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("success should return 200, got %d", w.Code)
	}
}

func TestHandlersHandleOSAutomation_Failure(t *testing.T) {
	handlers := newStubbedHandlers(t, stageFunctions{
		osAutomation: func(_ upload.OSAutomationInjectRequest, _ *uploadsec.Security) upload.StageResponse {
			return upload.StageResponse{Success: false, Stage: 4, Error: "automation failed"}
		},
	})
	w, req := postJSON("/api/os-automation/inject", `{"file_path":"/x/a"}`)
	handlers.HandleOSAutomation(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("failure should return 400, got %d", w.Code)
	}
}

func TestHandlersHandleOSAutomation_InvalidJSON(t *testing.T) {
	handlers := newStubbedHandlers(t, stageFunctions{})
	w, req := postJSON("/api/os-automation/inject", `{bad`)
	handlers.HandleOSAutomation(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("invalid JSON should return 400, got %d", w.Code)
	}
}

// ---------------------------------------------------------------------------
// Handlers.HandleOSAutomationDismiss: enabled success + failure (500)
// ---------------------------------------------------------------------------

func TestHandlersHandleOSAutomationDismiss_Success(t *testing.T) {
	handlers := newStubbedHandlers(t, stageFunctions{
		dismissDialog: func() upload.StageResponse {
			return upload.StageResponse{Success: true, Stage: 4, Status: "dismissed"}
		},
	})
	req := httptest.NewRequest("POST", "/api/os-automation/dismiss", nil)
	w := httptest.NewRecorder()
	handlers.HandleOSAutomationDismiss(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("success should return 200, got %d", w.Code)
	}
}

func TestHandlersHandleOSAutomationDismiss_Failure(t *testing.T) {
	handlers := newStubbedHandlers(t, stageFunctions{
		dismissDialog: func() upload.StageResponse {
			return upload.StageResponse{Success: false, Error: "no dialog"}
		},
	})
	req := httptest.NewRequest("POST", "/api/os-automation/dismiss", nil)
	w := httptest.NewRecorder()
	handlers.HandleOSAutomationDismiss(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("transport failure should return 500, got %d", w.Code)
	}
}

// Ensure the upload package alias import is exercised (compile-time guard that
// the stubbed signatures match the real stage function types).
