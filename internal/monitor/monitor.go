package monitor

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/w00zzz/goddns/internal/config"
	"github.com/w00zzz/goddns/internal/discovery"
	"github.com/w00zzz/goddns/internal/provider"
	"github.com/w00zzz/goddns/internal/storage"
)

type DiscoverFunc func(context.Context) (net.IP, error)
type VerifyFunc func(context.Context, string, net.IP) error

type CheckResult struct {
	PreviousIP net.IP
	CurrentIP  net.IP
	Changed    bool
	Updated    bool
}

type Monitor struct {
	config   config.Config
	provider provider.DNSProvider
	discover DiscoverFunc
	verify   VerifyFunc
	now      func() time.Time
}

func New(cfg config.Config, dnsProvider provider.DNSProvider, sources []discovery.IPSource) *Monitor {
	return NewWithDependencies(cfg, dnsProvider, func(ctx context.Context) (net.IP, error) {
		return discovery.Discover(ctx, sources, cfg.Discovery.MinConsensus)
	})
}

func NewWithDependencies(cfg config.Config, dnsProvider provider.DNSProvider, discover DiscoverFunc) *Monitor {
	return &Monitor{
		config:   cfg,
		provider: dnsProvider,
		discover: discover,
		now:      time.Now,
	}
}

func (m *Monitor) SetVerifier(verify VerifyFunc) {
	m.verify = verify
}

func (m *Monitor) Check(ctx context.Context) (CheckResult, error) {
	if m.provider == nil {
		return CheckResult{}, fmt.Errorf("DNS provider is required")
	}
	if m.discover == nil {
		return CheckResult{}, fmt.Errorf("IP discovery function is required")
	}
	if err := ctx.Err(); err != nil {
		return CheckResult{}, err
	}

	state, err := storage.Load(m.config.State.Path)
	if err != nil && !errors.Is(err, storage.ErrNotFound) {
		return CheckResult{}, fmt.Errorf("load state: %w", err)
	}
	currentIP, err := m.discover(ctx)
	if err != nil {
		return CheckResult{}, fmt.Errorf("discover public IP: %w", err)
	}
	currentIPv4 := currentIP.To4()
	if currentIPv4 == nil {
		return CheckResult{}, fmt.Errorf("discovery returned an invalid IPv4 address")
	}

	result := CheckResult{CurrentIP: currentIPv4}
	if state.LastIPv4 != "" {
		result.PreviousIP = net.ParseIP(state.LastIPv4).To4()
	}
	result.Changed = result.PreviousIP == nil || !result.PreviousIP.Equal(currentIPv4)
	if !result.Changed {
		return result, nil
	}

	req := provider.UpdateRequest{
		Domain: m.config.DNS.Domain,
		Host:   m.config.DNS.Host,
		IP:     currentIPv4.String(),
	}
	if err := m.provider.UpdateRecord(ctx, req); err != nil {
		return result, fmt.Errorf("update DNS record: %w", err)
	}
	if m.verify != nil {
		hostname := qualifiedHostname(req.Domain, req.Host)
		if err := m.verify(ctx, hostname, currentIPv4); err != nil {
			return result, fmt.Errorf("verify DNS record: %w", err)
		}
	}

	if err := state.UpdateIPv4(currentIPv4, m.now()); err != nil {
		return result, fmt.Errorf("update state: %w", err)
	}
	if err := storage.Save(m.config.State.Path, state); err != nil {
		return result, fmt.Errorf("save state: %w", err)
	}
	result.Updated = true
	return result, nil
}

func (m *Monitor) Run(ctx context.Context) error {
	if m.config.General.Interval <= 0 {
		return fmt.Errorf("monitor interval must be greater than zero")
	}
	if _, err := m.Check(ctx); err != nil && ctx.Err() != nil {
		return ctx.Err()
	}

	ticker := time.NewTicker(m.config.General.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			// A failed check is intentionally not fatal. The previous state is
			// preserved and the next tick retries the operation.
			_, _ = m.Check(ctx)
		}
	}
}

func qualifiedHostname(domain, host string) string {
	if host == "@" || host == "" || host == domain {
		return domain
	}
	return host + "." + domain
}
