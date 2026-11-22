// Purpose: Defines error helpers and HTTP status error messages for form submission failures.
// Why: Separates error formatting from form submission streaming and validation.
package upload

import (
	"fmt"
	"net/http"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/platform/util"
)

func formSubmitStage3Error(msg string) StageResponse {
	return StageResponse{Success: false, Stage: 3, Error: msg}
}

var httpStatusErrors = map[int]string{
	401: "User not logged into platform (HTTP 401). Please log in and retry.",
	403: "CSRF token mismatch or forbidden (HTTP 403). Token may be expired.",
	422: "Form validation failed (HTTP 422). Check required fields.",
}

func buildHTTPErrorResponse(resp *http.Response, fileName string, fileSize int64, elapsed int64) StageResponse {
	bodyBytes := make([]byte, 1024)
	n, _ := resp.Body.Read(bodyBytes)
	bodyPreview := string(bodyBytes[:n])

	errMsg, ok := httpStatusErrors[resp.StatusCode]
	if !ok {
		errMsg = fmt.Sprintf("Platform returned HTTP %d", resp.StatusCode)
	}

	return StageResponse{
		Success:       false,
		Stage:         3,
		Error:         errMsg,
		FileName:      fileName,
		FileSizeBytes: fileSize,
		DurationMs:    elapsed,
		Suggestions:   []string{"Check authentication", "Verify CSRF token", "Response: " + util.Truncate(bodyPreview, 200)},
	}
}
