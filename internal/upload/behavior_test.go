// behavior_test.go — Tests deterministic file-read boundaries and upload metadata helpers.

package upload

import (
	"encoding/base64"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/upload/uploadsec"
)

func TestFileReadBase64BoundaryWithoutLargeFixture(t *testing.T) {
	dir := t.TempDir()
	security := uploadsec.NewSecurity(dir, nil)
	for _, test := range []struct {
		name       string
		content    []byte
		wantBase64 bool
	}{
		{name: "exact", content: []byte{0, 1, 2, 3, 4, 5, 6, 7}, wantBase64: true},
		{name: "above", content: []byte{0, 1, 2, 3, 4, 5, 6, 7, 8}, wantBase64: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(dir, test.name+".bin")
			if err := os.WriteFile(path, test.content, 0o600); err != nil {
				t.Fatal(err)
			}
			response := handleFileReadWithLimit(FileReadRequest{FilePath: path}, security, false, 8)
			if !response.Success || (response.DataBase64 != "") != test.wantBase64 {
				t.Fatalf("response = %#v", response)
			}
			if test.wantBase64 {
				decoded, err := base64.StdEncoding.DecodeString(response.DataBase64)
				if err != nil || string(decoded) != string(test.content) {
					t.Fatalf("decoded = %v, %v", decoded, err)
				}
			}
		})
	}
}

func TestFileReadRejectsDirectoriesAndRelativePaths(t *testing.T) {
	security := uploadsec.NewSecurity(t.TempDir(), nil)
	if response := HandleFileRead(FileReadRequest{FilePath: t.TempDir()}, security, false); response.Success || !strings.Contains(strings.ToLower(response.Error), "directory") {
		t.Fatalf("directory response = %#v", response)
	}
	if response := HandleFileRead(FileReadRequest{FilePath: "relative.txt"}, security, false); response.Success || !strings.Contains(response.Error, "absolute path") {
		t.Fatalf("relative response = %#v", response)
	}
}

func TestUploadMetadataBoundaries(t *testing.T) {
	for filename, expected := range map[string]string{
		"FILE.MP4":       "video/mp4",
		"Image.JPG":      "image/jpeg",
		"Makefile":       "application/octet-stream",
		".gitignore":     "application/octet-stream",
		"archive.tar.gz": "application/gzip",
	} {
		if actual := DetectMimeType(filename); actual != expected {
			t.Fatalf("DetectMimeType(%q) = %q, want %q", filename, actual, expected)
		}
	}
	if tier := GetProgressTier(0); tier != ProgressTierSimple {
		t.Fatalf("zero-byte progress tier = %q", tier)
	}
}

func TestFormSubmitStreamsFileFieldsCSRFAndCookies(t *testing.T) {
	uploadsec.SetSkipSSRFCheck(true)
	t.Cleanup(func() { uploadsec.SetSkipSSRFCheck(false) })
	received := make(map[string]string)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		received["cookie"] = request.Header.Get("Cookie")
		_, parameters, err := mime.ParseMediaType(request.Header.Get("Content-Type"))
		if err != nil {
			t.Error(err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		reader := multipart.NewReader(request.Body, parameters["boundary"])
		for {
			part, nextErr := reader.NextPart()
			if nextErr == io.EOF {
				break
			}
			if nextErr != nil {
				t.Error(nextErr)
				return
			}
			data, readErr := io.ReadAll(part)
			if readErr != nil {
				t.Error(readErr)
				return
			}
			received[part.FormName()] = string(data)
			if part.FileName() != "" {
				received["filename"] = part.FileName()
			}
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	dir := t.TempDir()
	path := filepath.Join(dir, "upload.txt")
	if err := os.WriteFile(path, []byte("file content"), 0o600); err != nil {
		t.Fatal(err)
	}
	response := HandleFormSubmit(FormSubmitRequest{
		FormAction: server.URL, Method: http.MethodPost, FileInputName: "file", FilePath: path,
		CSRFToken: "token", Fields: map[string]string{"title": "value"}, Cookies: "session=abc",
	}, testSecurityWithDir(t, dir))
	if !response.Success || received["file"] != "file content" || received["filename"] != "upload.txt" ||
		received["csrf_token"] != "token" || received["title"] != "value" || received["cookie"] != "session=abc" {
		t.Fatalf("response/received = %#v/%#v", response, received)
	}
}

func TestFormSubmitReportsUpstreamStatus(t *testing.T) {
	uploadsec.SetSkipSSRFCheck(true)
	t.Cleanup(func() { uploadsec.SetSkipSSRFCheck(false) })
	dir := t.TempDir()
	path := filepath.Join(dir, "upload.txt")
	if err := os.WriteFile(path, []byte("content"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, status := range []int{http.StatusUnauthorized, http.StatusForbidden, http.StatusUnprocessableEntity} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(status) }))
			defer server.Close()
			response := HandleFormSubmit(FormSubmitRequest{FormAction: server.URL, Method: http.MethodPost, FileInputName: "file", FilePath: path}, testSecurityWithDir(t, dir))
			if response.Success || !strings.Contains(response.Error, strconv.Itoa(status)) {
				t.Fatalf("status %d response = %#v", status, response)
			}
		})
	}
}
