// policy.go — Per-origin consent for browser DRIVING (as distinct from observation).
//
// Kaboom holds <all_urls> host permissions and the debugger permission. Once input runs over
// a persistent CDP session with isTrusted:true events, anything that reaches the interact
// tool can click and type as the user on every origin the browser is signed into. The
// existing domain filters govern which pages are OBSERVED; nothing governed which pages
// could be DRIVEN. This package is that boundary.
//
// Two rules make it a boundary rather than a formality:
//   - Gating is defined by an explicit read-only set, so an action added later is gated by
//     default. Deriving it from a list of mutating actions would fail open.
//   - A target that cannot be resolved to an origin is denied, not waved through.
package browserconsent

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"sort"
	"strings"
	"sync"
)

// Named decision reasons. Callers and tests match these instead of prose.
const (
	ReasonNotGated           = "action_not_gated"
	ReasonLocalhost          = "localhost_default_allow"
	ReasonSessionConsent     = "session_consent"
	ReasonPersistentConsent  = "persistent_consent"
	ReasonNoConsent          = "no_consent_for_origin"
	ReasonUnresolvableTarget = "unresolvable_target"
)

// ErrUnusableOrigin reports a URL that cannot be reduced to an http(s) origin.
var ErrUnusableOrigin = errors.New("not an http(s) origin")

// readOnlyActions never change page, browser, or profile state, so they ride on the existing
// observation filters. EVERYTHING ELSE IS GATED — including actions that do not exist yet.
// Adding an entry here is a deliberate decision to exempt it from consent.
var readOnlyActions = map[string]bool{
	"get_text":         true,
	"get_value":        true,
	"get_attribute":    true,
	"query":            true,
	"list_interactive": true,
	"get_readable":     true,
	"get_markdown":     true,
	"explore_page":     true,
	"wait_for":         true,
	"wait_for_stable":  true,
	"list_states":      true,
	"clipboard_read":   true,
}

// IsGated reports whether an action requires driving consent for its target origin.
// Unknown actions are gated: the safe default for a security boundary is to refuse.
func IsGated(action string) bool {
	return !readOnlyActions[strings.ToLower(strings.TrimSpace(action))]
}

// Decision is the outcome of one consent check.
type Decision struct {
	Allowed bool
	Reason  string
	Origin  string
}

// Policy holds the consented origins. The zero value is not usable; call NewPolicy.
type Policy struct {
	mu             sync.RWMutex
	persistent     map[string]bool
	session        map[string]bool
	allowLocalhost bool
}

// NewPolicy returns an empty policy that allows local development origins by default.
func NewPolicy() *Policy {
	return &Policy{
		persistent:     make(map[string]bool),
		session:        make(map[string]bool),
		allowLocalhost: true,
	}
}

// OriginOf reduces a URL to scheme://host[:port], dropping path, query and fragment so a
// consent entry can never carry a token or an email address into a list or a log.
func OriginOf(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", ErrUnusableOrigin
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return "", fmt.Errorf("%w: %s", ErrUnusableOrigin, err)
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return "", ErrUnusableOrigin
	}
	host := strings.ToLower(parsed.Hostname())
	if host == "" {
		return "", ErrUnusableOrigin
	}
	port := parsed.Port()
	if (scheme == "http" && port == "80") || (scheme == "https" && port == "443") {
		port = ""
	}
	if port != "" {
		return scheme + "://" + net.JoinHostPort(host, port), nil
	}
	if strings.Contains(host, ":") { // bare IPv6 literal
		return scheme + "://[" + host + "]", nil
	}
	return scheme + "://" + host, nil
}

// isLoopback reports whether an origin is a local development target.
func isLoopback(origin string) bool {
	parsed, err := url.Parse(origin)
	if err != nil {
		return false
	}
	host := parsed.Hostname()
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// Allow grants persistent consent for an origin.
func (p *Policy) Allow(raw string) error { return p.add(raw, false) }

// AllowForSession grants consent that ClearSession revokes.
func (p *Policy) AllowForSession(raw string) error { return p.add(raw, true) }

func (p *Policy) add(raw string, session bool) error {
	origin, err := OriginOf(raw)
	if err != nil {
		return fmt.Errorf("cannot grant consent for %q: %w", raw, err)
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if session {
		p.session[origin] = true
		return nil
	}
	p.persistent[origin] = true
	return nil
}

// Revoke removes persistent and session consent for an origin.
func (p *Policy) Revoke(raw string) error {
	origin, err := OriginOf(raw)
	if err != nil {
		return fmt.Errorf("cannot revoke consent for %q: %w", raw, err)
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.persistent, origin)
	delete(p.session, origin)
	return nil
}

// ClearSession drops every session-scoped grant, leaving persistent consent intact.
func (p *Policy) ClearSession() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.session = make(map[string]bool)
}

// SetAllowLocalhost turns the local-development default on or off.
func (p *Policy) SetAllowLocalhost(allow bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.allowLocalhost = allow
}

// List returns the persistently consented origins, sorted. Session grants are excluded so
// the durable list a user inspects is exactly what survives a restart.
func (p *Policy) List() []string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	out := make([]string, 0, len(p.persistent))
	for origin := range p.persistent {
		out = append(out, origin)
	}
	sort.Strings(out)
	return out
}

// Decide reports whether an action may run against a target URL.
func (p *Policy) Decide(action, targetURL string) Decision {
	if !IsGated(action) {
		return Decision{Allowed: true, Reason: ReasonNotGated}
	}
	origin, err := OriginOf(targetURL)
	if err != nil {
		// A gated action whose target cannot be identified is the case where proceeding is
		// least safe, so it is refused rather than defaulted.
		return Decision{Allowed: false, Reason: ReasonUnresolvableTarget}
	}

	p.mu.RLock()
	defer p.mu.RUnlock()
	switch {
	case p.session[origin]:
		return Decision{Allowed: true, Reason: ReasonSessionConsent, Origin: origin}
	case p.persistent[origin]:
		return Decision{Allowed: true, Reason: ReasonPersistentConsent, Origin: origin}
	case p.allowLocalhost && isLoopback(origin):
		return Decision{Allowed: true, Reason: ReasonLocalhost, Origin: origin}
	default:
		return Decision{Allowed: false, Reason: ReasonNoConsent, Origin: origin}
	}
}
