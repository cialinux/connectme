package remote

import (
	"net/http/httptest"
	"net/netip"
	"testing"
)

func TestDestinationPolicy(t *testing.T) {
	for _, address := range []string{"192.0.2.10", "198.51.100.20", "2001:db8::1", "127.0.0.1"} {
		t.Setenv("CONNECTME_REMOTE_DENY_CIDRS", "")
		if !destinationAllowed(netip.MustParseAddr(address), "ssh", 22) {
			t.Fatal("default must allow every address")
		}
	}
	t.Setenv("CONNECTME_REMOTE_DENY_CIDRS", "192.0.2.0/24")
	if destinationAllowed(netip.MustParseAddr("192.0.2.10"), "ssh", 22) {
		t.Fatal("explicit operator deny ignored")
	}
	t.Setenv("CONNECTME_REMOTE_DENY_CIDRS", "invalid")
	if destinationAllowed(netip.MustParseAddr("192.0.2.10"), "ssh", 22) {
		t.Fatal("malformed policy accepted")
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
func TestCertificateExceptionIsGlobalOptIn(t *testing.T) {
	for _, legacy := range []string{"", "192.0.2.20/32", "invalid"} {
		t.Setenv("CONNECTME_RDP_INSECURE_CIDRS", legacy)
		for _, setting := range []string{"true", "false", "", "invalid"} {
			t.Setenv("CONNECTME_RDP_IGNORE_CERT", setting)
			if got := allowUntrustedCertificate(); got != (setting == "true") {
				t.Fatalf("global certificate option %q returned %v", setting, got)
			}
		}
	}
}
