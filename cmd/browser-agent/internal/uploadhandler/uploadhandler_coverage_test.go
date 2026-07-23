// uploadhandler_coverage_test.go — Status mapping and gating for the upload HTTP handlers.
//
// The handlers themselves contain one piece of real logic each: how a stage
// result becomes an HTTP status, and whether the stage runs at all. Both are
// exercised here by swapping the package-level stage functions for stubs, which
// keeps the tests off the filesystem and off the network entirely.
//
// These tests mutate package globals, so none of them may use t.Parallel().

package uploadhandler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Stage stubs
// ---------------------------------------------------------------------------

// covStubFileRead replaces the stage-1 implementation for the duration of a test.
func covStubFileRead(t *testing.T, fn func(FileReadRequest, *Security, bool) FileReadResponse) {
	t.Helper()
	original := fileReadFn
	fileReadFn = fn
	t.Cleanup(func() { fileReadFn = original })
}

func covStubDialogInject(t *testing.T, fn func(FileDialogInjectRequest, *Security) StageResponse) {
	t.Helper()
	original := dialogInjectFn
	dialogInjectFn = fn
	t.Cleanup(func() { dialogInjectFn = original })
}

func covStubFormSubmit(t *testing.T, fn func(FormSubmitRequest, *Security) StageResponse) {
	t.Helper()
	original := formSubmitFn
	formSubmitFn = fn
	t.Cleanup(func() { formSubmitFn = original })
}

func covStubOSAutomation(t *testing.T, fn func(OSAutomationInjectRequest, *Security) StageResponse) {
	t.Helper()
	original := osAutomationFn
	osAutomationFn = fn
	t.Cleanup(func() { osAutomationFn = original })
}

func covStubDismiss(t *testing.T, fn func() StageResponse) {
	t.Helper()
	original := dismissDialogFn
	dismissDialogFn = fn
	t.Cleanup(func() { dismissDialogFn = original })
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func covUploadPost(t *testing.T, path, body string) *http.Request {
	t.Helper()
	req := httptest.NewRequest("POST", path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	return req
}

func covUploadBody(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode response: %v (body=%s)", err, rec.Body.String())
	}
	return out
}

// ---------------------------------------------------------------------------
// Stage 1 — POST /api/file/read
// ---------------------------------------------------------------------------

// TestFileRead_MapsStageErrorsToStatus pins the whole error-to-status table.
// Collapsing 404/403 into a blanket 400 would make the extension retry a
// permission failure as if the request had been malformed.
func TestFileRead_MapsStageErrorsToStatus(t *testing.T) {
	cases := []struct {
		name       string
		stageError string
		want       int
	}{
		{"not found", "file not found: /tmp/x", http.StatusNotFound},
		{"no such file", "open /tmp/x: no such file or directory", http.StatusNotFound},
		{"permission", "permission denied reading /etc/shadow", http.StatusForbidden},
		{"denied by policy", "path is on the upload denylist", http.StatusBadRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			covStubFileRead(t, func(FileReadRequest, *Security, bool) FileReadResponse {
				return FileReadResponse{Success: false, Error: tc.stageError}
			})

			rec := httptest.NewRecorder()
			HandleFileReadHTTP(rec, covUploadPost(t, "/api/file/read", `{"file_path":"/tmp/x"}`), nil, testJSONResponder)

			if rec.Code != tc.want {
				t.Fatalf("status = %d, want %d (body=%s)", rec.Code, tc.want, rec.Body.String())
			}
			body := covUploadBody(t, rec)
			if body["success"] != false {
				t.Fatalf("success = %v, want false", body["success"])
			}
			// The stage error is the only diagnostic the caller gets; a
			// rewritten message loses the path that failed.
			if body["error"] != tc.stageError {
				t.Fatalf("error = %v, want %q", body["error"], tc.stageError)
			}
		})
	}
}

func TestFileRead_SuccessReturnsMetadataInSnakeCase(t *testing.T) {
	covStubFileRead(t, func(FileReadRequest, *Security, bool) FileReadResponse {
		return FileReadResponse{Success: true, FileName: "report.pdf", FileSize: 2048, MimeType: "application/pdf"}
	})

	rec := httptest.NewRecorder()
	HandleFileReadHTTP(rec, covUploadPost(t, "/api/file/read", `{"file_path":"/tmp/report.pdf"}`), nil, testJSONResponder)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
	body := covUploadBody(t, rec)
	// Wire contract with the extension: these keys are snake_case, always.
	for key, want := range map[string]any{
		"success":   true,
		"file_name": "report.pdf",
		"file_size": float64(2048),
		"mime_type": "application/pdf",
	} {
		if body[key] != want {
			t.Errorf("%s = %v, want %v", key, body[key], want)
		}
	}
}

func TestFileRead_ForwardsPathAndNeverRequestsBase64(t *testing.T) {
	var gotPath string
	var gotBase64 bool
	covStubFileRead(t, func(req FileReadRequest, _ *Security, base64 bool) FileReadResponse {
		gotPath, gotBase64 = req.FilePath, base64
		return FileReadResponse{Success: true}
	})

	rec := httptest.NewRecorder()
	HandleFileReadHTTP(rec, covUploadPost(t, "/api/file/read", `{"file_path":"/tmp/big.iso"}`), nil, testJSONResponder)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if gotPath != "/tmp/big.iso" {
		t.Fatalf("stage received file_path %q, want /tmp/big.iso", gotPath)
	}
	// Stage 1 is a metadata probe. Requesting base64 here would inline whole
	// files into an HTTP response the caller only wanted a size from.
	if gotBase64 {
		t.Fatal("stage 1 must not ask for base64 content")
	}
}

// ---------------------------------------------------------------------------
// Stage 2 — POST /api/file/dialog/inject
// ---------------------------------------------------------------------------

func TestDialogInject_RejectsMalformedBody(t *testing.T) {
	called := false
	covStubDialogInject(t, func(FileDialogInjectRequest, *Security) StageResponse {
		called = true
		return StageResponse{Success: true}
	})

	rec := httptest.NewRecorder()
	HandleFileDialogInjectHTTP(rec, covUploadPost(t, "/api/file/dialog/inject", `{"file_path":`), nil, testJSONResponder)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	if msg, _ := covUploadBody(t, rec)["error"].(string); !strings.HasPrefix(msg, "Invalid JSON: ") {
		t.Fatalf("error = %q, want an Invalid JSON prefix", msg)
	}
	// An unparsed body must never reach the dialog automation.
	if called {
		t.Fatal("stage 2 ran despite a malformed body")
	}
}

func TestDialogInject_MapsStageOutcomeToStatus(t *testing.T) {
	for _, tc := range []struct {
		name    string
		success bool
		want    int
	}{
		{"success", true, http.StatusOK},
		{"failure", false, http.StatusBadRequest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			covStubDialogInject(t, func(req FileDialogInjectRequest, _ *Security) StageResponse {
				return StageResponse{Success: tc.success, Stage: 2, Status: "done", Error: "dialog not found"}
			})

			rec := httptest.NewRecorder()
			HandleFileDialogInjectHTTP(rec, covUploadPost(t, "/api/file/dialog/inject", `{"file_path":"/tmp/a","browser_pid":42}`), nil, testJSONResponder)

			if rec.Code != tc.want {
				t.Fatalf("status = %d, want %d (body=%s)", rec.Code, tc.want, rec.Body.String())
			}
			if body := covUploadBody(t, rec); body["stage"] != float64(2) {
				t.Fatalf("stage = %v, want 2 echoed back", body["stage"])
			}
		})
	}
}

func TestDialogInject_ForwardsBrowserPID(t *testing.T) {
	var gotPID int
	covStubDialogInject(t, func(req FileDialogInjectRequest, _ *Security) StageResponse {
		gotPID = req.BrowserPID
		return StageResponse{Success: true}
	})

	rec := httptest.NewRecorder()
	HandleFileDialogInjectHTTP(rec, covUploadPost(t, "/api/file/dialog/inject", `{"file_path":"/tmp/a","browser_pid":1234}`), nil, testJSONResponder)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	// Injecting into the wrong PID types a path into an unrelated application.
	if gotPID != 1234 {
		t.Fatalf("browser_pid = %d, want 1234", gotPID)
	}
}

// ---------------------------------------------------------------------------
// Stage 3 — POST /api/form/submit
// ---------------------------------------------------------------------------

func TestFormSubmit_MapsStageOutcomeToStatus(t *testing.T) {
	for _, tc := range []struct {
		name    string
		success bool
		want    int
	}{
		{"success", true, http.StatusOK},
		{"failure", false, http.StatusBadRequest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			covStubFormSubmit(t, func(FormSubmitRequest, *Security) StageResponse {
				return StageResponse{Success: tc.success, Stage: 3, FileName: "a.png", FileSizeBytes: 12}
			})

			rec := httptest.NewRecorder()
			HandleFormSubmitHTTP(rec, covUploadPost(t, "/api/form/submit", `{"form_action":"https://x.test/u","file_path":"/tmp/a.png","file_input_name":"file"}`), nil, testJSONResponder)

			if rec.Code != tc.want {
				t.Fatalf("status = %d, want %d (body=%s)", rec.Code, tc.want, rec.Body.String())
			}
			body := covUploadBody(t, rec)
			if body["file_size_bytes"] != float64(12) {
				t.Fatalf("file_size_bytes = %v, want 12", body["file_size_bytes"])
			}
		})
	}
}

func TestFormSubmit_ForwardsTheWholeRequest(t *testing.T) {
	var got FormSubmitRequest
	covStubFormSubmit(t, func(req FormSubmitRequest, _ *Security) StageResponse {
		got = req
		return StageResponse{Success: true}
	})

	rec := httptest.NewRecorder()
	HandleFormSubmitHTTP(rec, covUploadPost(t, "/api/form/submit",
		`{"form_action":"https://x.test/u","method":"PUT","file_input_name":"avatar","file_path":"/tmp/a.png","csrf_token":"tok","fields":{"k":"v"}}`),
		nil, testJSONResponder)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
	// Dropping any of these silently changes what gets posted where.
	if got.FormAction != "https://x.test/u" || got.Method != "PUT" || got.FileInputName != "avatar" || got.CSRFToken != "tok" {
		t.Fatalf("decoded request = %+v, want every field carried through", got)
	}
	if got.Fields["k"] != "v" {
		t.Fatalf("fields = %v, want k=v", got.Fields)
	}
}

// ---------------------------------------------------------------------------
// Stage 4 — POST /api/os-automation/inject
//
// Stage 4 drives the host's keyboard and mouse. The gate is the whole security
// story for this endpoint, so it is asserted from both directions: it must
// refuse when disabled, and it must refuse *before* parsing anything.
// ---------------------------------------------------------------------------

func TestOSAutomation_DisabledRefusesBeforeParsingTheBody(t *testing.T) {
	called := false
	covStubOSAutomation(t, func(OSAutomationInjectRequest, *Security) StageResponse {
		called = true
		return StageResponse{Success: true}
	})

	rec := httptest.NewRecorder()
	HandleOSAutomationHTTP(rec, covUploadPost(t, "/api/os-automation/inject", `{"file_path":`), false, nil, testJSONResponder)

	// A body-parse 400 here would mean the gate sits behind the parser; the
	// gate must come first so no attacker-shaped input is ever interpreted.
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 even for a malformed body", rec.Code)
	}
	body := covUploadBody(t, rec)
	if body["stage"] != float64(4) {
		t.Fatalf("stage = %v, want 4", body["stage"])
	}
	if msg, _ := body["error"].(string); !strings.Contains(msg, "--enable-os-upload-automation") {
		t.Fatalf("error = %q, want it to name the flag that enables the feature", msg)
	}
	if called {
		t.Fatal("OS automation ran while disabled")
	}
}

func TestOSAutomation_EnabledRejectsMalformedBody(t *testing.T) {
	called := false
	covStubOSAutomation(t, func(OSAutomationInjectRequest, *Security) StageResponse {
		called = true
		return StageResponse{Success: true}
	})

	rec := httptest.NewRecorder()
	HandleOSAutomationHTTP(rec, covUploadPost(t, "/api/os-automation/inject", `{"file_path":`), true, nil, testJSONResponder)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	body := covUploadBody(t, rec)
	if body["stage"] != float64(4) {
		t.Fatalf("stage = %v, want 4", body["stage"])
	}
	if msg, _ := body["error"].(string); !strings.HasPrefix(msg, "Invalid JSON: ") {
		t.Fatalf("error = %q, want an Invalid JSON prefix", msg)
	}
	if called {
		t.Fatal("OS automation ran on an unparsed body")
	}
}

func TestOSAutomation_MapsStageOutcomeToStatus(t *testing.T) {
	for _, tc := range []struct {
		name    string
		success bool
		want    int
	}{
		{"success", true, http.StatusOK},
		{"failure", false, http.StatusBadRequest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var got OSAutomationInjectRequest
			covStubOSAutomation(t, func(req OSAutomationInjectRequest, _ *Security) StageResponse {
				got = req
				return StageResponse{Success: tc.success, Stage: 4, EscalationReason: "dialog_open"}
			})

			rec := httptest.NewRecorder()
			HandleOSAutomationHTTP(rec, covUploadPost(t, "/api/os-automation/inject",
				`{"file_path":"/tmp/a.png","browser_pid":99,"retry_count":2}`), true, nil, testJSONResponder)

			if rec.Code != tc.want {
				t.Fatalf("status = %d, want %d (body=%s)", rec.Code, tc.want, rec.Body.String())
			}
			if got.FilePath != "/tmp/a.png" || got.BrowserPID != 99 || got.RetryCount != 2 {
				t.Fatalf("decoded request = %+v, want every field carried through", got)
			}
			if body := covUploadBody(t, rec); body["escalation_reason"] != "dialog_open" {
				t.Fatalf("escalation_reason = %v, want it echoed", body["escalation_reason"])
			}
		})
	}
}

func TestOSAutomation_WrongMethodAdvertisesPOST(t *testing.T) {
	rec := httptest.NewRecorder()
	HandleOSAutomationHTTP(rec, httptest.NewRequest("PUT", "/api/os-automation/inject", nil), true, nil, testJSONResponder)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rec.Code)
	}
	// RFC 9110 requires Allow on a 405; without it a client cannot discover the
	// supported method.
	if got := rec.Header().Get("Allow"); got != "POST" {
		t.Fatalf("Allow = %q, want POST", got)
	}
}

// ---------------------------------------------------------------------------
// Stage 4 — POST /api/os-automation/dismiss
// ---------------------------------------------------------------------------

func TestOSAutomationDismiss_DisabledDoesNotTouchTheDialog(t *testing.T) {
	called := false
	covStubDismiss(t, func() StageResponse {
		called = true
		return StageResponse{Success: true}
	})

	rec := httptest.NewRecorder()
	HandleOSAutomationDismissHTTP(rec, httptest.NewRequest("POST", "/api/os-automation/dismiss", nil), false, testJSONResponder)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
	if body := covUploadBody(t, rec); body["stage"] != float64(4) {
		t.Fatalf("stage = %v, want 4", body["stage"])
	}
	// Sending Escape to whatever window has focus is a real side effect; the
	// gate must stop it, not merely report a failure afterwards.
	if called {
		t.Fatal("dismiss ran while OS automation was disabled")
	}
}

func TestOSAutomationDismiss_Succeeds(t *testing.T) {
	covStubDismiss(t, func() StageResponse {
		return StageResponse{Success: true, Stage: 4, Status: "dismissed"}
	})

	rec := httptest.NewRecorder()
	HandleOSAutomationDismissHTTP(rec, httptest.NewRequest("POST", "/api/os-automation/dismiss", nil), true, testJSONResponder)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
	if body := covUploadBody(t, rec); body["status"] != "dismissed" {
		t.Fatalf("status field = %v, want \"dismissed\"", body["status"])
	}
}

func TestOSAutomationDismiss_TransportFailureIs500(t *testing.T) {
	covStubDismiss(t, func() StageResponse {
		return StageResponse{Success: false, Stage: 4, Error: "osascript exited 1"}
	})

	rec := httptest.NewRecorder()
	HandleOSAutomationDismissHTTP(rec, httptest.NewRequest("POST", "/api/os-automation/dismiss", nil), true, testJSONResponder)

	// The request was valid and permitted; it failed on the host side, so this
	// is a 500 rather than a 400 the client would pointlessly reformulate.
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500 (body=%s)", rec.Code, rec.Body.String())
	}
	if body := covUploadBody(t, rec); body["error"] != "osascript exited 1" {
		t.Fatalf("error = %v, want the underlying failure preserved", body["error"])
	}
}

func TestOSAutomationDismiss_WrongMethodAdvertisesPOST(t *testing.T) {
	rec := httptest.NewRecorder()
	HandleOSAutomationDismissHTTP(rec, httptest.NewRequest("DELETE", "/api/os-automation/dismiss", nil), true, testJSONResponder)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rec.Code)
	}
	if got := rec.Header().Get("Allow"); got != "POST" {
		t.Fatalf("Allow = %q, want POST", got)
	}
}

// ---------------------------------------------------------------------------
// Method gating that the existing suite does not cover
// ---------------------------------------------------------------------------

func TestUploadHandlers_RejectNonPOSTWithoutRunningTheStage(t *testing.T) {
	ran := false
	covStubFileRead(t, func(FileReadRequest, *Security, bool) FileReadResponse { ran = true; return FileReadResponse{} })
	covStubDialogInject(t, func(FileDialogInjectRequest, *Security) StageResponse { ran = true; return StageResponse{} })
	covStubFormSubmit(t, func(FormSubmitRequest, *Security) StageResponse { ran = true; return StageResponse{} })

	calls := []struct {
		name string
		call func(http.ResponseWriter, *http.Request)
		path string
	}{
		{"file read", func(w http.ResponseWriter, r *http.Request) {
			HandleFileReadHTTP(w, r, nil, testJSONResponder)
		}, "/api/file/read"},
		{"dialog inject", func(w http.ResponseWriter, r *http.Request) {
			HandleFileDialogInjectHTTP(w, r, nil, testJSONResponder)
		}, "/api/file/dialog/inject"},
		{"form submit", func(w http.ResponseWriter, r *http.Request) {
			HandleFormSubmitHTTP(w, r, nil, testJSONResponder)
		}, "/api/form/submit"},
	}
	for _, c := range calls {
		for _, method := range []string{"GET", "PUT", "DELETE"} {
			rec := httptest.NewRecorder()
			c.call(rec, httptest.NewRequest(method, c.path, nil))
			if rec.Code != http.StatusMethodNotAllowed {
				t.Errorf("%s %s = %d, want 405", method, c.name, rec.Code)
			}
			if body := covUploadBody(t, rec); body["error"] != "Method not allowed" {
				t.Errorf("%s %s error = %v, want \"Method not allowed\"", method, c.name, body["error"])
			}
		}
	}
	if ran {
		t.Fatal("a rejected method must not reach the stage implementation")
	}
}
