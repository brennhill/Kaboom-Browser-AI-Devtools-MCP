// Purpose: Applies session-scoped whitelist overrides and integrates CSP generation with MCP tools.
// Why: Separates tool integration and override application from core CSP generation.
package security

import (
	"encoding/json"
	"fmt"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/security/policy"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/util"
)

// applyWhitelistOverrides appends session-scoped manual overrides to default-src.
//
// Failure semantics:
// - Overrides affect generated output only; no persistent config is mutated.
func (g *CSPGenerator) applyWhitelistOverrides(response *CSPResponse, overrides []string) {
	if response.Directives["default-src"] == nil {
		response.Directives["default-src"] = []string{"'self'"}
	}
	response.Directives["default-src"] = append(response.Directives["default-src"], overrides...)

	response.CSPHeader = g.buildCSPHeader(response.Directives)
	response.MetaTag = formatMetaTag(response.CSPHeader)

	for _, origin := range overrides {
		response.Warnings = append(response.Warnings, fmt.Sprintf(
			"⚠️  SECURITY: Temporary whitelist override applied (SESSION-ONLY)\n"+
				"   Origin: %s\n"+
				"   Source: MCP tool parameter\n"+
				"   Action: Review origin legitimacy before permanent whitelist\n"+
				"\n"+
				"💡 To permanently whitelist (after human review):\n"+
				"   1. Verify origin is legitimate and trusted\n"+
				"   2. %s\n"+
				"   3. Add to 'whitelisted_origins' array",
			origin,
			policy.EditInstruction(),
		))
	}

	response.Audit = &CSPAudit{
		SessionOverrides:    overrides,
		PersistentWhitelist: []string{},
		OverrideSource:      "mcp_tool_parameter",
	}

	for _, origin := range overrides {
		policy.LogEvent(policy.AuditEvent{
			Action:     "whitelist_override",
			Origin:     origin,
			Reason:     "CSP generation with session-only override",
			Persistent: false,
			Source:     "mcp",
		})
	}
}

// RecordOriginFromBody extracts origin and resource type from a NetworkBody
// and records it in the origin accumulator. Called from the network ingestion path.
func (g *CSPGenerator) RecordOriginFromBody(body capture.NetworkBody, pageURL string) {
	origin := util.ExtractOrigin(body.URL)
	if origin == "" {
		return
	}
	resourceType := contentTypeToResourceType(body.ContentType)
	g.RecordOrigin(origin, resourceType, pageURL)
}

// HandleGenerateCSP is the MCP tool handler for generate_csp.
func (g *CSPGenerator) HandleGenerateCSP(params json.RawMessage) (any, error) {
	var cspParams CSPParams
	if len(params) > 0 {
		if err := json.Unmarshal(params, &cspParams); err != nil {
			return nil, fmt.Errorf("invalid CSP parameters: %w", err)
		}
	}

	resp := g.GenerateCSP(cspParams)
	return &resp, nil
}
