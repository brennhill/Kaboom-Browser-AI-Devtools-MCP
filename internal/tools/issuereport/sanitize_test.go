// sanitize_test.go — Tests for issue report sanitization and redaction.

package issuereport

import (
	"strings"
	"testing"
)

// fakeRedactor replaces any occurrence of secrets with [REDACTED].
type fakeRedactor struct {
	secrets []string
}

func (r *fakeRedactor) Redact(input string) string {
	result := input
	for _, s := range r.secrets {
		result = strings.ReplaceAll(result, s, "[REDACTED]")
	}
	return result
}

func TestSanitizeReport_RedactsTitle(t *testing.T) {
	t.Parallel()
	s := NewSanitizer(&fakeRedactor{secrets: []string{"AKIAIOSFODNN7EXAMPLE"}})
	report := IssueReport{
		Title: "Error with key AKIAIOSFODNN7EXAMPLE",
	}
	sanitized := s.SanitizeReport(report)
	if strings.Contains(sanitized.Title, "AKIAIOSFODNN7EXAMPLE") {
		t.Fatalf("title not redacted: %s", sanitized.Title)
	}
	if !strings.Contains(sanitized.Title, "[REDACTED]") {
		t.Fatalf("title missing redaction marker: %s", sanitized.Title)
	}
}

func TestSanitizeReport_RedactsUserContext(t *testing.T) {
	t.Parallel()
	s := NewSanitizer(&fakeRedactor{secrets: []string{"ghp_abc123secret"}})
	report := IssueReport{
		UserContext: "Got error with token ghp_abc123secret",
	}
	sanitized := s.SanitizeReport(report)
	if strings.Contains(sanitized.UserContext, "ghp_abc123secret") {
		t.Fatalf("user_context not redacted: %s", sanitized.UserContext)
	}
}

func TestSanitizeReport_RedactsExtensionSource(t *testing.T) {
	t.Parallel()
	s := NewSanitizer(&fakeRedactor{secrets: []string{"mysecretpath"}})
	report := IssueReport{
		Diagnostics: DiagnosticData{
			Extension: ExtensionDiag{Source: "loaded from mysecretpath"},
		},
	}
	sanitized := s.SanitizeReport(report)
	if strings.Contains(sanitized.Diagnostics.Extension.Source, "mysecretpath") {
		t.Fatalf("extension source not redacted: %s", sanitized.Diagnostics.Extension.Source)
	}
}

func TestSanitizeReport_PreservesNonSensitiveData(t *testing.T) {
	t.Parallel()
	s := NewSanitizer(&fakeRedactor{secrets: []string{"secretvalue"}})
	report := IssueReport{
		Template: "bug",
		Title:    "Normal title",
		Diagnostics: DiagnosticData{
			Server: ServerDiag{
				Version:      "0.7.10",
				TotalCalls:   42,
				TotalErrors:  3,
				ErrorRatePct: 7.1,
			},
			Extension: ExtensionDiag{Connected: true},
			Platform:  PlatformDiag{OS: "darwin", Arch: "arm64", GoVersion: "go1.22"},
			Buffers:   BufferStats{ConsoleEntries: 10, NetworkEntries: 5},
		},
	}
	sanitized := s.SanitizeReport(report)
	if sanitized.Template != "bug" {
		t.Errorf("template changed: %s", sanitized.Template)
	}
	if sanitized.Title != "Normal title" {
		t.Errorf("title changed: %s", sanitized.Title)
	}
	if sanitized.Diagnostics.Server.Version != "0.7.10" {
		t.Errorf("version changed: %s", sanitized.Diagnostics.Server.Version)
	}
	if sanitized.Diagnostics.Server.TotalCalls != 42 {
		t.Errorf("total_calls changed: %d", sanitized.Diagnostics.Server.TotalCalls)
	}
	if sanitized.Diagnostics.Extension.Connected != true {
		t.Error("extension connected changed")
	}
	if sanitized.Diagnostics.Platform.OS != "darwin" {
		t.Errorf("platform OS changed: %s", sanitized.Diagnostics.Platform.OS)
	}
}

func TestSanitizeReport_DoesNotMutateOriginal(t *testing.T) {
	t.Parallel()
	s := NewSanitizer(&fakeRedactor{secrets: []string{"secret123"}})
	report := IssueReport{
		Title:       "Contains secret123",
		UserContext: "Also secret123",
	}
	_ = s.SanitizeReport(report)
	if !strings.Contains(report.Title, "secret123") {
		t.Fatal("original report was mutated")
	}
	if !strings.Contains(report.UserContext, "secret123") {
		t.Fatal("original report user_context was mutated")
	}
}
