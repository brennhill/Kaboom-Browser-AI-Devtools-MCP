// relay_test.go — Verifies daemon push draining and MCP relay behavior.

package pushrelay

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	internbridge "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/bridge"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/push"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) { return fn(request) }

func TestDrainOnceRelaysEveryPendingEvent(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"events": []push.PushEvent{{ID: "1", Type: "chat"}, {ID: "2", Type: "screenshot"}},
			"count":  2,
		})
	}))
	defer server.Close()

	var payloads [][]byte
	relay := New(server.Client(), server.URL, Deps{
		Framing: func() internbridge.StdioFraming { return internbridge.StdioFramingLine },
		Write: func(payload []byte, _ internbridge.StdioFraming) {
			payloads = append(payloads, append([]byte(nil), payload...))
		},
		Debugf: func(string, ...any) {},
	})
	relay.DrainOnce(t.Context())

	if len(payloads) != 2 {
		t.Fatalf("relayed payload count = %d, want 2", len(payloads))
	}
	for _, payload := range payloads {
		if !strings.Contains(string(payload), "sampling/createMessage") {
			t.Fatalf("payload = %q, want sampling/createMessage", payload)
		}
	}
}

func TestDrainOnceDiagnosesMalformedResponse(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("{not-json"))
	}))
	defer server.Close()

	var diagnostic string
	relay := New(server.Client(), server.URL, Deps{
		Framing: func() internbridge.StdioFraming { return internbridge.StdioFramingLine },
		Write:   func([]byte, internbridge.StdioFraming) {},
		Debugf:  func(format string, args ...any) { diagnostic = format },
	})
	relay.DrainOnce(t.Context())
	if diagnostic == "" {
		t.Fatal("malformed daemon response was silently discarded")
	}
}

func TestDrainOnceTreatsEmptyAndUnavailableInboxesAsExpectedAbsence(t *testing.T) {
	t.Parallel()
	for _, status := range []int{http.StatusOK, http.StatusServiceUnavailable} {
		status := status
		t.Run(http.StatusText(status), func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(status)
				if status == http.StatusOK {
					_ = json.NewEncoder(w).Encode(map[string]any{"events": []push.PushEvent{}, "count": 0})
				}
			}))
			defer server.Close()
			writes := 0
			relay := New(server.Client(), server.URL, Deps{
				Framing: func() internbridge.StdioFraming { return internbridge.StdioFramingLine },
				Write:   func([]byte, internbridge.StdioFraming) { writes++ },
				Debugf:  func(string, ...any) {},
			})
			relay.DrainOnce(t.Context())
			if writes != 0 {
				t.Fatalf("writes = %d, want 0", writes)
			}
		})
	}

	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("daemon unavailable")
	})}
	relay := New(client, "http://daemon.test", Deps{
		Framing: func() internbridge.StdioFraming { return internbridge.StdioFramingLine },
		Write:   func([]byte, internbridge.StdioFraming) { t.Fatal("unexpected write") },
		Debugf:  func(string, ...any) {},
	})
	relay.DrainOnce(t.Context())
}
