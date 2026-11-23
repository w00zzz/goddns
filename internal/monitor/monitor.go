package monitor

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
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
	log      *slog.Logger
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

func (m *Monitor) SetLogger(log *slog.Logger) {
	m.log = log
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
	if m.log != nil {
		m.log.Info("checking public IP")
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
		if m.log != nil {
			m.log.Info("IP unchanged", "ip", currentIPv4.String())
		}
		return result, nil
	}
	if m.log != nil {
		m.log.Info("IP changed", "previous", result.PreviousIP, "current", currentIPv4.String())
	}

	req := provider.UpdateRequest{
		Domain: m.config.DNS.Domain,
		Host:   m.config.DNS.Host,
		IP:     currentIPv4.String(),
	}
	if m.log != nil {
		m.log.Info("updating DNS record", "provider", m.provider.Name())
	}
	if err := m.provider.UpdateRecord(ctx, req); err != nil {
		if m.log != nil {
			m.log.Error("DNS update failed", "error", err)
		}
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
	if m.log != nil {
		m.log.Info("DNS update successful", "ip", currentIPv4.String())
	}
	return result, nil
}

func (m *Monitor) Run(ctx context.Context) error {
	if m.config.General.Interval <= 0 {
		return fmt.Errorf("monitor interval must be greater than zero")
	}
	if _, err := m.Check(ctx); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if m.log != nil {
			m.log.Error("initial monitor check failed", "error", err)
		}
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
			if _, err := m.Check(ctx); err != nil && m.log != nil {
				m.log.Error("monitor check failed", "error", err)
			}
		}
	}
}

func qualifiedHostname(domain, host string) string {
	if host == "@" || host == "" || host == domain {
		return domain
	}
	return host + "." + domain
}
