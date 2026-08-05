// relay.go — Drains daemon push events and emits them over the active MCP transport.

package pushrelay

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	internbridge "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/bridge"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/push"
)

const (
	pollInterval = 500 * time.Millisecond
	pollTimeout  = 3 * time.Second
)

// Deps are the MCP transport collaborators used by a relay.
type Deps struct {
	Framing func() internbridge.StdioFraming
	Write   func([]byte, internbridge.StdioFraming)
	Debugf  func(string, ...any)
}

// Relay owns polling and delivery for one bridge session.
type Relay struct {
	client   *http.Client
	endpoint string
	deps     Deps
}

// New constructs a relay for one daemon endpoint.
func New(client *http.Client, endpoint string, deps Deps) *Relay {
	return &Relay{client: client, endpoint: endpoint, deps: deps}
}

// Start polls until the bridge session closes.
func (r *Relay) Start(done <-chan struct{}) {
	go func() { // lint:allow-bare-goroutine — lifecycle-tied to done channel.
		ticker := time.NewTicker(pollInterval)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				r.DrainOnce(context.Background())
			}
		}
	}()
}

// DrainOnce fetches and relays the daemon's currently pending push events.
func (r *Relay) DrainOnce(parent context.Context) {
	ctx, cancel := context.WithTimeout(parent, pollTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, r.endpoint+"/push/drain", nil)
	if err != nil {
		r.deps.Debugf("push relay: build drain request: %v", err)
		return
	}
	resp, err := r.client.Do(req)
	if err != nil {
		// EXPECTED_ABSENCE: daemon handoff and shutdown briefly leave no listener;
		// the next bounded poll retries, so logging every miss would obscure incidents.
		return
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			r.deps.Debugf("push relay: close drain response: %v", closeErr)
		}
	}()
	if resp.StatusCode != http.StatusOK {
		// EXPECTED_ABSENCE: the endpoint may be unavailable across mixed-version
		// daemon handoff; polling retries after the canonical daemon is ready.
		return
	}
	var drain struct {
		Events []push.PushEvent `json:"events"`
		Count  int              `json:"count"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&drain); err != nil {
		r.deps.Debugf("push relay: decode drain response: %v", err)
		return
	}
	if drain.Count == 0 {
		return
	}
	for i := range drain.Events {
		payload, err := json.Marshal(push.BuildSamplingRequest(drain.Events[i]))
		if err != nil {
			r.deps.Debugf("push relay: encode %s event: %v", drain.Events[i].Type, err)
			continue
		}
		r.deps.Write(payload, r.deps.Framing())
		r.deps.Debugf("push relay: sent %s event", drain.Events[i].Type)
	}
}
