package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExecuteVersion(t *testing.T) {
	var output bytes.Buffer
	if err := execute([]string{"version"}, &output, &output); err != nil {
		t.Fatalf("execute() error = %v", err)
	}
	if strings.TrimSpace(output.String()) != version {
		t.Fatalf("version output = %q, want %q", output.String(), version)
	}
}

func TestExecuteRejectsUnknownCommand(t *testing.T) {
	if err := execute([]string{"unknown"}, &bytes.Buffer{}, &bytes.Buffer{}); err == nil {
		t.Fatal("execute() expected unknown command error")
	}
}

func TestStatusCommandWithoutState(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.json")
	content := `{
		"general": {"interval": "10m", "log_level": "info"},
		"dns": {
			"provider": "simply",
			"domain": "example.com",
			"host": "home",
			"credentials": {"account": "account", "api_key": "secret"}
		},
		"discovery": {
			"ipv4": true,
			"sources": ["https://api.ipify.org"],
			"min_consensus": 1
		},
		"state": {"path": "` + filepath.Join(dir, "state.json") + `"}
	}`
	if err := os.WriteFile(configPath, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	var output bytes.Buffer
	if err := statusCommand([]string{"--config", configPath}, &output, &output); err != nil {
		t.Fatalf("statusCommand() error = %v", err)
	}
	if !strings.Contains(output.String(), `"last_ipv4": ""`) {
		t.Fatalf("status output = %q", output.String())
	}
}
