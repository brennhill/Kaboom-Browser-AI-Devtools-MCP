// Purpose: Validates form submission fields, file paths, and security constraints before upload.
// Why: Separates pre-submission validation from streaming and error handling.
package upload

import (
	"fmt"
	"os"
	"strings"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/upload/uploadsec"
)

func ValidateFormSubmitFields(req *FormSubmitRequest, sec *uploadsec.Security) (*uploadsec.PathValidationResult, error) {
	if req.FormAction == "" {
		return nil, fmt.Errorf("missing required parameter: form_action")
	}
	if req.FilePath == "" {
		return nil, fmt.Errorf("missing required parameter: file_path")
	}
	if req.FileInputName == "" {
		return nil, fmt.Errorf("missing required parameter: file_input_name")
	}

	pathResult, pathErr := sec.ValidateFilePath(req.FilePath, true)
	if pathErr != nil {
		return nil, pathErr
	}

	if err := uploadsec.ValidateFormActionURL(req.FormAction); err != nil {
		return nil, fmt.Errorf("invalid form_action URL: %w", err)
	}
	if req.Method == "" {
		req.Method = "POST"
	}
	if err := uploadsec.ValidateHTTPMethod(req.Method); err != nil {
		return nil, err
	}
	if err := uploadsec.ValidateCookieHeader(req.Cookies); err != nil {
		return nil, err
	}

	for k := range req.Fields {
		if strings.ContainsAny(k, "\r\n\x00\"") {
			return nil, fmt.Errorf("form field name %q contains invalid characters", k)
		}
	}

	return pathResult, nil
}

func OpenAndValidateFile(resolvedPath, displayPath string) (*os.File, os.FileInfo, error) {
	// #nosec G304 -- file path validated by uploadsec.Security chain
	file, err := os.Open(resolvedPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, fmt.Errorf("file not found: %s: %w", displayPath, err)
		}
		return nil, nil, fmt.Errorf("failed to open file: %s: %w", displayPath, err)
	}

	info, err := file.Stat()
	if err != nil {
		file.Close() //nolint:errcheck // closing on error path
		return nil, nil, fmt.Errorf("failed to stat file: %s: %w", displayPath, err)
	}

	if err := uploadsec.CheckHardlink(info); err != nil {
		file.Close() //nolint:errcheck // closing on error path
		return nil, nil, err
	}

	return file, info, nil
}
