// Purpose: Blocks SSRF via DNS-pinned dialing: resolves hostnames, rejects private IPs, and provides SSRF-safe HTTP transport.
// Docs: docs/features/feature/file-upload/index.md

package uploadsec

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

const SSRFLookupTimeout = 5 * time.Second

// ssrfMu protects ssrfAllowedHosts and ssrfSkipCheck from concurrent access.
var (
	ssrfMu           sync.RWMutex
	ssrfAllowedHosts []string
	ssrfSkipCheck    bool
)

// SetSSRFAllowedHosts replaces the allowed-hosts list (set via --ssrf-allow-host flag).
func SetSSRFAllowedHosts(hosts []string) {
	ssrfMu.Lock()
	defer ssrfMu.Unlock()
	ssrfAllowedHosts = hosts
}

// SSRFAllowedHosts returns a defensive copy of the current allowed-hosts list.
func SSRFAllowedHosts() []string {
	ssrfMu.RLock()
	defer ssrfMu.RUnlock()
	cp := make([]string, len(ssrfAllowedHosts))
	copy(cp, ssrfAllowedHosts)
	return cp
}

// SetSkipSSRFCheck toggles private-IP bypass (test use only).
func SetSkipSSRFCheck(skip bool) {
	ssrfMu.Lock()
	defer ssrfMu.Unlock()
	ssrfSkipCheck = skip
}

// skipSSRFCheckEnabled returns true when private-IP blocking is bypassed.
func skipSSRFCheckEnabled() bool {
	ssrfMu.RLock()
	defer ssrfMu.RUnlock()
	return ssrfSkipCheck
}

// privateRanges is parsed once at init for efficient SSRF checks.
var privateRanges []*net.IPNet

func init() {
	for _, cidr := range []string{
		"127.0.0.0/8",    // IPv4 loopback
		"10.0.0.0/8",     // RFC 1918
		"172.16.0.0/12",  // RFC 1918
		"192.168.0.0/16", // RFC 1918
		"169.254.0.0/16", // link-local / cloud metadata
		"0.0.0.0/8",      // unspecified (routes to localhost)
		"224.0.0.0/4",    // IPv4 multicast
		"100.64.0.0/10",  // RFC 6598 Carrier-Grade NAT
		"198.18.0.0/15",  // RFC 2544 benchmarking
		"::1/128",        // IPv6 loopback
		"fc00::/7",       // IPv6 unique local
		"fe80::/10",      // IPv6 link-local
		"ff00::/8",       // IPv6 multicast
	} {
		_, ipNet, _ := net.ParseCIDR(cidr)
		privateRanges = append(privateRanges, ipNet)
	}
}

// IsPrivateIP returns true if the IP is in a private, loopback, link-local, or unspecified range.
func IsPrivateIP(ip net.IP) bool {
	if ip.IsUnspecified() || ip.IsLoopback() || ip.IsMulticast() {
		return true
	}
	for _, cidr := range privateRanges {
		if cidr.Contains(ip) {
			return true
		}
	}
	return false
}

// IsSSRFAllowedHost returns true if hostOrAddr matches an --ssrf-allow-host entry.
func IsSSRFAllowedHost(hostOrAddr string) bool {
	for _, allowed := range SSRFAllowedHosts() {
		if allowed == hostOrAddr {
			return true
		}
	}
	return false
}

// ResolvePublicIP resolves host and returns the first non-private IP.
func ResolvePublicIP(ctx context.Context, host string) (net.IP, error) {
	normalized := strings.TrimSpace(host)
	if normalized == "" {
		return nil, fmt.Errorf("empty hostname")
	}

	// Strip optional IPv6 zone suffix to keep ParseIP deterministic.
	if idx := strings.IndexByte(normalized, '%'); idx != -1 {
		normalized = normalized[:idx]
	}

	if ip := net.ParseIP(normalized); ip != nil {
		if IsPrivateIP(ip) {
			return nil, fmt.Errorf("host %q is private IP %s", host, ip.String())
		}
		return ip, nil
	}

	ips, err := net.DefaultResolver.LookupIPAddr(ctx, normalized)
	if err != nil {
		return nil, fmt.Errorf("DNS lookup failed for %q: %w", host, err)
	}
	if len(ips) == 0 {
		return nil, fmt.Errorf("DNS lookup returned no addresses for %q", host)
	}

	for _, ipAddr := range ips {
		if ipAddr.IP == nil {
			continue
		}
		if !IsPrivateIP(ipAddr.IP) {
			return ipAddr.IP, nil
		}
	}

	return nil, fmt.Errorf("hostname %q resolves only to private IP addresses", host)
}

// SSRFSafeDialContext validates destination address and dials a pinned public IP.
func SSRFSafeDialContext(ctx context.Context, network, addr string, allowPrivate bool) (net.Conn, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, fmt.Errorf("ssrf_blocked: invalid address %s: %w", addr, err)
	}

	// Check --ssrf-allow-host flag (test use: allows localhost test servers)
	if IsSSRFAllowedHost(addr) || IsSSRFAllowedHost(host) {
		allowPrivate = true
	}

	// Test hook for httptest servers on loopback.
	if allowPrivate {
		var d net.Dialer
		return d.DialContext(ctx, network, net.JoinHostPort(host, port))
	}

	lookupCtx, cancel := context.WithTimeout(ctx, SSRFLookupTimeout)
	defer cancel()

	ip, err := ResolvePublicIP(lookupCtx, host)
	if err != nil {
		return nil, fmt.Errorf("ssrf_blocked: %w", err)
	}

	var d net.Dialer
	return d.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
}

// NewSSRFSafeTransport returns an HTTP transport that blocks private/internal targets
// and pins DNS resolution to the dialed IP per connection.
func NewSSRFSafeTransport(allowPrivateFn func() bool) *http.Transport {
	transport := (&http.Transport{}).Clone()
	if base, ok := http.DefaultTransport.(*http.Transport); ok && base != nil {
		transport = base.Clone()
	}
	transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		allowPrivate := false
		if allowPrivateFn != nil {
			allowPrivate = allowPrivateFn()
		}
		return SSRFSafeDialContext(ctx, network, addr, allowPrivate)
	}
	return transport
}

// DefaultSSRFTransport returns an SSRF-safe transport that honours the
// --ssrf-allow-host / skip-check test hooks. Callers outside this package use
// this instead of reading the skip flag directly, so the flag stays private.
func DefaultSSRFTransport() *http.Transport {
	return NewSSRFSafeTransport(skipSSRFCheckEnabled)
}
