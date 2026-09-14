package remote

import (
	"net/http/httptest"
	"net/netip"
	"testing"
)

func TestSelfSSHException(t *testing.T) {
	t.Setenv("CONNECTME_REMOTE_DENY_CIDRS", "192.168.1.248/32,192.168.1.249/32")
	t.Setenv("CONNECTME_SELF_SSH_TARGET", "192.168.1.248:22")
	for _, v := range []struct {
		address, protocol string
		port              int
		want              bool
	}{
		{"192.168.1.248", "ssh", 22, true},
		{"192.168.1.248", "ssh", 8080, false},
		{"192.168.1.248", "rdp", 22, false},
		{"192.168.1.248", "rdp", 3389, false},
		{"192.168.1.249", "ssh", 22, false},
		{"192.168.1.200", "rdp", 3389, true},
	} {
		if destinationAllowed(netip.MustParseAddr(v.address), v.protocol, v.port) != v.want {
			t.Errorf("unexpected policy: %+v", v)
		}
	}
	for _, target := range []string{"", "192.168.1.248/32", "hostname:22", "192.168.1.248:0"} {
		t.Setenv("CONNECTME_SELF_SSH_TARGET", target)
		if destinationAllowed(netip.MustParseAddr("192.168.1.248"), "ssh", 22) {
			t.Fatal("invalid exception accepted")
		}
	}
	t.Setenv("CONNECTME_SELF_SSH_TARGET", "192.168.1.248:22")
	t.Setenv("CONNECTME_REMOTE_DENY_CIDRS", "invalid")
	if destinationAllowed(netip.MustParseAddr("192.168.1.248"), "ssh", 22) {
		t.Fatal("invalid deny list bypassed")
	}
}

func TestOriginPolicy(t *testing.T) {
	for _, v := range []struct {
		method, origin, upgrade string
		want                    bool
	}{
		{"HEAD", "", "", true}, {"GET", "", "", true}, {"POST", "", "", false},
		{"POST", "http://localhost", "", true}, {"POST", "http://attacker.invalid", "", false},
		{"GET", "", "websocket", false}, {"GET", "http://localhost", "websocket", true},
	} {
		r := httptest.NewRequest(v.method, "http://localhost/guacamole/api/session", nil)
		r.Header.Set("Origin", v.origin)
		r.Header.Set("Upgrade", v.upgrade)
		if validOrigin(r) != v.want {
			t.Errorf("unexpected origin decision: %+v", v)
		}
	}
}
func TestCertificateExceptionIsScoped(t *testing.T) {
	t.Setenv("CONNECTME_RDP_IGNORE_CERT", "true")
	t.Setenv("CONNECTME_RDP_INSECURE_CIDRS", "192.168.1.200/32")
	if !allowUntrustedCertificate("192.168.1.200") {
		t.Fatal("approved target rejected")
	}
	if allowUntrustedCertificate("192.168.1.201") {
		t.Fatal("exception escaped scope")
	}
	t.Setenv("CONNECTME_RDP_INSECURE_CIDRS", "")
	if allowUntrustedCertificate("192.168.1.200") {
		t.Fatal("empty allowlist accepted")
	}
}
