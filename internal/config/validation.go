package config

import (
	"fmt"
	"net/url"
	"strings"
)

var validLogLevels = map[string]bool{
	"debug": true,
	"info":  true,
	"warn":  true,
	"error": true,
}

// Validate checks that the configuration contains all values required by the MVP.
func (c Config) Validate() error {
	if c.General.Interval <= 0 {
		return fmt.Errorf("general.interval must be greater than zero")
	}

	logLevel := strings.ToLower(strings.TrimSpace(c.General.LogLevel))
	if !validLogLevels[logLevel] {
		return fmt.Errorf("general.log_level must be one of: debug, info, warn, error")
	}

	provider := strings.ToLower(strings.TrimSpace(c.DNS.Provider))
	if provider == "" {
		return fmt.Errorf("dns.provider is required")
	}
	if provider != "simply" {
		return fmt.Errorf("dns.provider %q is not available", provider)
	}
	if strings.TrimSpace(c.DNS.Domain) == "" {
		return fmt.Errorf("dns.domain is required")
	}
	if strings.TrimSpace(c.DNS.Host) == "" {
		return fmt.Errorf("dns.host is required")
	}
	if strings.TrimSpace(c.DNS.Credentials["account"]) == "" {
		return fmt.Errorf("dns.credentials.account is required")
	}
	if strings.TrimSpace(c.DNS.Credentials["api_key"]) == "" {
		return fmt.Errorf("dns.credentials.api_key is required")
	}

	if !c.Discovery.IPv4 && !c.Discovery.IPv6 {
		return fmt.Errorf("discovery must enable ipv4 or ipv6")
	}
	if len(c.Discovery.Sources) == 0 {
		return fmt.Errorf("discovery.sources must contain at least one source")
	}
	if c.Discovery.MinConsensus < 1 {
		return fmt.Errorf("discovery.min_consensus must be at least 1")
	}
	if c.Discovery.MinConsensus > len(c.Discovery.Sources) {
		return fmt.Errorf("discovery.min_consensus cannot exceed the number of sources")
	}
	for i, source := range c.Discovery.Sources {
		u, err := url.ParseRequestURI(strings.TrimSpace(source))
		if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
			return fmt.Errorf("discovery.sources[%d] must be a valid HTTP or HTTPS URL", i)
		}
	}

	if strings.TrimSpace(c.State.Path) == "" {
		return fmt.Errorf("state.path is required")
	}

	return nil
}
