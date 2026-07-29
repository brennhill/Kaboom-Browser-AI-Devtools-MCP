// form_submit_writer_test.go — Panic-to-error contract for multipart writer goroutines.
package upload

import (
	"strings"
	"testing"
)

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
