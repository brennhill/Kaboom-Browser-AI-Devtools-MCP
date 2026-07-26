// Purpose: Builds aggregate third-party risk summaries and remediation recommendations.
// Why: Converts detailed third-party findings into prioritized, actionable guidance.
// Docs: docs/features/feature/enterprise-audit/index.md

package thirdparty

import (
	"fmt"
	"strings"
)

// buildThirdPartySummary computes aggregate counts from entries.
func buildThirdPartySummary(entries []ThirdPartyEntry) ThirdPartySummary {
	s := ThirdPartySummary{
		TotalThirdParties: len(entries),
	}
	for _, e := range entries {
		switch e.RiskLevel {
		case "critical":
			s.CriticalRisk++
		case "high":
			s.HighRisk++
		case "medium":
			s.MediumRisk++
		case "low":
			s.LowRisk++
		}
		if e.Resources.Scripts > 0 {
			s.ScriptsFromThirdParty++
		}
		if e.DataOutbound {
			s.OriginsReceivingData++
		}
		if e.SetsCookies {
			s.OriginsSettingCookies++
		}
		if e.Reputation.Classification == "suspicious" {
			s.SuspiciousOrigins++
		}
	}
	return s
}

// buildRecommendations generates actionable recommendation strings.
func buildRecommendations(entries []ThirdPartyEntry) []string {
	var recs []string
	recs = append(recs, suspiciousScriptRecs(entries)...)
	recs = append(recs, dataReceiverRecs(entries)...)
	recs = append(recs, piiOutboundRecs(entries)...)
	recs = append(recs, cookieSetterRecs(entries)...)
	return recs
}

// suspiciousScriptRecs returns recommendations for suspicious origins loading scripts.
func suspiciousScriptRecs(entries []ThirdPartyEntry) []string {
	var recs []string
	for _, e := range entries {
		if e.Reputation.Classification == "suspicious" && e.Resources.Scripts > 0 {
			recs = append(recs, fmt.Sprintf("CRITICAL: %s loads scripts AND is flagged suspicious — investigate immediately", e.Origin))
		}
	}
	return recs
}

// dataReceiverRecs returns a recommendation if any origins receive outbound data.
func dataReceiverRecs(entries []ThirdPartyEntry) []string {
	count := countEntries(entries, func(e ThirdPartyEntry) bool { return e.DataOutbound })
	if count == 0 {
		return nil
	}
	return []string{fmt.Sprintf("%d origin(s) receive user data — verify these are intentional", count)}
}

// piiOutboundRecs returns recommendations for origins receiving PII data.
func piiOutboundRecs(entries []ThirdPartyEntry) []string {
	var recs []string
	for _, e := range entries {
		if e.OutboundDetails != nil && len(e.OutboundDetails.PIIFields) > 0 {
			recs = append(recs, fmt.Sprintf("%s receives PII fields (%s) — ensure privacy policy covers this",
				e.Origin, strings.Join(e.OutboundDetails.PIIFields, ", ")))
		}
	}
	return recs
}

// cookieSetterRecs returns a recommendation if third parties set cookies.
func cookieSetterRecs(entries []ThirdPartyEntry) []string {
	count := countEntries(entries, func(e ThirdPartyEntry) bool { return e.SetsCookies })
	if count == 0 {
		return nil
	}
	return []string{fmt.Sprintf("%d third-party origin(s) set cookies — review for GDPR/CCPA compliance", count)}
}

// countEntries counts entries matching a predicate.
func countEntries(entries []ThirdPartyEntry, pred func(ThirdPartyEntry) bool) int {
	n := 0
	for _, e := range entries {
		if pred(e) {
			n++
		}
	}
	return n
}

// riskOrder returns a sort order for risk levels (lower = more severe).
func riskOrder(level string) int {
	switch level {
	case "critical":
		return 0
	case "high":
		return 1
	case "medium":
		return 2
	case "low":
		return 3
	default:
		return 4
	}
}
