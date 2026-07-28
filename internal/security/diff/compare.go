// compare.go — Computes regressions and improvements between two snapshots.
// Purpose: Computes regressions and improvements between security snapshots.
// Why: Isolates comparison logic so security findings remain deterministic and maintainable.
// Docs: docs/features/feature/security-hardening/index.md

package diff

import (
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

// Compare computes regressions/improvements between two snapshots.
//
// Failure semantics:
// - Missing/expired snapshot references return errors rather than partial comparisons.
func (m *Manager) Compare(fromName, toName string, currentBodies []types.NetworkBody) (*Result, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	fromSnapshot, err := m.resolveSnapshot(fromName)
	if err != nil {
		return nil, err
	}

	toSnapshot, err := m.resolveToSnapshot(toName, currentBodies)
	if err != nil {
		return nil, err
	}

	regressions, improvements := m.collectAllChanges(fromSnapshot, toSnapshot)
	verdict := determineVerdict(regressions, improvements)
	summary := buildSummary(regressions, improvements)

	return &Result{
		Verdict:      verdict,
		Regressions:  regressions,
		Improvements: improvements,
		Summary:      summary,
	}, nil
}

func (m *Manager) compareHeaders(from, to *Snapshot) ([]Change, []Change) {
	var regressions, improvements []Change
	origins := collectMapKeys(from.Headers, to.Headers)

	for origin := range origins {
		fromHeaders := from.Headers[origin]
		toHeaders := to.Headers[origin]
		if fromHeaders == nil {
			fromHeaders = make(map[string]string)
		}
		if toHeaders == nil {
			toHeaders = make(map[string]string)
		}

		reg, imp := diffHeadersForOrigin(origin, fromHeaders, toHeaders)
		regressions = append(regressions, reg...)
		improvements = append(improvements, imp...)
	}

	return regressions, improvements
}

func (m *Manager) compareCookies(from, to *Snapshot) ([]Change, []Change) {
	var regressions, improvements []Change
	origins := collectCookieMapKeys(from.Cookies, to.Cookies)

	for origin := range origins {
		fromMap := cookieSliceToMap(from.Cookies[origin])
		toMap := cookieSliceToMap(to.Cookies[origin])

		for name, fromCookie := range fromMap {
			toCookie, exists := toMap[name]
			if !exists {
				continue
			}
			reg, imp := diffCookieFlags(origin, name, fromCookie, toCookie)
			regressions = append(regressions, reg...)
			improvements = append(improvements, imp...)
		}
	}

	return regressions, improvements
}

func (m *Manager) compareAuth(from, to *Snapshot) ([]Change, []Change) {
	var regressions, improvements []Change
	endpoints := collectBoolMapKeys(from.Auth, to.Auth)

	for endpoint := range endpoints {
		fromAuth := from.Auth[endpoint]
		toAuth := to.Auth[endpoint]

		if fromAuth && !toAuth {
			regressions = append(regressions, Change{
				Category:       "auth",
				Severity:       "critical",
				Endpoint:       endpoint,
				Change:         "auth_removed",
				Before:         "authenticated",
				After:          "unauthenticated",
				Recommendation: "This endpoint previously required authentication but no longer does. Verify this is intentional.",
			})
		} else if !fromAuth && toAuth {
			improvements = append(improvements, Change{
				Category:       "auth",
				Severity:       "info",
				Endpoint:       endpoint,
				Change:         "auth_added",
				Before:         "unauthenticated",
				After:          "authenticated",
				Recommendation: "This endpoint now requires authentication.",
			})
		}
	}

	return regressions, improvements
}

func (m *Manager) compareTransport(from, to *Snapshot) ([]Change, []Change) {
	var regressions, improvements []Change

	fromByHost := normalizeTransportByHost(from.Transport)
	toByHost := normalizeTransportByHost(to.Transport)
	hosts := collectStringMapKeys(fromByHost, toByHost)

	for host := range hosts {
		fromScheme := fromByHost[host]
		toScheme := toByHost[host]

		if fromScheme == "https" && toScheme == "http" {
			regressions = append(regressions, Change{
				Category:       "transport",
				Severity:       "high",
				Origin:         host,
				Change:         "transport_downgrade",
				Before:         "https",
				After:          "http",
				Recommendation: "Origin downgraded from HTTPS to HTTP. Data in transit can be intercepted.",
			})
		} else if fromScheme == "http" && toScheme == "https" {
			improvements = append(improvements, Change{
				Category:       "transport",
				Severity:       "info",
				Origin:         host,
				Change:         "transport_upgrade",
				Before:         "http",
				After:          "https",
				Recommendation: "Origin upgraded from HTTP to HTTPS.",
			})
		}
	}

	return regressions, improvements
}

func (m *Manager) collectAllChanges(from, to *Snapshot) ([]Change, []Change) {
	var regressions, improvements []Change

	compareFns := []func(*Snapshot, *Snapshot) ([]Change, []Change){
		m.compareHeaders,
		m.compareCookies,
		m.compareAuth,
		m.compareTransport,
	}
	for _, compareFn := range compareFns {
		reg, imp := compareFn(from, to)
		regressions = append(regressions, reg...)
		improvements = append(improvements, imp...)
	}

	return regressions, improvements
}

func determineVerdict(regressions, improvements []Change) string {
	if len(regressions) > 0 {
		return "regressed"
	}
	if len(improvements) > 0 {
		return "improved"
	}
	return "unchanged"
}
