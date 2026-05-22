package netguard

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"net/url"
	"strings"
	"time"
)

var ErrUnsafeAddress = errors.New("unsafe outbound address")

const defaultDialTimeout = 10 * time.Second

func ValidateHTTPURL(raw string) error {
	parsed, err := url.ParseRequestURI(strings.TrimSpace(raw))
	if err != nil {
		return err
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("%w: unsupported scheme", ErrUnsafeAddress)
	}
	if parsed.User != nil {
		return fmt.Errorf("%w: userinfo is not allowed", ErrUnsafeAddress)
	}
	return ValidateHost(parsed.Hostname())
}

func ValidateHost(host string) error {
	host = normalizeHost(host)
	if host == "" {
		return fmt.Errorf("%w: empty host", ErrUnsafeAddress)
	}
	if host == "localhost" || strings.HasSuffix(host, ".localhost") || strings.HasSuffix(host, ".local") {
		return fmt.Errorf("%w: local host is not allowed", ErrUnsafeAddress)
	}
	if addr, err := netip.ParseAddr(host); err == nil {
		return validateAddr(addr)
	}
	return nil
}

func DialContext(ctx context.Context, network string, address string) (net.Conn, error) {
	return Dialer{Timeout: defaultDialTimeout}.DialContext(ctx, network, address)
}

type Dialer struct {
	Timeout  time.Duration
	Resolver *net.Resolver
}

func (d Dialer) DialContext(ctx context.Context, network string, address string) (net.Conn, error) {
	if network != "tcp" && network != "tcp4" && network != "tcp6" {
		return nil, fmt.Errorf("%w: unsupported network %s", ErrUnsafeAddress, network)
	}

	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, fmt.Errorf("split outbound address: %w", err)
	}
	if err := ValidateHost(host); err != nil {
		return nil, err
	}

	addrs, err := d.lookup(ctx, host)
	if err != nil {
		return nil, err
	}
	if len(addrs) == 0 {
		return nil, fmt.Errorf("resolve outbound host: no addresses for %s", host)
	}

	timeout := d.Timeout
	if timeout <= 0 {
		timeout = defaultDialTimeout
	}
	dialer := &net.Dialer{Timeout: timeout}
	var lastErr error
	for _, addr := range addrs {
		if err := validateAddr(addr); err != nil {
			return nil, err
		}
		if network == "tcp4" && !addr.Is4() {
			continue
		}
		if network == "tcp6" && addr.Is4() {
			continue
		}
		conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(addr.String(), port))
		if err == nil {
			return conn, nil
		}
		lastErr = err
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fmt.Errorf("resolve outbound host: no %s address for %s", network, host)
}

func (d Dialer) lookup(ctx context.Context, host string) ([]netip.Addr, error) {
	if addr, err := netip.ParseAddr(normalizeHost(host)); err == nil {
		return []netip.Addr{addr}, nil
	}
	resolver := d.Resolver
	if resolver == nil {
		resolver = net.DefaultResolver
	}
	addrs, err := resolver.LookupNetIP(ctx, "ip", host)
	if err != nil {
		return nil, fmt.Errorf("resolve outbound host: %w", err)
	}
	return addrs, nil
}

func validateAddr(addr netip.Addr) error {
	addr = addr.Unmap()
	if addr.IsLoopback() ||
		addr.IsPrivate() ||
		addr.IsLinkLocalUnicast() ||
		addr.IsLinkLocalMulticast() ||
		addr.IsInterfaceLocalMulticast() ||
		addr.IsMulticast() ||
		addr.IsUnspecified() {
		return fmt.Errorf("%w: %s", ErrUnsafeAddress, addr.String())
	}
	return nil
}

func normalizeHost(host string) string {
	host = strings.TrimSpace(strings.TrimSuffix(host, "."))
	return strings.ToLower(strings.Trim(host, "[]"))
}
