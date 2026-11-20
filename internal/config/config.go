package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"
)

type Config struct {
	General   GeneralConfig   `json:"general"`
	DNS       DNSConfig       `json:"dns"`
	Discovery DiscoveryConfig `json:"discovery"`
	State     StateConfig     `json:"state"`
}

type GeneralConfig struct {
	Interval time.Duration `json:"interval"`
	LogLevel string        `json:"log_level"`
}

// UnmarshalJSON accepts duration strings such as "10m" or "1h30m".
func (c *GeneralConfig) UnmarshalJSON(data []byte) error {
	var raw struct {
		Interval string `json:"interval"`
		LogLevel string `json:"log_level"`
	}

	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&raw); err != nil {
		return err
	}

	interval, err := time.ParseDuration(raw.Interval)
	if err != nil {
		return fmt.Errorf("invalid general.interval %q: %w", raw.Interval, err)
	}

	c.Interval = interval
	c.LogLevel = raw.LogLevel
	return nil
}

type DNSConfig struct {
	Provider    string            `json:"provider"`
	Domain      string            `json:"domain"`
	Host        string            `json:"host"`
	Credentials map[string]string `json:"credentials"`
}

type DiscoveryConfig struct {
	IPv4         bool     `json:"ipv4"`
	IPv6         bool     `json:"ipv6"`
	Sources      []string `json:"sources"`
	MinConsensus int      `json:"min_consensus"`
}

type StateConfig struct {
	Path string `json:"path"`
}
