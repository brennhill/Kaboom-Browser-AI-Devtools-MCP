// Purpose: Streams multipart form data including file content to the upload endpoint.
// Why: Separates multipart streaming from validation and error handling.
package upload

import (
	"fmt"
	"io"
	"mime/multipart"
	"net/textproto"
	"os"
	"path/filepath"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/upload/uploadsec"
)

// StreamMultipartForm writes the multipart form data to the pipe writer.
func StreamMultipartForm(pw *io.PipeWriter, writer *multipart.Writer, req FormSubmitRequest, file *os.File) error {
	defer pw.Close() //nolint:errcheck // pipe close

	if req.CSRFToken != "" {
		if err := writer.WriteField("csrf_token", req.CSRFToken); err != nil {
			return err
		}
	}

	for k, v := range req.Fields {
		if err := writer.WriteField(k, v); err != nil {
			return err
		}
	}

	fileName := filepath.Base(req.FilePath)
	mimeType := DetectMimeType(fileName)
	partHeader := make(textproto.MIMEHeader)
	safeName := uploadsec.SanitizeForContentDisposition(req.FileInputName)
	safeFileName := uploadsec.SanitizeForContentDisposition(fileName)
	partHeader.Set("Content-Disposition",
		fmt.Sprintf(`form-data; name="%s"; filename="%s"`, safeName, safeFileName))
	partHeader.Set("Content-Type", mimeType)

	fw, err := writer.CreatePart(partHeader)
	if err != nil {
		return err
	}

	if _, err := io.Copy(fw, file); err != nil {
		return err
	}

	return writer.Close()
}
