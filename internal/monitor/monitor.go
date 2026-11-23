package monitor

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"strings"
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
	return &Monitor{config: cfg, provider: dnsProvider, discover: discover, now: time.Now}
}

func (m *Monitor) SetLogger(log *slog.Logger)    { m.log = log }
func (m *Monitor) SetVerifier(verify VerifyFunc) { m.verify = verify }

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
	targets := configuredRecords(m.config)
	var updateErrors []error
	allSynced := true
	for _, target := range targets {
		key := storage.RecordKey(m.provider.Name(), target.Domain, target.Host, target.Type)
		if state.RecordSynced(key, currentIPv4.String()) {
			if m.log != nil {
				m.log.Info("DNS record unchanged", "domain", target.Domain, "host", target.Host, "ip", currentIPv4.String())
			}
			continue
		}

		result.Changed = true
		allSynced = false
		req := provider.UpdateRequest{Domain: target.Domain, Host: target.Host, Type: target.Type, IP: currentIPv4.String()}
		if m.log != nil {
			m.log.Info("updating DNS record", "provider", m.provider.Name(), "domain", target.Domain, "host", target.Host, "type", target.Type)
		}
		if err := m.provider.UpdateRecord(ctx, req); err != nil {
			if m.log != nil {
				m.log.Error("DNS update failed", "domain", target.Domain, "host", target.Host, "error", err)
			}
			updateErrors = append(updateErrors, fmt.Errorf("%s/%s: %w", target.Domain, target.Host, err))
			continue
		}
		if m.verify != nil {
			hostname := qualifiedHostname(target.Domain, target.Host)
			if err := m.verify(ctx, hostname, currentIPv4); err != nil {
				updateErrors = append(updateErrors, fmt.Errorf("verify %s: %w", hostname, err))
				continue
			}
		}
		if err := state.UpdateRecord(key, currentIPv4, m.now()); err != nil {
			updateErrors = append(updateErrors, fmt.Errorf("record state %s/%s: %w", target.Domain, target.Host, err))
			continue
		}
		if err := storage.Save(m.config.State.Path, state); err != nil {
			updateErrors = append(updateErrors, fmt.Errorf("save state after %s/%s: %w", target.Domain, target.Host, err))
			continue
		}
		result.Updated = true
		if m.log != nil {
			m.log.Info("DNS update successful", "domain", target.Domain, "host", target.Host, "ip", currentIPv4.String())
		}
	}

	if len(updateErrors) > 0 {
		return result, errors.Join(updateErrors...)
	}
	if allSynced || result.Updated {
		if state.LastIPv4 != currentIPv4.String() {
			if err := state.UpdateIPv4(currentIPv4, m.now()); err != nil {
				return result, fmt.Errorf("update state: %w", err)
			}
			if err := storage.Save(m.config.State.Path, state); err != nil {
				return result, fmt.Errorf("save state: %w", err)
			}
		}
	}
	if !result.Changed && m.log != nil {
		m.log.Info("IP unchanged", "ip", currentIPv4.String())
	}
	return result, nil
}

func configuredRecords(cfg config.Config) []config.DNSRecordConfig {
	if len(cfg.DNS.Records) > 0 {
		records := make([]config.DNSRecordConfig, len(cfg.DNS.Records))
		copy(records, cfg.DNS.Records)
		for i := range records {
			if records[i].Domain == "" {
				records[i].Domain = cfg.DNS.Domain
			}
			if records[i].Type == "" {
				records[i].Type = "A"
			}
		}
		return records
	}
	return []config.DNSRecordConfig{{Domain: cfg.DNS.Domain, Host: cfg.DNS.Host, Type: "A"}}
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
			if _, err := m.Check(ctx); err != nil && m.log != nil {
				m.log.Error("monitor check failed", "error", err)
			}
		}
	}
}

func qualifiedHostname(domain, host string) string {
	if host == "@" || host == "" || strings.EqualFold(host, domain) {
		return domain
	}
	if strings.HasSuffix(strings.ToLower(host), "."+strings.ToLower(domain)) {
		return host
	}
	return host + "." + domain
}
