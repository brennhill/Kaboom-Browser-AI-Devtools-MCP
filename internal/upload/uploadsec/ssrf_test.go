// Purpose: Tests for SSRF protection in upload endpoints.
// Docs: docs/features/feature/file-upload/index.md

// ssrf_test.go — Tests for SSRF-safe transport and dialer helpers.
package uploadsec

import (
	"context"
	"net"
	"strings"
	"testing"
	"time"
)

// ============================================
// 1. IsPrivateIP — Private range detection
// ============================================

func TestSSRF_IsPrivateIP_RFC1918(t *testing.T) {
	privateIPs := []string{
		"10.0.0.1", "10.255.255.255", "10.1.2.3",
		"172.16.0.1", "172.31.255.255", "172.20.10.5",
		"192.168.0.1", "192.168.255.255", "192.168.1.100",
	}

	for _, ipStr := range privateIPs {
		t.Run(ipStr, func(t *testing.T) {
			ip := net.ParseIP(ipStr)
			if ip == nil {
				t.Fatalf("net.ParseIP(%q) returned nil", ipStr)
			}
			if !IsPrivateIP(ip) {
				t.Errorf("IsPrivateIP(%s) = false, want true (RFC 1918)", ipStr)
			}
		})
	}
}

func TestSSRF_IsPrivateIP_Loopback(t *testing.T) {
	loopbackIPs := []string{"127.0.0.1", "127.0.0.2", "127.255.255.255", "::1"}
	for _, ipStr := range loopbackIPs {
		t.Run(ipStr, func(t *testing.T) {
			ip := net.ParseIP(ipStr)
			if ip == nil {
				t.Fatalf("net.ParseIP(%q) returned nil", ipStr)
			}
			if !IsPrivateIP(ip) {
				t.Errorf("IsPrivateIP(%s) = false, want true (loopback)", ipStr)
			}
		})
	}
}

func TestSSRF_IsPrivateIP_LinkLocal(t *testing.T) {
	linkLocalIPs := []string{"169.254.0.1", "169.254.169.254", "169.254.255.255", "fe80::1", "fe80::abcd:1234"}
	for _, ipStr := range linkLocalIPs {
		t.Run(ipStr, func(t *testing.T) {
			ip := net.ParseIP(ipStr)
			if ip == nil {
				t.Fatalf("net.ParseIP(%q) returned nil", ipStr)
			}
			if !IsPrivateIP(ip) {
				t.Errorf("IsPrivateIP(%s) = false, want true (link-local)", ipStr)
			}
		})
	}
}

func TestSSRF_IsPrivateIP_Unspecified(t *testing.T) {
	unspecifiedIPs := []string{"0.0.0.0", "0.0.0.1", "0.255.255.255"}
	for _, ipStr := range unspecifiedIPs {
		t.Run(ipStr, func(t *testing.T) {
			ip := net.ParseIP(ipStr)
			if ip == nil {
				t.Fatalf("net.ParseIP(%q) returned nil", ipStr)
			}
			if !IsPrivateIP(ip) {
				t.Errorf("IsPrivateIP(%s) = false, want true (unspecified/zero)", ipStr)
			}
		})
	}
}

func TestSSRF_IsPrivateIP_IPv6UniqueLocal(t *testing.T) {
	uniqueLocalIPs := []string{"fc00::1", "fd00::1", "fdff:ffff:ffff:ffff:ffff:ffff:ffff:ffff"}
	for _, ipStr := range uniqueLocalIPs {
		t.Run(ipStr, func(t *testing.T) {
			ip := net.ParseIP(ipStr)
			if ip == nil {
				t.Fatalf("net.ParseIP(%q) returned nil", ipStr)
			}
			if !IsPrivateIP(ip) {
				t.Errorf("IsPrivateIP(%s) = false, want true (IPv6 unique local)", ipStr)
			}
		})
	}
}

func TestSSRF_IsPrivateIP_PublicIPs(t *testing.T) {
	publicIPs := []string{
		"8.8.8.8", "1.1.1.1", "93.184.216.34", "151.101.1.140",
		"172.32.0.1", "172.15.255.255", "11.0.0.1", "192.169.0.1",
		"2607:f8b0:4004::1", "2001:db8::1",
	}
	for _, ipStr := range publicIPs {
		t.Run(ipStr, func(t *testing.T) {
			ip := net.ParseIP(ipStr)
			if ip == nil {
				t.Fatalf("net.ParseIP(%q) returned nil", ipStr)
			}
			if IsPrivateIP(ip) {
				t.Errorf("IsPrivateIP(%s) = true, want false (public IP)", ipStr)
			}
		})
	}
}

// ============================================
// 2. ResolvePublicIP — IP/hostname resolution
// ============================================

func TestSSRF_ResolvePublicIP_LiteralPublicIPv4(t *testing.T) {
	ctx := context.Background()
	ip, err := ResolvePublicIP(ctx, "8.8.8.8")
	if err != nil {
		t.Fatalf("ResolvePublicIP(8.8.8.8) error = %v", err)
	}
	if !ip.Equal(net.ParseIP("8.8.8.8")) {
		t.Fatalf("ResolvePublicIP(8.8.8.8) = %s, want 8.8.8.8", ip)
	}
}

func TestSSRF_ResolvePublicIP_LiteralPublicIPv6(t *testing.T) {
	ctx := context.Background()
	ip, err := ResolvePublicIP(ctx, "2607:f8b0:4004::1")
	if err != nil {
		t.Fatalf("ResolvePublicIP(2607:f8b0:4004::1) error = %v", err)
	}
	if !ip.Equal(net.ParseIP("2607:f8b0:4004::1")) {
		t.Fatalf("ResolvePublicIP = %s, want 2607:f8b0:4004::1", ip)
	}
}

func TestSSRF_ResolvePublicIP_RejectsPrivateIPv4Literals(t *testing.T) {
	privateIPs := []string{"127.0.0.1", "10.0.0.1", "172.16.0.1", "192.168.1.1", "169.254.169.254", "0.0.0.0"}
	ctx := context.Background()
	for _, ipStr := range privateIPs {
		t.Run(ipStr, func(t *testing.T) {
			_, err := ResolvePublicIP(ctx, ipStr)
			if err == nil {
				t.Errorf("ResolvePublicIP(%q) should return error for private IP", ipStr)
			}
			if !strings.Contains(err.Error(), "private IP") {
				t.Errorf("error should mention 'private IP', got %q", err.Error())
			}
		})
	}
}

func TestSSRF_ResolvePublicIP_RejectsPrivateIPv6Literals(t *testing.T) {
	privateIPs := []string{"::1", "fc00::1", "fd00::1", "fe80::1"}
	ctx := context.Background()
	for _, ipStr := range privateIPs {
		t.Run(ipStr, func(t *testing.T) {
			_, err := ResolvePublicIP(ctx, ipStr)
			if err == nil {
				t.Errorf("ResolvePublicIP(%q) should return error for private IPv6", ipStr)
			}
			if !strings.Contains(err.Error(), "private IP") {
				t.Errorf("error should mention 'private IP', got %q", err.Error())
			}
		})
	}
}

func TestSSRF_ResolvePublicIP_EmptyHostname(t *testing.T) {
	ctx := context.Background()
	_, err := ResolvePublicIP(ctx, "")
	if err == nil {
		t.Fatal("ResolvePublicIP('') should return error")
	}
	if !strings.Contains(err.Error(), "empty hostname") {
		t.Errorf("error should mention 'empty hostname', got %q", err.Error())
	}
}

func TestSSRF_ResolvePublicIP_WhitespaceOnlyHostname(t *testing.T) {
	ctx := context.Background()
	_, err := ResolvePublicIP(ctx, "   ")
	if err == nil {
		t.Fatal("ResolvePublicIP('   ') should return error")
	}
	if !strings.Contains(err.Error(), "empty hostname") {
		t.Errorf("error should mention 'empty hostname', got %q", err.Error())
	}
}

func TestSSRF_ResolvePublicIP_WhitespaceTrimming(t *testing.T) {
	ctx := context.Background()
	ip, err := ResolvePublicIP(ctx, "  8.8.8.8  ")
	if err != nil {
		t.Fatalf("ResolvePublicIP('  8.8.8.8  ') error = %v", err)
	}
	if !ip.Equal(net.ParseIP("8.8.8.8")) {
		t.Fatalf("ip = %s, want 8.8.8.8", ip)
	}
}

func TestSSRF_ResolvePublicIP_IPv6ZoneSuffix(t *testing.T) {
	ctx := context.Background()
	_, err := ResolvePublicIP(ctx, "fe80::1%eth0")
	if err == nil {
		t.Fatal("should reject private IPv6 even with zone suffix")
	}
	if !strings.Contains(err.Error(), "private IP") {
		t.Errorf("error should mention 'private IP', got %q", err.Error())
	}
}

func TestSSRF_ResolvePublicIP_IPv6ZoneSuffix_PublicIP(t *testing.T) {
	ctx := context.Background()
	ip, err := ResolvePublicIP(ctx, "2607:f8b0:4004::1%eth0")
	if err != nil {
		t.Fatalf("ResolvePublicIP with zone suffix error = %v", err)
	}
	if !ip.Equal(net.ParseIP("2607:f8b0:4004::1")) {
		t.Fatalf("ip = %s, want 2607:f8b0:4004::1", ip)
	}
}

func TestSSRF_ResolvePublicIP_CancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	ip, err := ResolvePublicIP(ctx, "8.8.8.8")
	if err != nil {
		t.Fatalf("literal IP should resolve regardless of context: %v", err)
	}
	if !ip.Equal(net.ParseIP("8.8.8.8")) {
		t.Fatalf("ip = %s, want 8.8.8.8", ip)
	}
}

func TestSSRF_ResolvePublicIP_DNSLookupTimeout(t *testing.T) {
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-1*time.Second))
	defer cancel()
	_, err := ResolvePublicIP(ctx, "this-host-should-timeout.example.invalid")
	if err == nil {
		t.Fatal("should return error with expired context for DNS lookup")
	}
}

// ============================================
// 3. SSRFSafeDialContext — Dial-level SSRF blocking
// ============================================

func TestSSRF_SafeDial_InvalidAddress(t *testing.T) {
	ctx := context.Background()
	_, err := SSRFSafeDialContext(ctx, "tcp", "no-port-here", false)
	if err == nil {
		t.Fatal("should reject address without port")
	}
	if !strings.Contains(err.Error(), "ssrf_blocked") {
		t.Errorf("error should contain 'ssrf_blocked', got %q", err.Error())
	}
}

func TestSSRF_SafeDial_BlocksPrivateIPv4(t *testing.T) {
	privateAddrs := []string{
		"127.0.0.1:80", "10.0.0.1:443", "172.16.0.1:8080",
		"192.168.1.1:3000", "169.254.169.254:80", "0.0.0.0:80",
	}
	ctx := context.Background()
	for _, addr := range privateAddrs {
		t.Run(addr, func(t *testing.T) {
			_, err := SSRFSafeDialContext(ctx, "tcp", addr, false)
			if err == nil {
				t.Errorf("SSRFSafeDialContext should block private address %s", addr)
			}
			if !strings.Contains(err.Error(), "ssrf_blocked") {
				t.Errorf("error should contain 'ssrf_blocked', got %q", err.Error())
			}
		})
	}
}

func TestSSRF_SafeDial_BlocksPrivateIPv6(t *testing.T) {
	privateAddrs := []string{"[::1]:80", "[fc00::1]:443", "[fd00::1]:8080", "[fe80::1]:3000"}
	ctx := context.Background()
	for _, addr := range privateAddrs {
		t.Run(addr, func(t *testing.T) {
			_, err := SSRFSafeDialContext(ctx, "tcp", addr, false)
			if err == nil {
				t.Errorf("SSRFSafeDialContext should block private IPv6 address %s", addr)
			}
			if !strings.Contains(err.Error(), "ssrf_blocked") {
				t.Errorf("error should contain 'ssrf_blocked', got %q", err.Error())
			}
		})
	}
}

func TestSSRF_SafeDial_AllowsPrivateWhenFlagSet(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_, err := SSRFSafeDialContext(ctx, "tcp", "127.0.0.1:19999", true)
	if err == nil {
		t.Fatal("should fail to connect (nothing listening) but not ssrf_blocked")
	}
	if strings.Contains(err.Error(), "ssrf_blocked") {
		t.Errorf("allowPrivate=true should bypass SSRF check, got: %v", err)
	}
}

// ============================================
// 4. SSRFAllowedHost bypass
// ============================================

func TestSSRF_AllowedHost_BypassesCheck(t *testing.T) {
	origList := SSRFAllowedHosts()
	defer func() { SetSSRFAllowedHosts(origList) }()
	SetSSRFAllowedHosts([]string{"127.0.0.1:19999"})

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_, err := SSRFSafeDialContext(ctx, "tcp", "127.0.0.1:19999", false)
	if err == nil {
		return
	}
	if strings.Contains(err.Error(), "ssrf_blocked") {
		t.Errorf("ssrf-allow-host should bypass SSRF, got: %v", err)
	}
}

func TestSSRF_AllowedHost_HostOnlyMatch(t *testing.T) {
	origList := SSRFAllowedHosts()
	defer func() { SetSSRFAllowedHosts(origList) }()
	SetSSRFAllowedHosts([]string{"127.0.0.1"})

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_, err := SSRFSafeDialContext(ctx, "tcp", "127.0.0.1:19999", false)
	if err == nil {
		return
	}
	if strings.Contains(err.Error(), "ssrf_blocked") {
		t.Errorf("host-only allow should bypass SSRF, got: %v", err)
	}
}

func TestSSRF_AllowedHost_NoMatchStillBlocks(t *testing.T) {
	origList := SSRFAllowedHosts()
	defer func() { SetSSRFAllowedHosts(origList) }()
	SetSSRFAllowedHosts([]string{"some-other-host:8080"})

	ctx := context.Background()
	_, err := SSRFSafeDialContext(ctx, "tcp", "127.0.0.1:80", false)
	if err == nil {
		t.Fatal("should block when host is not in allow list")
	}
	if !strings.Contains(err.Error(), "ssrf_blocked") {
		t.Errorf("non-matching allow list should still block, got: %v", err)
	}
}

func TestSSRF_IsSSRFAllowedHost_EmptyList(t *testing.T) {
	origList := SSRFAllowedHosts()
	defer func() { SetSSRFAllowedHosts(origList) }()
	SetSSRFAllowedHosts(nil)

	if IsSSRFAllowedHost("127.0.0.1:80") {
		t.Error("empty list should never match")
	}
	if IsSSRFAllowedHost("") {
		t.Error("empty list should never match empty string")
	}
}

func TestSSRF_IsSSRFAllowedHost_ExactMatch(t *testing.T) {
	origList := SSRFAllowedHosts()
	defer func() { SetSSRFAllowedHosts(origList) }()
	SetSSRFAllowedHosts([]string{"localhost:3000", "10.0.0.1:8080"})

	tests := []struct {
		input string
		want  bool
	}{
		{"localhost:3000", true},
		{"10.0.0.1:8080", true},
		{"localhost:3001", false},
		{"localhost", false},
		{"10.0.0.2:8080", false},
		{"LOCALHOST:3000", false},
	}
	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got := IsSSRFAllowedHost(tc.input)
			if got != tc.want {
				t.Errorf("IsSSRFAllowedHost(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

// ============================================
// 5. NewSSRFSafeTransport — Transport construction
// ============================================

func TestSSRF_NewTransport_NotNil(t *testing.T) {
	transport := NewSSRFSafeTransport(nil)
	if transport == nil {
		t.Fatal("NewSSRFSafeTransport should return non-nil transport")
	}
	if transport.DialContext == nil {
		t.Fatal("transport.DialContext should be set")
	}
}

func TestSSRF_NewTransport_AllowPrivateFalse(t *testing.T) {
	transport := NewSSRFSafeTransport(func() bool { return false })
	ctx := context.Background()
	_, err := transport.DialContext(ctx, "tcp", "127.0.0.1:80")
	if err == nil {
		t.Fatal("should block loopback when allowPrivate returns false")
	}
	if !strings.Contains(err.Error(), "ssrf_blocked") {
		t.Errorf("error should contain 'ssrf_blocked', got %q", err.Error())
	}
}

func TestSSRF_NewTransport_AllowPrivateTrue(t *testing.T) {
	transport := NewSSRFSafeTransport(func() bool { return true })
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_, err := transport.DialContext(ctx, "tcp", "127.0.0.1:19999")
	if err == nil {
		return
	}
	if strings.Contains(err.Error(), "ssrf_blocked") {
		t.Errorf("allowPrivate=true should bypass SSRF, got: %v", err)
	}
}

func TestSSRF_NewTransport_NilAllowPrivateFn(t *testing.T) {
	transport := NewSSRFSafeTransport(nil)
	ctx := context.Background()
	_, err := transport.DialContext(ctx, "tcp", "127.0.0.1:80")
	if err == nil {
		t.Fatal("nil allowPrivateFn should default to blocking private IPs")
	}
	if !strings.Contains(err.Error(), "ssrf_blocked") {
		t.Errorf("error should contain 'ssrf_blocked', got %q", err.Error())
	}
}

// ============================================
// 6. Cloud metadata endpoint protection
// ============================================

func TestSSRF_BlocksCloudMetadataEndpoint(t *testing.T) {
	ctx := context.Background()
	_, err := SSRFSafeDialContext(ctx, "tcp", "169.254.169.254:80", false)
	if err == nil {
		t.Fatal("should block cloud metadata endpoint")
	}
	if !strings.Contains(err.Error(), "ssrf_blocked") {
		t.Errorf("error should contain 'ssrf_blocked', got %q", err.Error())
	}
}

func TestSSRF_ResolvePublicIP_BlocksMetadataIP(t *testing.T) {
	ctx := context.Background()
	_, err := ResolvePublicIP(ctx, "169.254.169.254")
	if err == nil {
		t.Fatal("should reject metadata IP as private")
	}
}

// ============================================
// 7. Edge cases and boundary IPs
// ============================================

func TestSSRF_BoundaryIPs(t *testing.T) {
	tests := []struct {
		name    string
		ip      string
		private bool
	}{
		{"10_start", "10.0.0.0", true}, {"10_end", "10.255.255.255", true},
		{"before_10", "9.255.255.255", false}, {"after_10", "11.0.0.0", false},
		{"172_16_start", "172.16.0.0", true}, {"172_31_end", "172.31.255.255", true},
		{"before_172_16", "172.15.255.255", false}, {"after_172_31", "172.32.0.0", false},
		{"192_168_start", "192.168.0.0", true}, {"192_168_end", "192.168.255.255", true},
		{"before_192_168", "192.167.255.255", false}, {"after_192_168", "192.169.0.0", false},
		{"127_start", "127.0.0.0", true}, {"127_end", "127.255.255.255", true},
		{"before_127", "126.255.255.255", false}, {"after_127", "128.0.0.0", false},
		{"link_local_start", "169.254.0.0", true}, {"link_local_end", "169.254.255.255", true},
		{"before_link_local", "169.253.255.255", false}, {"after_link_local", "169.255.0.0", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ip := net.ParseIP(tc.ip)
			if ip == nil {
				t.Fatalf("net.ParseIP(%q) returned nil", tc.ip)
			}
			got := IsPrivateIP(ip)
			if got != tc.private {
				t.Errorf("IsPrivateIP(%s) = %v, want %v", tc.ip, got, tc.private)
			}
		})
	}
}

// ============================================
// 8. SSRFSafeDialContext — address splitting
// ============================================

func TestSSRF_SafeDial_EmptyAddress(t *testing.T) {
	ctx := context.Background()
	_, err := SSRFSafeDialContext(ctx, "tcp", "", false)
	if err == nil {
		t.Fatal("should reject empty address")
	}
	if !strings.Contains(err.Error(), "ssrf_blocked") {
		t.Errorf("error should contain 'ssrf_blocked', got %q", err.Error())
	}
}

func TestSSRF_SafeDial_MalformedAddresses(t *testing.T) {
	malformed := []string{"just-a-host", ":80", "host:port:extra"}
	ctx := context.Background()
	for _, addr := range malformed {
		t.Run(addr, func(t *testing.T) {
			_, err := SSRFSafeDialContext(ctx, "tcp", addr, false)
			if err == nil {
				t.Errorf("should reject malformed address %q", addr)
			}
		})
	}
}

// ============================================
// 9. DNS rebinding — private resolution
// ============================================

func TestSSRF_ResolvePublicIP_InvalidHostname(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err := ResolvePublicIP(ctx, "this-hostname-does-not-exist.invalid")
	if err == nil {
		t.Fatal("should return error for non-existent hostname")
	}
	if !strings.Contains(err.Error(), "DNS lookup") {
		t.Errorf("error should mention DNS lookup, got %q", err.Error())
	}
}

// ============================================
// 10. SSRFLookupTimeout constant
// ============================================

func TestSSRF_LookupTimeoutValue(t *testing.T) {
	if SSRFLookupTimeout != 5*time.Second {
		t.Errorf("SSRFLookupTimeout = %v, want 5s", SSRFLookupTimeout)
	}
}
