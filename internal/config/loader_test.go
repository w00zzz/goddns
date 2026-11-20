package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const validJSON = `{
  "general": {"interval": "10m", "log_level": "info"},
  "dns": {
    "provider": "simply",
    "domain": "example.com",
    "host": "home",
    "credentials": {"account": "account", "api_key": "secret"}
  },
  "discovery": {
    "ipv4": true,
    "ipv6": false,
    "sources": ["https://api.ipify.org", "https://icanhazip.com"],
    "min_consensus": 2
  },
  "state": {"path": "/tmp/goddns-state.json"}
}`

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatalf("write test configuration: %v", err)
	}
	return path
}

func TestLoad(t *testing.T) {
	cfg, err := Load(writeConfig(t, validJSON))
	if err != nil {
		t.Fatalf("load valid configuration: %v", err)
	}
	if cfg.General.Interval != 10*time.Minute {
		t.Fatalf("unexpected interval: %s", cfg.General.Interval)
	}
	if cfg.DNS.Host != "home" {
		t.Fatalf("unexpected host: %q", cfg.DNS.Host)
	}
}

func TestLoadErrors(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		content string
		want    string
	}{
		{name: "missing file", path: filepath.Join(t.TempDir(), "missing.json"), want: "read configuration"},
		{name: "invalid json", content: "{", want: "decode configuration"},
		{name: "invalid interval", content: strings.Replace(validJSON, `"10m"`, `"10x"`, 1), want: "decode configuration"},
		{name: "missing required field", content: strings.Replace(validJSON, `"path": "/tmp/goddns-state.json"`, `"path": ""`, 1), want: "validate configuration"},
		{name: "unknown field", content: strings.Replace(validJSON, `"log_level": "info"`, `"log_level": "info", "extra": true`, 1), want: "decode configuration"},
		{name: "trailing json", content: validJSON + "{}", want: "decode configuration"},
		{name: "malformed trailing json", content: validJSON + "{", want: "decode configuration"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := tt.path
			if tt.content != "" {
				path = writeConfig(t, tt.content)
			}
			_, err := Load(path)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Load() error = %v, want substring %q", err, tt.want)
			}
		})
	}
}

func TestLoadRejectsEmptyPath(t *testing.T) {
	if _, err := Load(""); err == nil {
		t.Fatal("expected empty path to be rejected")
	}
}
