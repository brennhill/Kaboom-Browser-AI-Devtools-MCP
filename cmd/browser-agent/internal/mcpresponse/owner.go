// owner.go — Applies ordered MCP response warnings and lifecycle metadata.

package mcpresponse

import (
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/semver"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/appruntime"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture/syncruntime"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/diag"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

const updateWarningCooldown = 24 * time.Hour

// Config supplies lifecycle state and host-owned warning queues.
type Config struct {
	Runtime       *appruntime.Runtime
	AddWarning    func(string)
	DrainWarnings func() []string
	PendingAudit  func() bool
	Now           func() time.Time
}

// Owner applies all non-telemetry MCP response warnings in canonical order.
type Owner struct {
	config   Config
	captured *capture.Capture
}

// New constructs an MCP response policy owner.
func New(config Config) *Owner {
	if config.Runtime == nil {
		config.Runtime = appruntime.New("dev")
	}
	if config.AddWarning == nil {
		config.AddWarning = func(value string) { diag.Printf("[Kaboom] %s\n", value) }
	}
	if config.DrainWarnings == nil {
		config.DrainWarnings = func() []string { return nil }
	}
	if config.PendingAudit == nil {
		config.PendingAudit = func() bool { return false }
	}
	if config.Now == nil {
		config.Now = time.Now
	}
	return &Owner{config: config}
}

// SetCapture installs the capture source during one-time backend composition.
func (o *Owner) SetCapture(captured *capture.Capture) {
	o.captured = captured
}

// WarnUnknownArguments queues stable warnings for schema-unknown tool fields.
func (o *Owner) WarnUnknownArguments(toolName string, arguments json.RawMessage, schemas []mcp.MCPTool) {
	if len(arguments) == 0 {
		return
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(arguments, &raw); err != nil || len(raw) == 0 {
		return // EXPECTED_ABSENCE: malformed tool arguments are rejected by the owning tool validator.
	}
	allowed := allowedArgumentKeys(toolName, schemas)
	if len(allowed) == 0 {
		return
	}
	unknown := make([]string, 0)
	for key := range raw {
		if _, found := allowed[key]; !found {
			unknown = append(unknown, key)
		}
	}
	sort.Strings(unknown)
	for _, key := range unknown {
		o.config.AddWarning(fmt.Sprintf("unknown parameter '%s' for tool '%s' (ignored)", key, toolName))
	}
}

// Augment applies queued, security, version, upgrade, and pending-intent warnings.
func (o *Owner) Augment(response mcp.JSONRPCResponse, executorConfigured bool) mcp.JSONRPCResponse {
	response = mcp.AppendWarningsToResponse(response, o.config.DrainWarnings())
	response = o.addSecurityModeWarning(response, executorConfigured)
	response = o.addVersionWarning(response, executorConfigured)
	response = o.addUpdateWarning(response)
	response = o.addUpgradeWarning(response)
	return o.addPendingAuditWarning(response)
}

func allowedArgumentKeys(toolName string, schemas []mcp.MCPTool) map[string]struct{} {
	for _, tool := range schemas {
		if tool.Name != toolName {
			continue
		}
		keys := make(map[string]struct{})
		properties, ok := tool.InputSchema["properties"].(map[string]any)
		if !ok {
			return keys
		}
		for key := range properties {
			keys[key] = struct{}{}
		}
		return keys
	}
	return nil
}

func (o *Owner) addPendingAuditWarning(response mcp.JSONRPCResponse) mcp.JSONRPCResponse {
	if response.Result == nil || !o.config.PendingAudit() {
		return response
	}
	return mcp.PrependWarningToResponse(response, "ACTION REQUIRED: The user clicked 'Audit' in the browser. "+
		"Run the Kaboom audit workflow (/kaboom/audit or /audit fallback) for a full six-lane report.\n\n")
}

func (o *Owner) addSecurityModeWarning(response mcp.JSONRPCResponse, executorConfigured bool) mcp.JSONRPCResponse {
	if !executorConfigured || response.Result == nil || o.captured == nil {
		return response
	}
	mode, productionParity, rewrites := o.captured.Extension().GetSecurityMode()
	if mode == syncruntime.SecurityModeNormal {
		return response
	}
	response = mcp.PrependWarningToResponse(response, "[ALTERED ENVIRONMENT] security_mode=insecure_proxy; production_parity=false. CSP headers are rewritten for debugging.\n\n")
	return mcp.MutateToolResult(response, func(result *mcp.MCPToolResult) {
		if result.Metadata == nil {
			result.Metadata = make(map[string]any)
		}
		result.Metadata["security_mode"] = mode
		result.Metadata["production_parity"] = productionParity
		result.Metadata["insecure_rewrites_applied"] = rewrites
	})
}

func (o *Owner) addVersionWarning(response mcp.JSONRPCResponse, executorConfigured bool) mcp.JSONRPCResponse {
	if !executorConfigured || response.Result == nil || o.captured == nil {
		return response
	}
	extensionVersion, serverVersion, mismatch := o.captured.Extension().VersionMismatch()
	if !mismatch {
		return response
	}
	return mcp.PrependWarningToResponse(response, fmt.Sprintf(
		"WARNING: Version mismatch detected — server v%s, extension v%s. Update your extension to avoid issues.\n\n",
		serverVersion, extensionVersion,
	))
}

func (o *Owner) addUpdateWarning(response mcp.JSONRPCResponse) mcp.JSONRPCResponse {
	if response.Result == nil {
		return response
	}
	if upgrade := o.config.Runtime.Upgrade(); upgrade != nil {
		if pending, _, _ := upgrade.UpgradeInfo(); pending {
			return response
		}
	}
	available := o.config.Runtime.ReleaseChecker().Available()
	if available == "" || !semver.IsNewer(available, o.config.Runtime.Version()) {
		return response
	}
	if !o.config.Runtime.ClaimUpdateWarning(o.config.Now(), updateWarningCooldown) {
		return response
	}
	return mcp.PrependWarningToResponse(response, fmt.Sprintf(
		"UPDATE AVAILABLE: Kaboom v%s is available (current: v%s). Run: npm install -g kaboom-agentic-browser@latest\n\n",
		available, o.config.Runtime.Version(),
	))
}

func (o *Owner) addUpgradeWarning(response mcp.JSONRPCResponse) mcp.JSONRPCResponse {
	upgrade := o.config.Runtime.Upgrade()
	if upgrade == nil || response.Result == nil {
		return response
	}
	pending, newVersion, detectedAt := upgrade.UpgradeInfo()
	if !pending {
		return response
	}
	elapsed := o.config.Now().Sub(detectedAt).Truncate(time.Second)
	return mcp.PrependWarningToResponse(response, fmt.Sprintf(
		"NOTICE: Kaboom v%s detected on disk (current: v%s, detected %s ago). Auto-restart imminent. Your next tool call will use the new version.\n\n",
		newVersion, o.config.Runtime.Version(), elapsed,
	))
}
