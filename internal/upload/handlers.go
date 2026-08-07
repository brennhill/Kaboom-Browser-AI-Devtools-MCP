// Purpose: Handles Stage 1 (file read with base64 encoding) and Stage 2 (file dialog injection) upload operations.
// Docs: docs/features/feature/file-upload/index.md

package upload

import (
	"encoding/base64"
	"io"
	"os"
	"path/filepath"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/upload/uploadsec"
)

// ============================================
// Stage 1: File Read
// ============================================

// HandleFileRead is the core logic for file read, testable without HTTP.
// Opens the file first, then fstats the open handle to avoid TOCTOU races.
// #lizard forgives
func HandleFileRead(req FileReadRequest, sec *uploadsec.Security, requireUploadDir bool) FileReadResponse {
	return handleFileReadWithLimit(req, sec, requireUploadDir, MaxBase64FileSize)
}

func handleFileReadWithLimit(req FileReadRequest, sec *uploadsec.Security, requireUploadDir bool, base64Limit int64) FileReadResponse {
	if req.FilePath == "" {
		return FileReadResponse{
			Success: false,
			Error:   "Missing required parameter: file_path",
		}
	}

	// uploadsec.Security: full validation chain (Clean → IsAbs → EvalSymlinks → denylist → upload-dir)
	result, err := sec.ValidateFilePath(req.FilePath, requireUploadDir)
	if err != nil {
		return FileReadResponse{
			Success: false,
			Error:   err.Error(),
		}
	}

	// Open the resolved path (symlink-free, TOCTOU safe)
	// #nosec G304 -- file path validated by uploadsec.Security chain
	file, err := os.Open(result.ResolvedPath)
	if err != nil {
		if os.IsNotExist(err) {
			return FileReadResponse{
				Success: false,
				Error:   "File not found: " + req.FilePath + ". Verify the file path is correct.",
			}
		}
		if os.IsPermission(err) {
			return FileReadResponse{
				Success: false,
				Error:   "Permission denied reading file: " + req.FilePath + ". Check file permissions.",
			}
		}
		return FileReadResponse{
			Success: false,
			Error:   "Failed to access file: " + req.FilePath,
		}
	}
	defer file.Close() //nolint:errcheck // deferred close

	info, err := file.Stat()
	if err != nil {
		return FileReadResponse{
			Success: false,
			Error:   "Failed to stat file: " + req.FilePath,
		}
	}

	if info.IsDir() {
		return FileReadResponse{
			Success: false,
			Error:   "Path is a directory, not a file: " + req.FilePath,
		}
	}

	if err := uploadsec.CheckHardlink(info); err != nil {
		return FileReadResponse{
			Success: false,
			Error:   err.Error(),
		}
	}

	fileName := filepath.Base(req.FilePath)
	mimeType := DetectMimeType(fileName)
	fileSize := info.Size()

	resp := FileReadResponse{
		Success:  true,
		FileName: fileName,
		FileSize: fileSize,
		MimeType: mimeType,
	}

	// Only base64 encode files <= 100MB. Files above this threshold
	// return metadata only; use Stage 3 streaming for the actual upload.
	if fileSize <= base64Limit {
		data, err := io.ReadAll(io.LimitReader(file, base64Limit+1))
		if err != nil {
			return FileReadResponse{
				Success: false,
				Error:   "Failed to read file: " + err.Error(),
			}
		}
		resp.DataBase64 = base64.StdEncoding.EncodeToString(data)
	}

	return resp
}

// ============================================
// Stage 2: File Dialog Injection
// ============================================

// HandleDialogInject is the core logic for dialog injection, testable without HTTP.
// Stage 2 requires --upload-dir.
// #lizard forgives
func HandleDialogInject(req FileDialogInjectRequest, sec *uploadsec.Security) StageResponse {
	if req.FilePath == "" {
		return StageResponse{
			Success: false,
			Stage:   2,
			Error:   "Missing required parameter: file_path",
		}
	}

	if req.BrowserPID <= 0 {
		return StageResponse{
			Success: false,
			Stage:   2,
			Error:   "Missing or invalid browser_pid. Provide the Chrome browser process ID.",
		}
	}

	// uploadsec.Security: full validation chain (requires upload-dir for Stage 2)
	result, err := sec.ValidateFilePath(req.FilePath, true)
	if err != nil {
		return StageResponse{
			Success: false,
			Stage:   2,
			Error:   err.Error(),
		}
	}

	// Verify file exists via stat on resolved path
	info, err := os.Stat(result.ResolvedPath)
	if err != nil {
		if os.IsNotExist(err) {
			return StageResponse{
				Success: false,
				Stage:   2,
				Error:   "File not found: " + req.FilePath,
			}
		}
		return StageResponse{
			Success: false,
			Stage:   2,
			Error:   "Failed to access file: " + req.FilePath,
		}
	}

	return StageResponse{
		Success:       true,
		Stage:         2,
		Status:        "File dialog injection queued",
		FileName:      filepath.Base(result.ResolvedPath),
		FileSizeBytes: info.Size(),
	}
}
