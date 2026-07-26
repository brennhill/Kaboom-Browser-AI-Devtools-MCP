// helpers_headers_cookies.go — Header and cookie-flag change detection.
// Purpose: Compares security headers and cookies between snapshots to detect regressions.
// Why: Separates header/cookie diff logic from URL, transport, and summary diff helpers.
package diff

import "fmt"

func diffHeadersForOrigin(origin string, fromHeaders, toHeaders map[string]string) ([]Change, []Change) {
	var regressions, improvements []Change

	for _, hdr := range trackedHeaders {
		fromVal, fromHas := fromHeaders[hdr]
		toVal, toHas := toHeaders[hdr]

		if fromHas && !toHas {
			regressions = append(regressions, Change{
				Category:       "headers",
				Severity:       "warning",
				Origin:         origin,
				Change:         "header_removed",
				Header:         hdr,
				Before:         fromVal,
				After:          "",
				Recommendation: headerRemovedRecommendation(hdr),
			})
		} else if !fromHas && toHas {
			improvements = append(improvements, Change{
				Category:       "headers",
				Severity:       "info",
				Origin:         origin,
				Change:         "header_added",
				Header:         hdr,
				Before:         "",
				After:          toVal,
				Recommendation: fmt.Sprintf("%s header has been added, improving security posture.", hdr),
			})
		}
	}

	return regressions, improvements
}

type cookieFlagSpec struct {
	flagName   string
	fromActive bool
	toActive   bool
	fromVal    string
	toVal      string
	lostMsg    string
	gainedMsg  string
}

func diffCookieFlags(origin, name string, from, to Cookie) ([]Change, []Change) {
	var regressions, improvements []Change

	flags := []cookieFlagSpec{
		{
			flagName:   "HttpOnly",
			fromActive: from.HttpOnly,
			toActive:   to.HttpOnly,
			fromVal:    "present",
			toVal:      "present",
			lostMsg:    fmt.Sprintf("Cookie '%s' lost HttpOnly flag. Client-side JavaScript can now read it.", name),
			gainedMsg:  fmt.Sprintf("Cookie '%s' gained HttpOnly flag.", name),
		},
		{
			flagName:   "Secure",
			fromActive: from.Secure,
			toActive:   to.Secure,
			fromVal:    "present",
			toVal:      "present",
			lostMsg:    fmt.Sprintf("Cookie '%s' lost Secure flag. Cookie can now be sent over HTTP.", name),
			gainedMsg:  fmt.Sprintf("Cookie '%s' gained Secure flag.", name),
		},
		{
			flagName:   "SameSite",
			fromActive: from.SameSite != "",
			toActive:   to.SameSite != "",
			fromVal:    from.SameSite,
			toVal:      to.SameSite,
			lostMsg:    fmt.Sprintf("Cookie '%s' lost SameSite flag. Cookie may now be sent in cross-site requests.", name),
			gainedMsg:  fmt.Sprintf("Cookie '%s' gained SameSite flag.", name),
		},
	}

	for _, f := range flags {
		change := diffSingleCookieFlag(origin, name, f)
		if change == nil {
			continue
		}
		if change.Change == "flag_removed" {
			regressions = append(regressions, *change)
		} else {
			improvements = append(improvements, *change)
		}
	}

	return regressions, improvements
}

func diffSingleCookieFlag(origin, cookieName string, f cookieFlagSpec) *Change {
	if f.fromActive && !f.toActive {
		before := f.fromVal
		if f.flagName != "SameSite" {
			before = "present"
		}
		return &Change{
			Category: "cookies", Severity: "warning", Origin: origin,
			Change: "flag_removed", CookieName: cookieName, Flag: f.flagName,
			Before: before, After: flagAbsentValue(f.flagName, ""),
			Recommendation: f.lostMsg,
		}
	}
	if !f.fromActive && f.toActive {
		after := f.toVal
		if f.flagName != "SameSite" {
			after = "present"
		}
		return &Change{
			Category: "cookies", Severity: "info", Origin: origin,
			Change: "flag_added", CookieName: cookieName, Flag: f.flagName,
			Before: flagAbsentValue(f.flagName, ""), After: after,
			Recommendation: f.gainedMsg,
		}
	}
	return nil
}

func flagAbsentValue(flagName, fallback string) string {
	if flagName == "SameSite" {
		return fallback
	}
	return "absent"
}

func headerRemovedRecommendation(header string) string {
	if rec, ok := headerRemovedRecommendations[header]; ok {
		return rec
	}
	return fmt.Sprintf("%s was present before but is now missing.", header)
}
