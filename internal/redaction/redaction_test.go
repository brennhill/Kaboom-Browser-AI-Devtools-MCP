// redaction_test.go — Tests built-in secret and PII patterns plus Luhn checks.
// Docs: docs/features/feature/redaction-patterns/index.md

package redaction

import (
	"strings"
	"testing"
)

// ============================================
// Built-in Pattern Tests
// ============================================

func TestRedactBearerToken(t *testing.T) {
	t.Parallel()
	engine := NewRedactionEngine("")
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "standard bearer token",
			input: `Authorization: Bearer eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.payload.sig`,
			want:  `Authorization: [REDACTED:bearer-token]`,
		},
		{
			name:  "bearer in JSON",
			input: `{"token": "Bearer abc123def456-._~+/="}`,
			want:  `{"token": "[REDACTED:bearer-token]"}`,
		},
		{
			name:  "case insensitive bearer with repeated horizontal whitespace",
			input: "authorization: bEaReR\t  abc123def456",
			want:  "authorization: [REDACTED:bearer-token]",
		},
		{
			name:  "no bearer keyword",
			input: `Authorization: Basic dXNlcjpwYXNz`,
			want:  `Authorization: [REDACTED:basic-auth]`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := engine.Redact(tt.input)
			if got != tt.want {
				t.Errorf("Redact() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRedactAWSKeys(t *testing.T) {
	t.Parallel()
	engine := NewRedactionEngine("")
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "AWS access key ID",
			input: `aws_access_key_id = AKIAIOSFODNN7EXAMPLE`,
			want:  `aws_access_key_id = [REDACTED:aws-key]`,
		},
		{
			name:  "AWS key in environment variable",
			input: `AWS_ACCESS_KEY_ID=AKIAI44QH8DHBEXAMPLE`,
			want:  `AWS_ACCESS_KEY_ID=[REDACTED:aws-key]`,
		},
		{
			name:  "not an AWS key (too short)",
			input: `AKIA1234`,
			want:  `AKIA1234`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := engine.Redact(tt.input)
			if got != tt.want {
				t.Errorf("Redact() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRedactJWT(t *testing.T) {
	t.Parallel()
	engine := NewRedactionEngine("")
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "standard JWT",
			input: `token: eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.dozjgNryP4J3jVmNHl0w5N_XgL0n3I9PlFUP0THsR8U`,
			want:  `token: [REDACTED:jwt]`,
		},
		{
			name:  "JWT with URL-safe base64",
			input: `eyJhbGciOiJSUzI1NiJ9.eyJpc3MiOiJqb2UiLCJleHAiOjEzMDA4MTkzODB9.abc_def-ghi`,
			want:  `[REDACTED:jwt]`,
		},
		{
			name:  "not a JWT (missing parts)",
			input: `eyJhbGciOiJIUzI1NiJ9.notavalidjwt`,
			want:  `eyJhbGciOiJIUzI1NiJ9.notavalidjwt`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := engine.Redact(tt.input)
			if got != tt.want {
				t.Errorf("Redact() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRedactGitHubPAT(t *testing.T) {
	t.Parallel()
	engine := NewRedactionEngine("")
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "GitHub personal access token (classic)",
			input: `GITHUB_TOKEN=ghp_ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghij`,
			want:  `GITHUB_TOKEN=[REDACTED:github-pat]`,
		},
		{
			name:  "GitHub fine-grained PAT",
			input: `token: github_pat_1234567890ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghij1234567890abcdefghijklmnopq`,
			want:  `token: [REDACTED:github-pat]`,
		},
		{
			name:  "not a GitHub PAT",
			input: `ghp_short`,
			want:  `ghp_short`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := engine.Redact(tt.input)
			if got != tt.want {
				t.Errorf("Redact() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRedactPrivateKey(t *testing.T) {
	t.Parallel()
	engine := NewRedactionEngine("")

	input := `Here is my key:
-----BEGIN RSA PRIVATE KEY-----
MIIEpAIBAAKCAQEA0Z3VS5JJcds3xfn/yGmDq2sNDG8K
-----END RSA PRIVATE KEY-----
done`

	got := engine.Redact(input)
	if !strings.Contains(got, "[REDACTED:private-key]") {
		t.Errorf("Expected private key to be redacted, got: %q", got)
	}
	if strings.Contains(got, "MIIEpAIBAAKCAQEA") {
		t.Errorf("Private key content should not be present in output")
	}
}

func TestRedactCreditCard(t *testing.T) {
	t.Parallel()
	engine := NewRedactionEngine("")
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "visa card with spaces",
			input: `card: 4111 1111 1111 1111`,
			want:  `card: [REDACTED:credit-card]`,
		},
		{
			name:  "visa card with dashes",
			input: `card: 4111-1111-1111-1111`,
			want:  `card: [REDACTED:credit-card]`,
		},
		{
			name:  "visa card no separators",
			input: `card: 4111111111111111`,
			want:  `card: [REDACTED:credit-card]`,
		},
		{
			name:  "not a valid card (fails Luhn)",
			input: `number: 1234567890123456`,
			want:  `number: 1234567890123456`,
		},
		{
			name:  "too short",
			input: `4111 1111 1111`,
			want:  `4111 1111 1111`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := engine.Redact(tt.input)
			if got != tt.want {
				t.Errorf("Redact() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRedactSSN(t *testing.T) {
	t.Parallel()
	engine := NewRedactionEngine("")
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "standard SSN",
			input: `ssn: 123-45-6789`,
			want:  `ssn: [REDACTED:ssn]`,
		},
		{
			name:  "SSN in text",
			input: `Patient SSN is 078-05-1120 on file`,
			want:  `Patient SSN is [REDACTED:ssn] on file`,
		},
		{
			name:  "not an SSN (no dashes)",
			input: `123456789`,
			want:  `123456789`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := engine.Redact(tt.input)
			if got != tt.want {
				t.Errorf("Redact() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRedactAPIKey(t *testing.T) {
	t.Parallel()
	engine := NewRedactionEngine("")
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "api_key in header",
			input: `api_key: sk-1234567890abcdef`,
			want:  `[REDACTED:api-key]`,
		},
		{
			name:  "API-Key format",
			input: `API-Key = my_secret_key_value`,
			want:  `[REDACTED:api-key]`,
		},
		{
			name:  "secret_key assignment",
			input: `secret_key=super_secret_123`,
			want:  `[REDACTED:api-key]`,
		},
		{
			name:  "apikey single word",
			input: `apikey: abcdef123456`,
			want:  `[REDACTED:api-key]`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := engine.Redact(tt.input)
			if got != tt.want {
				t.Errorf("Redact() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRedactBasicAuth(t *testing.T) {
	t.Parallel()
	engine := NewRedactionEngine("")
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "basic auth header",
			input: `Authorization: Basic dXNlcjpwYXNzd29yZA==`,
			want:  `Authorization: [REDACTED:basic-auth]`,
		},
		{
			name:  "basic auth in JSON",
			input: `{"auth": "Basic YWRtaW46c2VjcmV0"}`,
			want:  `{"auth": "[REDACTED:basic-auth]"}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := engine.Redact(tt.input)
			if got != tt.want {
				t.Errorf("Redact() = %q, want %q", got, tt.want)
			}
		})
	}
}

// ============================================
// Luhn Validation Tests
// ============================================

func TestLuhnValidation(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		input string
		valid bool
	}{
		{"valid visa", "4111111111111111", true},
		{"valid mastercard", "5500000000000004", true},
		{"valid amex (15-digit, Luhn valid)", "378282246310005", true}, // Luhn validates 13-19 digits
		{"invalid number", "1234567890123456", false},
		{"all zeros", "0000000000000000", true}, // Luhn valid but unlikely real
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := luhnValid(tt.input)
			if got != tt.valid {
				t.Errorf("luhnValid(%q) = %v, want %v", tt.input, got, tt.valid)
			}
		})
	}
}

// ============================================
// Session Cookie / Token Tests
// ============================================

func TestRedactSessionCookie(t *testing.T) {
	t.Parallel()
	engine := NewRedactionEngine("")
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "session cookie",
			input: `Cookie: session=abcdef1234567890ABCDEF`,
			want:  `Cookie: [REDACTED:session-cookie]`,
		},
		{
			name:  "sid value",
			input: `sid=A1B2C3D4E5F6G7H8I9J0K1L2M3N4O5P6`,
			want:  `[REDACTED:session-cookie]`,
		},
		{
			name:  "token assignment",
			input: `token = eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9abcdef`,
			want:  `[REDACTED:session-cookie]`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := engine.Redact(tt.input)
			if got != tt.want {
				t.Errorf("Redact() = %q, want %q", got, tt.want)
			}
		})
	}
}
