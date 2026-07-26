// Purpose: Implements third-party origin auditing over captured network traffic and request metadata.
// Why: Surfaces supply-chain and data-exfiltration risks introduced by non-first-party dependencies.
// Docs: docs/features/feature/enterprise-audit/index.md

// Package thirdparty audits non-first-party origins seen in captured network traffic.
//
// Requests are grouped by origin, classified by what they carry (scripts, cookies,
// outbound data including detected PII fields), scored for reputation (known CDN,
// abuse TLD, DGA-like hostname entropy, user-supplied allow/deny lists) and rolled
// up into a risk-ranked summary with remediation recommendations.
package thirdparty

import (
	"encoding/json"
	"fmt"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/util"
)

// ThirdPartyAuditor performs third-party domain analysis.
type ThirdPartyAuditor struct{}

// ThirdPartyParams defines input for the audit tool.
type ThirdPartyParams struct {
	FirstPartyOrigins []string     `json:"first_party_origins"`
	IncludeStatic     *bool        `json:"include_static"`
	CustomLists       *CustomLists `json:"custom_lists"`
	CustomListsFile   string       `json:"custom_lists_file"`
}

// CustomLists defines enterprise custom domain lists.
type CustomLists struct {
	Allowed  []string `json:"allowed"`
	Blocked  []string `json:"blocked"`
	Internal []string `json:"internal"`
}

// ThirdPartyResult is the full audit response.
type ThirdPartyResult struct {
	FirstPartyOrigin string            `json:"first_party_origin"`
	ThirdParties     []ThirdPartyEntry `json:"third_parties"`
	Summary          ThirdPartySummary `json:"summary"`
	Recommendations  []string          `json:"recommendations"`
}

// ThirdPartyEntry describes a single third-party origin.
type ThirdPartyEntry struct {
	Origin          string           `json:"origin"`
	RiskLevel       string           `json:"risk_level"`
	RiskReason      string           `json:"risk_reason"`
	Resources       ResourceCounts   `json:"resources"`
	DataOutbound    bool             `json:"data_outbound"`
	OutboundDetails *OutboundDetails `json:"outbound_details,omitempty"`
	SetsCookies     bool             `json:"sets_cookies"`
	RequestCount    int              `json:"request_count"`
	TotalBytes      int64            `json:"total_transfer_bytes"`
	URLs            []string         `json:"urls"`
	Reputation      DomainReputation `json:"reputation"`
}

// ResourceCounts tracks resource types loaded from an origin.
type ResourceCounts struct {
	Scripts int `json:"scripts"`
	Styles  int `json:"styles"`
	Fonts   int `json:"fonts"`
	Images  int `json:"images"`
	Other   int `json:"other"`
}

// OutboundDetails describes data sent to a third party.
type OutboundDetails struct {
	Methods      []string `json:"methods"`
	ContentTypes []string `json:"content_types"`
	PIIFields    []string `json:"contains_pii_fields,omitempty"`
}

// DomainReputation is the reputation assessment for a domain.
type DomainReputation struct {
	Classification string   `json:"classification"` // known_cdn, suspicious, unknown, enterprise_allowed, enterprise_blocked
	Source         string   `json:"source,omitempty"`
	SuspicionFlags []string `json:"suspicion_flags,omitempty"`
	Notes          string   `json:"notes,omitempty"`
}

// ThirdPartySummary provides aggregate counts.
type ThirdPartySummary struct {
	TotalThirdParties     int `json:"total_third_parties"`
	CriticalRisk          int `json:"critical_risk"`
	HighRisk              int `json:"high_risk"`
	MediumRisk            int `json:"medium_risk"`
	LowRisk               int `json:"low_risk"`
	ScriptsFromThirdParty int `json:"scripts_from_third_parties"`
	OriginsReceivingData  int `json:"origins_receiving_data"`
	OriginsSettingCookies int `json:"origins_setting_cookies"`
	SuspiciousOrigins     int `json:"suspicious_origins"`
}

// NewThirdPartyAuditor creates a new ThirdPartyAuditor.
func NewThirdPartyAuditor() *ThirdPartyAuditor {
	return &ThirdPartyAuditor{}
}

// originData groups network bodies and URLs for a single origin.
type originData struct {
	bodies []capture.NetworkBody
	urls   []string
}

// Audit analyzes network bodies to identify and classify third-party origins.
func (a *ThirdPartyAuditor) Audit(bodies []capture.NetworkBody, pageURLs []string, params ThirdPartyParams) ThirdPartyResult {
	customLists := resolveCustomLists(params)
	firstPartyOrigins := buildFirstPartySet(params, pageURLs, customLists)

	originMap := groupByThirdPartyOrigin(bodies, firstPartyOrigins)
	entries := buildAllEntries(originMap, customLists)
	entries = filterAndSort(entries, params.IncludeStatic)

	primaryFirstParty := ""
	if len(pageURLs) > 0 {
		primaryFirstParty = util.ExtractOrigin(pageURLs[0])
	}

	return ThirdPartyResult{
		FirstPartyOrigin: primaryFirstParty,
		ThirdParties:     entries,
		Summary:          buildThirdPartySummary(entries),
		Recommendations:  buildRecommendations(entries),
	}
}

// HandleAuditThirdParties is the MCP handler for the audit_third_parties tool.
func HandleAuditThirdParties(params json.RawMessage, bodies []capture.NetworkBody, pageURLs []string) (any, error) {
	var p ThirdPartyParams
	if len(params) > 0 {
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, fmt.Errorf("invalid params: %w", err)
		}
	}
	auditor := NewThirdPartyAuditor()
	result := auditor.Audit(bodies, pageURLs, p)
	return result, nil
}
