package dnscheck

import (
	"context"
	"fmt"
	"net"
	"time"
)

type Resolver interface {
	LookupIP(ctx context.Context, network, host string) ([]net.IP, error)
}

// Verify checks that the public DNS resolver returns the expected IPv4 address.
func Verify(ctx context.Context, resolver Resolver, hostname string, expected net.IP) error {
	if hostname == "" {
		return fmt.Errorf("hostname is required")
	}
	want := expected.To4()
	if want == nil {
		return fmt.Errorf("expected address must be a valid IPv4 address")
	}
	if resolver == nil {
		resolver = net.DefaultResolver
	}

	ips, err := resolver.LookupIP(ctx, "ip4", hostname)
	if err != nil {
		return fmt.Errorf("lookup %q: %w", hostname, err)
	}
	for _, ip := range ips {
		if ip.To4() != nil && ip.Equal(want) {
			return nil
		}
	}
	return fmt.Errorf("DNS record %q does not match expected IP %s", hostname, want)
}

// WaitForIP retries DNS verification until it succeeds or the context expires.
func WaitForIP(ctx context.Context, resolver Resolver, hostname string, expected net.IP, interval time.Duration) error {
	if interval <= 0 {
		interval = time.Second
	}
	var lastErr error
	for {
		if err := Verify(ctx, resolver, hostname, expected); err == nil {
			return nil
		} else {
			lastErr = err
		}

		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return fmt.Errorf("DNS verification timed out: %w (last error: %v)", ctx.Err(), lastErr)
		case <-timer.C:
		}
	}
}
