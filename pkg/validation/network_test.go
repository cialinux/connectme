package validation

import (
	"context"
	"net"
	"net/netip"
	"testing"
)

func TestDestinationCIDR(t *testing.T) {
	allowed := []netip.Prefix{netip.MustParsePrefix("192.168.1.0/24")}
	ip, err := Destination(context.Background(), net.DefaultResolver, "192.168.1.20", allowed)
	if err != nil || ip.String() != "192.168.1.20" {
		t.Fatalf("ip=%v err=%v", ip, err)
	}
	if _, err = Destination(context.Background(), net.DefaultResolver, "127.0.0.1", allowed); err == nil {
		t.Fatal("loopback permitido")
	}
	if _, err = Destination(context.Background(), net.DefaultResolver, "192.168.2.20", allowed); err == nil {
		t.Fatal("IP fora do CIDR permitido")
	}
}
func TestCIDRMasked(t *testing.T) {
	p, err := CIDR("10.20.30.9/24")
	if err != nil || p.String() != "10.20.30.0/24" {
		t.Fatalf("%v %v", p, err)
	}
}

func TestSpecialAddressesDenied(t *testing.T) {
	allowed := []netip.Prefix{netip.MustParsePrefix("0.0.0.0/0"), netip.MustParsePrefix("::/0")}
	for _, address := range []string{"::", "ff02::1", "255.255.255.255", "::ffff:127.0.0.1", "169.254.169.254", "fe80::1%eth0"} {
		if _, err := Destination(context.Background(), net.DefaultResolver, address, allowed); err == nil {
			t.Errorf("allowed %s", address)
		}
	}
}
