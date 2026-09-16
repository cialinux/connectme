package remote

import (
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"strings"
)

// Optional operator-defined denylist; empty allows every address.
func destinationAllowed(ip netip.Addr, _ string, _ int) bool {
	blocked := false
	for _, raw := range strings.Split(os.Getenv("CONNECTME_REMOTE_DENY_CIDRS"), ",") {
		if strings.TrimSpace(raw) == "" {
			continue
		}
		prefix, err := netip.ParsePrefix(strings.TrimSpace(raw))
		if err != nil {
			return false
		}
		blocked = blocked || prefix.Contains(ip)
	}
	return !blocked
}

func validOrigin(r *http.Request) bool {
	if (r.Method == http.MethodGet || r.Method == http.MethodHead) && r.Header.Get("Upgrade") == "" {
		return true
	}
	origin, err := url.Parse(r.Header.Get("Origin"))
	return err == nil && (origin.Scheme == "http" || origin.Scheme == "https") && origin.Host == r.Host && origin.User == nil
}

// Explicit global opt-in: TLS remains encrypted but server identity is not verified.
// The retired CONNECTME_RDP_INSECURE_CIDRS setting has no effect.
func allowUntrustedCertificate() bool {
	return os.Getenv("CONNECTME_RDP_IGNORE_CERT") == "true"
}
