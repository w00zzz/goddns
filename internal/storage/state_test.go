package storage

import (
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoadReturnsNotFound(t *testing.T) {
	_, err := Load(filepath.Join(t.TempDir(), "missing.json"))
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Load() error = %v, want ErrNotFound", err)
	}
}

func TestSaveAndLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "state.json")
	want := State{LastIPv4: "198.51.100.10", LastUpdate: "2026-01-02T03:04:05Z"}

	if err := Save(path, want); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got != want {
		t.Fatalf("Load() = %+v, want %+v", got, want)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	if info.Mode().Perm()&0077 != 0 {
		t.Fatalf("state permissions = %o, should not be accessible by group/others", info.Mode().Perm())
	}
}

func TestSaveReplacesExistingStateAtomically(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	first := State{LastIPv4: "198.51.100.10", LastUpdate: "2026-01-02T03:04:05Z"}
	second := State{LastIPv4: "198.51.100.20", LastUpdate: "2026-01-02T03:05:05Z"}

	if err := Save(path, first); err != nil {
		t.Fatalf("first Save() error = %v", err)
	}
	if err := Save(path, second); err != nil {
		t.Fatalf("second Save() error = %v", err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got != second {
		t.Fatalf("Load() = %+v, want %+v", got, second)
	}
}

func TestLoadRejectsInvalidState(t *testing.T) {
	tests := []struct {
		name string
		data string
		want string
	}{
		{name: "invalid json", data: "{", want: "decode state"},
		{name: "unknown field", data: `{"unexpected": true}`, want: "decode state"},
		{name: "trailing json", data: `{"last_ipv4":"198.51.100.10"}{}`, want: "multiple JSON values"},
		{name: "invalid IPv4", data: `{"last_ipv4":"not-an-ip"}`, want: "last_ipv4"},
		{name: "invalid timestamp", data: `{"last_update":"yesterday"}`, want: "last_update"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "state.json")
			if err := os.WriteFile(path, []byte(tt.data), 0600); err != nil {
				t.Fatalf("WriteFile() error = %v", err)
			}
			_, err := Load(path)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Load() error = %v, want message containing %q", err, tt.want)
			}
		})
	}
}

func TestSaveRejectsInvalidStateAndPreservesPreviousFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	valid := State{LastIPv4: "198.51.100.10", LastUpdate: "2026-01-02T03:04:05Z"}
	if err := Save(path, valid); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if err := Save(path, State{LastIPv4: "invalid"}); err == nil {
		t.Fatal("Save() expected invalid state error")
	}
	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got != valid {
		t.Fatalf("previous state was not preserved: got %+v, want %+v", got, valid)
	}
}

func TestUpdateIPv4(t *testing.T) {
	var state State
	now := time.Date(2026, time.January, 2, 3, 4, 5, 123, time.FixedZone("test", 3600))
	if err := state.UpdateIPv4(net.ParseIP("198.51.100.10"), now); err != nil {
		t.Fatalf("UpdateIPv4() error = %v", err)
	}
	if state.LastIPv4 != "198.51.100.10" {
		t.Fatalf("LastIPv4 = %q", state.LastIPv4)
	}
	if state.LastUpdate != "2026-01-02T02:04:05.000000123Z" {
		t.Fatalf("LastUpdate = %q", state.LastUpdate)
	}
	if err := state.UpdateIPv4(net.ParseIP("2001:db8::1"), now); err == nil {
		t.Fatal("UpdateIPv4() expected IPv4 validation error")
	}
}
