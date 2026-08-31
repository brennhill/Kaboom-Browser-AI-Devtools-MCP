// ssrf_resolver_test.go — Deterministic coverage of the DNS half of SSRF checks.
// Why: these assertions used to run against the machine's real resolver, so their
// verdict depended on the network the suite happened to be on. A resolver that
// answers NXDOMAIN with an address — ISP hijacking, a captive portal, a corporate
// wildcard, some VPNs — turned the fail-closed test red, and because that test
// dereferenced a nil error it PANICKED, taking every other test in the package
// down with it. The resolver is injected here so the rule is tested, not the LAN.

package uploadsec

import (
	"context"
	"errors"
	"net"
	"strings"
	"testing"
)

func withResolver(t *testing.T, fn func(context.Context, string) ([]net.IPAddr, error)) {
	t.Helper()
	previous := lookupIPAddr
	t.Cleanup(func() { lookupIPAddr = previous })
	lookupIPAddr = fn
}

func failingResolver(err error) func(context.Context, string) ([]net.IPAddr, error) {
	return func(context.Context, string) ([]net.IPAddr, error) { return nil, err }
}

func answeringResolver(addrs ...string) func(context.Context, string) ([]net.IPAddr, error) {
	return func(context.Context, string) ([]net.IPAddr, error) {
		out := make([]net.IPAddr, 0, len(addrs))
		for _, a := range addrs {
			out = append(out, net.IPAddr{IP: net.ParseIP(a)})
		}
		return out, nil
	}
}

// Fail closed: a name that will not resolve must be rejected, and the error must
// say why so an operator is not left guessing.
func TestResolvePublicIP_DNSFailureIsFailClosed(t *testing.T) {
	withResolver(t, failingResolver(errors.New("no such host")))

	ip, err := ResolvePublicIP(context.Background(), "unresolvable.example")
	if err == nil {
		t.Fatalf("ResolvePublicIP() = %v, want an error when DNS fails (fail-closed)", ip)
	}
	if !strings.Contains(err.Error(), "DNS") {
		t.Errorf("error should mention DNS failure, got: %v", err)
	}
}

func TestValidateFormActionURL_DNSFailureIsFailClosed(t *testing.T) {
	withResolver(t, failingResolver(errors.New("no such host")))

	err := ValidateFormActionURL("https://unresolvable.example/upload")
	if err == nil {
		t.Fatal("ValidateFormActionURL() = nil, want rejection when the hostname does not resolve")
	}
	if !strings.Contains(err.Error(), "DNS") {
		t.Errorf("error should mention DNS failure, got: %v", err)
	}
}

// An empty answer is not a successful lookup. Without this, a resolver returning
// zero addresses would fall through as if the host were fine.
func TestResolvePublicIP_EmptyAnswerIsRejected(t *testing.T) {
	withResolver(t, answeringResolver())

	if _, err := ResolvePublicIP(context.Background(), "empty.example"); err == nil {
		t.Fatal("ResolvePublicIP() = nil error for a resolver that returned no addresses")
	}
}

// The case that actually broke this suite: a resolver that HIJACKS a nonexistent
// name and answers with an address. Resolution then succeeds, so the fail-closed
// path never runs — and that is correct. What must still hold is the private-IP
// rule, which is the guarantee that matters when a name resolves to somewhere it
// should not.
func TestResolvePublicIP_HijackedNXDOMAIN(t *testing.T) {
	t.Run("hijacked to a public address is permitted", func(t *testing.T) {
		withResolver(t, answeringResolver("203.0.113.10"))
		ip, err := ResolvePublicIP(context.Background(), "does-not-exist.example")
		if err != nil {
			t.Fatalf("ResolvePublicIP() error = %v, want the public address to be accepted", err)
		}
		if !ip.Equal(net.ParseIP("203.0.113.10")) {
			t.Errorf("ResolvePublicIP() = %v, want 203.0.113.10", ip)
		}
	})

	t.Run("hijacked to a private address is blocked", func(t *testing.T) {
		withResolver(t, answeringResolver("192.168.0.1"))
		if _, err := ResolvePublicIP(context.Background(), "does-not-exist.example"); err == nil {
			t.Fatal("ResolvePublicIP() accepted a hostname hijacked to a private address")
		}
	})
}

// DNS rebinding: a name that resolves to a private address must be rejected
// whatever the name looks like. This replaces a test that queried nip.io over the
// real network and could only run when that third party was up.
func TestResolvePublicIP_RebindingToPrivateAddressIsBlocked(t *testing.T) {
	for _, addr := range []string{"127.0.0.1", "10.0.0.1", "192.168.1.1", "169.254.169.254", "::1", "fc00::1"} {
		t.Run(addr, func(t *testing.T) {
			withResolver(t, answeringResolver(addr))
			_, err := ResolvePublicIP(context.Background(), "rebind.example")
			if err == nil {
				t.Fatalf("ResolvePublicIP() accepted a hostname resolving to private address %s", addr)
			}
			if !strings.Contains(err.Error(), "private") {
				t.Errorf("error should mention a private address, got: %v", err)
			}
		})
	}
}

// A mixed answer must yield the public address rather than being rejected wholesale.
func TestResolvePublicIP_PrefersThePublicAddressInAMixedAnswer(t *testing.T) {
	withResolver(t, answeringResolver("10.0.0.1", "203.0.113.10"))

	ip, err := ResolvePublicIP(context.Background(), "mixed.example")
	if err != nil {
		t.Fatalf("ResolvePublicIP() error = %v, want the public address from a mixed answer", err)
	}
	if !ip.Equal(net.ParseIP("203.0.113.10")) {
		t.Errorf("ResolvePublicIP() = %v, want 203.0.113.10", ip)
	}
}

// A literal IP must never reach the resolver at all.
func TestResolvePublicIP_LiteralIPSkipsDNS(t *testing.T) {
	withResolver(t, func(context.Context, string) ([]net.IPAddr, error) {
		t.Fatal("ResolvePublicIP() performed a DNS lookup for a literal IP address")
		return nil, nil
	})
	if _, err := ResolvePublicIP(context.Background(), "203.0.113.10"); err != nil {
		t.Fatalf("ResolvePublicIP() error = %v for a public literal IP", err)
	}
	if _, err := ResolvePublicIP(context.Background(), "127.0.0.1"); err == nil {
		t.Fatal("ResolvePublicIP() accepted the loopback literal")
	}
}
