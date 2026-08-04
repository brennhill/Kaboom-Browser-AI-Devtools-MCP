// Purpose: Handles Stage 3 multipart form submission: field validation, streaming upload, and HTTP error mapping.
// Docs: docs/features/feature/file-upload/index.md

package upload

import (
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"runtime/debug"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/upload/uploadsec"
)

// uploadHTTPClient is the canonical shared client for Stage 3 form submissions.
// Reuses connections via the default transport pool.
var uploadHTTPClient = &http.Client{
	Timeout:   10 * time.Minute, // Large file uploads can take a while
	Transport: uploadsec.DefaultSSRFTransport(),
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		// Prevent redirect to private IPs (SSRF via redirect)
		if err := uploadsec.ValidateFormActionURL(req.URL.String()); err != nil {
			return fmt.Errorf("redirect blocked: %w", err)
		}
		if len(via) >= 10 {
			return fmt.Errorf("too many redirects")
		}
		return nil
	},
}

func HandleFormSubmit(req FormSubmitRequest, sec *uploadsec.Security) StageResponse {
	return HandleFormSubmitCtx(context.Background(), req, sec)
}

func executeFormSubmitWithClient(ctx context.Context, client *http.Client, req FormSubmitRequest, file *os.File, info os.FileInfo, writer *multipart.Writer, pr *io.PipeReader, pw *io.PipeWriter, start time.Time) StageResponse {
	writeErrCh := runFormWriter(func() error {
		return StreamMultipartForm(pw, writer, req, file)
	})

	httpReq, err := http.NewRequestWithContext(ctx, req.Method, req.FormAction, pr)
	if err != nil {
		_ = pr.Close()
		<-writeErrCh
		return formSubmitStage3Error("Failed to create HTTP request: " + err.Error())
	}

	httpReq.Header.Set("Content-Type", writer.FormDataContentType())
	if req.Cookies != "" {
		httpReq.Header.Set("Cookie", req.Cookies)
	}

	// #nosec G704 -- req.FormAction is pre-validated by uploadsec.ValidateFormActionURL and redirect callback revalidates
	httpResp, err := client.Do(httpReq)
	if err != nil {
		_ = pr.CloseWithError(err)
		<-writeErrCh
		return StageResponse{
			Success: false, Stage: 3,
			Error: "Form submission failed: " + err.Error(), FileName: filepath.Base(req.FilePath),
			FileSizeBytes: info.Size(), DurationMs: time.Since(start).Milliseconds(),
		}
	}
	defer httpResp.Body.Close() //nolint:errcheck // deferred close

	if writeErr := <-writeErrCh; writeErr != nil {
		return StageResponse{
			Success: false, Stage: 3,
			Error: "Error writing form data: " + writeErr.Error(), DurationMs: time.Since(start).Milliseconds(),
		}
	}

	if httpResp.StatusCode >= 400 {
		return buildHTTPErrorResponse(httpResp, filepath.Base(req.FilePath), info.Size(), time.Since(start).Milliseconds())
	}

	return StageResponse{
		Success: true, Stage: 3,
		Status:        fmt.Sprintf("Form interception: %s submitted to platform (HTTP %d)", req.Method, httpResp.StatusCode),
		FileName:      filepath.Base(req.FilePath),
		FileSizeBytes: info.Size(), DurationMs: time.Since(start).Milliseconds(),
	}
}

func runFormWriter(write func() error) <-chan error {
	result := make(chan error, 1)
	go func() { // lint:allow-bare-goroutine — panic must become the caller-visible writer error
		defer func() {
			if recovered := recover(); recovered != nil {
				result <- fmt.Errorf("multipart writer panic: %v\n%s", recovered, debug.Stack())
			}
		}()
		result <- write()
	}()
	return result
}

func HandleFormSubmitCtx(ctx context.Context, req FormSubmitRequest, sec *uploadsec.Security) StageResponse {
	start := time.Now()

	pathResult, err := ValidateFormSubmitFields(&req, sec)
	if err != nil {
		return formSubmitStage3Error(err.Error())
	}

	file, info, err := OpenAndValidateFile(pathResult.ResolvedPath, req.FilePath)
	if err != nil {
		return formSubmitStage3Error(err.Error())
	}
	defer file.Close() //nolint:errcheck // deferred close

	pr, pw := io.Pipe()
	writer := multipart.NewWriter(pw)

	return executeFormSubmitWithClient(ctx, uploadHTTPClient, req, file, info, writer, pr, pw, start)
}
