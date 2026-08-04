// form_submit_writer_test.go — Panic-to-error contract for multipart writer goroutines.
package upload

import (
	"context"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

type rejectingRoundTripper func(*http.Request) (*http.Response, error)

func (fn rejectingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func TestRunFormWriterConvertsPanicToCallerVisibleError(t *testing.T) {
	result := runFormWriter(func() error {
		panic("writer exploded")
	})
	err := <-result
	if err == nil || !strings.Contains(err.Error(), "writer exploded") {
		t.Fatalf("runFormWriter panic error = %v", err)
	}
}

func TestRunFormWriterPreservesOrdinaryResult(t *testing.T) {
	result := runFormWriter(func() error { return nil })
	if err := <-result; err != nil {
		t.Fatalf("runFormWriter() error = %v", err)
	}
}

func TestExecuteFormSubmitClosesWriterAfterEarlyTransportFailure(t *testing.T) {
	file, err := os.CreateTemp(t.TempDir(), "upload-*.bin")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if _, err := file.Write(make([]byte, 2<<20)); err != nil {
		t.Fatal(err)
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	info, err := file.Stat()
	if err != nil {
		t.Fatal(err)
	}

	reader, writer := io.Pipe()
	multipartWriter := multipart.NewWriter(writer)
	client := &http.Client{Transport: rejectingRoundTripper(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("transport rejected before reading request body")
	})}
	done := make(chan StageResponse, 1)
	go func() {
		done <- executeFormSubmitWithClient(context.Background(), client, FormSubmitRequest{
			Method: "POST", FormAction: "https://upload.example.test", FileInputName: "file", FilePath: file.Name(),
		}, file, info, multipartWriter, reader, writer, time.Now())
	}()

	select {
	case response := <-done:
		if response.Success || !strings.Contains(response.Error, "transport rejected") {
			t.Fatalf("response = %+v, want prompt transport failure", response)
		}
	case <-time.After(time.Second):
		t.Fatal("form submission hung waiting for a writer whose pipe reader was abandoned")
	}
}
