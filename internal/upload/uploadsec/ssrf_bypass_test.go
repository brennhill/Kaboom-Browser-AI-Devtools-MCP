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

func TestSSRFBypassDecimalIP(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err := ResolvePublicIP(ctx, "2130706433")
	if err == nil {
		t.Error("ResolvePublicIP() expected error for decimal IP representation, got nil")
	}
}

func TestSSRFBypassHexIP(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err := ResolvePublicIP(ctx, "0x7f000001")
	if err == nil {
		t.Error("ResolvePublicIP() expected error for hex IP representation, got nil")
	}
}

func TestSSRFBypassOctalIP(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	ip, err := ResolvePublicIP(ctx, "0177.0.0.1")
	if err == nil && ip != nil && IsPrivateIP(ip) {
		t.Errorf("ResolvePublicIP() returned private IP %s without error for octal representation", ip)
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
func TestSSRFBypassDNSRebinding(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping DNS-dependent test in short mode")
	}

	tests := []struct {
		name string
		host string
	}{
		{"nip.io loopback", "127.0.0.1.nip.io"},
		{"nip.io private 10.x", "10.0.0.1.nip.io"},
		{"nip.io private 192.168.x", "192.168.1.1.nip.io"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			_, err := ResolvePublicIP(ctx, tt.host)
			if err == nil {
				t.Errorf("ResolvePublicIP() expected error for DNS rebinding host %q, got nil", tt.host)
			}
			if err != nil {
				errStr := err.Error()
				if !strings.Contains(errStr, "private") && !strings.Contains(errStr, "no such host") &&
					!strings.Contains(errStr, "lookup") && !strings.Contains(errStr, "timeout") {
					t.Logf("ResolvePublicIP() for %q returned: %v", tt.host, err)
				}
			}
		})
	}
}

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

func TestResolvePublicIPTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()
	_, err := ResolvePublicIP(ctx, "this-should-timeout.invalid")
	if err == nil {
		t.Error("ResolvePublicIP() expected timeout error, got nil")
	}
	if err != nil && !strings.Contains(err.Error(), "context") && !strings.Contains(err.Error(), "timeout") {
		t.Logf("ResolvePublicIP() timeout error: %v", err)
	}
}
