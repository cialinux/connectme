package validation

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"strings"
)

var blocked = []netip.Prefix{netip.MustParsePrefix("127.0.0.0/8"), netip.MustParsePrefix("169.254.0.0/16"), netip.MustParsePrefix("0.0.0.0/8"), netip.MustParsePrefix("224.0.0.0/4"), netip.MustParsePrefix("::1/128"), netip.MustParsePrefix("fe80::/10"), netip.MustParsePrefix("fd00:ec2::254/128")}

func CIDR(value string) (netip.Prefix, error) {
	p, err := netip.ParsePrefix(strings.TrimSpace(value))
	if err != nil {
		return p, fmt.Errorf("CIDR inválido: %w", err)
	}
	return p.Masked(), nil
}
func Port(port int) error {
	if port < 1 || port > 65535 {
		return errors.New("porta fora do intervalo 1-65535")
	}
	return nil
}
func Destination(ctx context.Context, resolver *net.Resolver, target string, allowed []netip.Prefix) (netip.Addr, error) {
	if len(allowed) == 0 {
		return netip.Addr{}, errors.New("nenhum CIDR autorizado")
	}
	target = strings.TrimSpace(target)
	var ips []netip.Addr
	if ip, err := netip.ParseAddr(target); err == nil {
		ips = []netip.Addr{ip.Unmap()}
	} else {
		if strings.ContainsAny(target, "/\\@:#") || len(target) > 253 {
			return netip.Addr{}, errors.New("hostname inválido")
		}
		resolved, err := resolver.LookupNetIP(ctx, "ip", target)
		if err != nil {
			return netip.Addr{}, fmt.Errorf("resolver destino: %w", err)
		}
		for _, ip := range resolved {
			ips = append(ips, ip.Unmap())
		}
	}
	if len(ips) == 0 {
		return netip.Addr{}, errors.New("destino sem endereço")
	}
	for _, ip := range ips {
		if ip.IsUnspecified() || ip.IsMulticast() || ip.Zone() != "" || !ip.IsGlobalUnicast() {
			return netip.Addr{}, errors.New("endereço não unicast bloqueado")
		}
		for _, p := range blocked {
			if p.Contains(ip) {
				return netip.Addr{}, fmt.Errorf("endereço bloqueado: %s", ip)
			}
		}
		ok := false
		for _, p := range allowed {
			if p.Contains(ip) {
				ok = true
				break
			}
		}
		if !ok {
			return netip.Addr{}, fmt.Errorf("endereço fora dos CIDRs autorizados: %s", ip)
		}
	}
	return ips[0], nil
}
