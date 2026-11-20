package config

import (
	"testing"
	"time"
)

func validConfig() Config {
	return Config{
		General: GeneralConfig{Interval: 10 * time.Minute, LogLevel: "info"},
		DNS: DNSConfig{
			Provider: "simply",
			Domain:   "example.com",
			Host:     "home",
			Credentials: map[string]string{
				"account": "account",
				"api_key": "secret",
			},
		},
		Discovery: DiscoveryConfig{
			IPv4:         true,
			Sources:      []string{"https://api.ipify.org", "https://icanhazip.com"},
			MinConsensus: 2,
		},
		State: StateConfig{Path: "/tmp/goddns-state.json"},
	}
}

func TestConfigValidate(t *testing.T) {
	if err := validConfig().Validate(); err != nil {
		t.Fatalf("valid configuration rejected: %v", err)
	}
}

func TestConfigValidateRejectsInvalidValues(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Config)
	}{
		{"interval", func(c *Config) { c.General.Interval = 0 }},
		{"log level", func(c *Config) { c.General.LogLevel = "trace" }},
		{"provider", func(c *Config) { c.DNS.Provider = "cloudflare" }},
		{"domain", func(c *Config) { c.DNS.Domain = "" }},
		{"host", func(c *Config) { c.DNS.Host = "" }},
		{"account", func(c *Config) { delete(c.DNS.Credentials, "account") }},
		{"api key", func(c *Config) { delete(c.DNS.Credentials, "api_key") }},
		{"sources", func(c *Config) { c.Discovery.Sources = nil }},
		{"consensus below one", func(c *Config) { c.Discovery.MinConsensus = 0 }},
		{"consensus above sources", func(c *Config) { c.Discovery.MinConsensus = 3 }},
		{"source URL", func(c *Config) { c.Discovery.Sources[0] = "not-a-url" }},
		{"state path", func(c *Config) { c.State.Path = "" }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validConfig()
			tt.mutate(&cfg)
			if err := cfg.Validate(); err == nil {
				t.Fatalf("expected validation error")
			}
		})
	}
}
