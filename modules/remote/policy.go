package remote

import (
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"strings"
)

// Read-only session probes do not carry Origin in normal browsers.
// The self-host SSH exception overrides only a matching deny entry; it does
// not bypass host CIDR validation, IP pinning, enabled flags or authorization.
func destinationAllowed(ip netip.Addr, protocol string, port int) bool {
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
	if !blocked {
		return true
	}
	target, err := netip.ParseAddrPort(os.Getenv("CONNECTME_SELF_SSH_TARGET"))
	return err == nil && protocol == "ssh" && port > 0 && port <= 65535 && int(target.Port()) == port && target.Addr() == ip
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
