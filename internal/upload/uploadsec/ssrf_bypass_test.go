// Purpose: Tests for SSRF bypass prevention in upload URL validation.
// Docs: docs/features/feature/file-upload/index.md

// ssrf_bypass_test.go — Tests for SSRF bypass resistance.
package uploadsec

import (
	"context"
	"net"
	"strings"
	"testing"
	"time"
)

// Alternate encodings of 127.0.0.1. net.ParseIP rejects all of them, so each one
// reaches the resolver — and what happens next depends entirely on that resolver.
// The old versions of these tests only passed because the machine's DNS happened
// to fail; on a resolver that answers anything (an ISP hijack, or a libc that
// interprets the decimal form via inet_aton) they went red, having proven nothing.
//
// The guarantee that actually matters is the same either way: the host must NOT be
// permitted. Both resolver outcomes are pinned here rather than left to the LAN.
func TestSSRFBypassAlternateLoopbackEncodings(t *testing.T) {
	encodings := []struct{ name, host string }{
		{"decimal", "2130706433"},
		{"hex", "0x7f000001"},
		{"octal-dotted", "0177.0.0.1"},
	}
	for _, enc := range encodings {
		t.Run(enc.name+"/resolver decodes it to loopback", func(t *testing.T) {
			previous := lookupIPAddr
			t.Cleanup(func() { lookupIPAddr = previous })
			lookupIPAddr = func(context.Context, string) ([]net.IPAddr, error) {
				return []net.IPAddr{{IP: net.ParseIP("127.0.0.1")}}, nil
			}
			ip, err := ResolvePublicIP(context.Background(), enc.host)
			if err == nil {
				t.Fatalf("ResolvePublicIP(%q) returned %v, want the loopback address blocked", enc.host, ip)
			}
			if !strings.Contains(err.Error(), "private") {
				t.Errorf("error should mention a private address, got: %v", err)
			}
		})

		t.Run(enc.name+"/resolver rejects it", func(t *testing.T) {
			previous := lookupIPAddr
			t.Cleanup(func() { lookupIPAddr = previous })
			lookupIPAddr = func(context.Context, string) ([]net.IPAddr, error) {
				return nil, &net.DNSError{Err: "no such host", IsNotFound: true}
			}
			if _, err := ResolvePublicIP(context.Background(), enc.host); err == nil {
				t.Fatalf("ResolvePublicIP(%q) was permitted when the lookup failed", enc.host)
			}
		})
	}
}

func TestSSRFBypassIPv6MappedIPv4(t *testing.T) {
	tests := []struct {
		name string
		host string
	}{
		{"IPv6-mapped loopback", "::ffff:127.0.0.1"},
		{"IPv6-mapped private 10.x", "::ffff:10.0.0.1"},
		{"IPv6-mapped private 192.168.x", "::ffff:192.168.1.1"},
		{"IPv6-mapped private 172.16.x", "::ffff:172.16.0.1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			_, err := ResolvePublicIP(ctx, tt.host)
			if err == nil {
				t.Errorf("ResolvePublicIP() expected error for IPv6-mapped private IP %q, got nil", tt.host)
			}
			if err != nil && !strings.Contains(err.Error(), "private") {
				t.Errorf("ResolvePublicIP() error should mention 'private', got: %v", err)
			}
		})
	}
}
// DNS rebinding is covered by TestResolvePublicIP_RebindingToPrivateAddressIsBlocked
// in ssrf_resolver_test.go. The version that stood here resolved 127.0.0.1.nip.io
// and friends over the real network, so it tested whether nip.io was up as much as
// it tested the rule.

func TestSSRFBypassIPv4MappedShortForms(t *testing.T) {
	tests := []struct {
		name string
		host string
	}{
		{"IPv6-mapped short form loopback", "::ffff:7f00:1"},
		{"IPv6-mapped short form private", "::ffff:c0a8:101"},
		{"IPv6-mapped short form 10.x", "::ffff:a00:1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			_, err := ResolvePublicIP(ctx, tt.host)
			if err == nil {
				t.Errorf("ResolvePublicIP() expected error for IPv6-mapped short form %q, got nil", tt.host)
			}
			if ip := net.ParseIP(tt.host); ip != nil {
				if !IsPrivateIP(ip) {
					t.Errorf("IsPrivateIP() failed to identify %q as private", tt.host)
				}
			}
		})
	}
}

func TestIsPrivateIPComprehensive(t *testing.T) {
	tests := []struct {
		name     string
		ip       string
		wantPriv bool
	}{
		{"loopback 127.0.0.1", "127.0.0.1", true},
		{"loopback 127.255.255.255", "127.255.255.255", true},
		{"private 10.0.0.0", "10.0.0.0", true},
		{"private 10.255.255.255", "10.255.255.255", true},
		{"private 172.16.0.0", "172.16.0.0", true},
		{"private 172.31.255.255", "172.31.255.255", true},
		{"private 192.168.0.0", "192.168.0.0", true},
		{"private 192.168.255.255", "192.168.255.255", true},
		{"link-local 169.254.0.0", "169.254.0.0", true},
		{"link-local 169.254.255.255", "169.254.255.255", true},
		{"public 8.8.8.8", "8.8.8.8", false},
		{"public 1.1.1.1", "1.1.1.1", false},
		{"IPv6 loopback", "::1", true},
		{"IPv6 ULA fc00::", "fc00::", true},
		{"IPv6 ULA fdff:ffff:ffff:ffff:ffff:ffff:ffff:ffff", "fdff:ffff:ffff:ffff:ffff:ffff:ffff:ffff", true},
		{"IPv6 link-local", "fe80::1", true},
		{"IPv6-mapped 127.0.0.1", "::ffff:127.0.0.1", true},
		{"IPv6-mapped 10.0.0.1", "::ffff:10.0.0.1", true},
		{"IPv6-mapped 192.168.1.1", "::ffff:192.168.1.1", true},
		{"IPv6-mapped public 8.8.8.8", "::ffff:8.8.8.8", false},
		{"IPv6 public 2001:4860:4860::8888", "2001:4860:4860::8888", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ip := net.ParseIP(tt.ip)
			if ip == nil {
				t.Fatalf("net.ParseIP(%q) failed", tt.ip)
			}
			got := IsPrivateIP(ip)
			if got != tt.wantPriv {
				t.Errorf("IsPrivateIP(%q) = %v, want %v", tt.ip, got, tt.wantPriv)
			}
		})
	}
}

// A resolver that outlives the caller's deadline must surface as an error, not as
// a permitted host. Driven through the injected resolver so the deadline is the
// thing under test rather than the responsiveness of a real nameserver.
func TestResolvePublicIPTimeout(t *testing.T) {
	previous := lookupIPAddr
	t.Cleanup(func() { lookupIPAddr = previous })
	lookupIPAddr = func(ctx context.Context, _ string) ([]net.IPAddr, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()

	if _, err := ResolvePublicIP(ctx, "slow.example"); err == nil {
		t.Fatal("ResolvePublicIP() = nil error when the lookup exceeded the deadline")
	}
}