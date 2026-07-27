// runtime.go — Shared MCP push capability, framing, and outbound sender state.

package pushapi

import (
	"encoding/json"
	"sync"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/bridge"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/push"
)

// PayloadWriter writes one complete MCP payload using the selected framing.
type PayloadWriter func(payload []byte, framing bridge.StdioFraming)

// Runtime owns process-wide push negotiation and outbound MCP delivery state.
type Runtime struct {
	mu       sync.RWMutex
	caps     push.ClientCapabilities
	framing  bridge.StdioFraming
	onChange func(push.ClientCapabilities)
	write    PayloadWriter
}

// NewRuntime creates a push runtime using the canonical MCP payload writer.
func NewRuntime(write PayloadWriter) *Runtime {
	return &Runtime{write: write}
}

// SetCapabilities updates negotiated capabilities and notifies the active router.
func (runtime *Runtime) SetCapabilities(caps push.ClientCapabilities) {
	callback := func() func(push.ClientCapabilities) {
		runtime.mu.Lock()
		defer runtime.mu.Unlock()
		runtime.caps = caps
		return runtime.onChange
	}()
	if callback != nil {
		callback(caps)
	}
}

// Capabilities returns the negotiated client capabilities.
func (runtime *Runtime) Capabilities() push.ClientCapabilities {
	runtime.mu.RLock()
	defer runtime.mu.RUnlock()
	return runtime.caps
}

// OnCapabilitiesChange installs the active router synchronization callback.
func (runtime *Runtime) OnCapabilitiesChange(callback func(push.ClientCapabilities)) {
	runtime.mu.Lock()
	defer runtime.mu.Unlock()
	runtime.onChange = callback
}

// StoreFraming records the framing negotiated by the bridge.
func (runtime *Runtime) StoreFraming(framing bridge.StdioFraming) {
	runtime.mu.Lock()
	defer runtime.mu.Unlock()
	runtime.framing = framing
}

// Framing returns the current bridge framing.
func (runtime *Runtime) Framing() bridge.StdioFraming {
	runtime.mu.RLock()
	defer runtime.mu.RUnlock()
	return runtime.framing
}

// ExtractClientCapabilities parses MCP initialize parameters.
func ExtractClientCapabilities(rawParams json.RawMessage) push.ClientCapabilities {
	if len(rawParams) == 0 {
		return push.ClientCapabilities{}
	}
	var params struct {
		Capabilities struct {
			Sampling json.RawMessage `json:"sampling"`
		} `json:"capabilities"`
		ClientInfo struct {
			Name string `json:"name"`
		} `json:"clientInfo"` // SPEC:MCP initialize params
	}
	if err := json.Unmarshal(rawParams, &params); err != nil {
		return push.ClientCapabilities{}
	}
	caps := push.ClientCapabilities{ClientName: params.ClientInfo.Name}
	caps.SupportsSampling = len(params.Capabilities.Sampling) > 0 && string(params.Capabilities.Sampling) != "null"
	caps.SupportsNotifications = caps.ClientName != ""
	return caps
}

// SendSampling implements push.SamplingSender.
func (runtime *Runtime) SendSampling(request push.SamplingRequest) error {
	payload, err := json.Marshal(request)
	if err != nil {
		return err
	}
	runtime.write(payload, runtime.Framing())
	return nil
}

// SendNotification implements push.Notifier.
func (runtime *Runtime) SendNotification(method string, params map[string]any) {
	payload, err := json.Marshal(map[string]any{
		"jsonrpc": mcp.JSONRPCVersion,
		"method":  method,
		"params":  params,
	})
	if err == nil {
		runtime.write(payload, runtime.Framing())
	}
}
