package storage

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"time"
)

var ErrNotFound = errors.New("state file not found")

type State struct {
	LastIPv4   string                 `json:"last_ipv4"`
	LastIPv6   string                 `json:"last_ipv6"`
	LastUpdate string                 `json:"last_update"`
	Records    map[string]RecordState `json:"records,omitempty"`
}

type RecordState struct {
	LastIPv4   string `json:"last_ipv4"`
	LastUpdate string `json:"last_update"`
}

func Load(path string) (State, error) {
	if path == "" {
		return State{}, fmt.Errorf("state path is required")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return State{}, fmt.Errorf("%w: %s", ErrNotFound, path)
		}
		return State{}, fmt.Errorf("read state: %w", err)
	}

	var state State
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&state); err != nil {
		return State{}, fmt.Errorf("decode state: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return State{}, fmt.Errorf("decode state: multiple JSON values")
		}
		return State{}, fmt.Errorf("decode state: trailing data: %w", err)
	}
	if err := state.Validate(); err != nil {
		return State{}, fmt.Errorf("validate state: %w", err)
	}
	return state, nil
}

func (s State) Validate() error {
	if s.LastIPv4 != "" {
		ip := net.ParseIP(s.LastIPv4)
		if ip == nil || ip.To4() == nil {
			return fmt.Errorf("last_ipv4 is not a valid IPv4 address")
		}
	}
	if s.LastIPv6 != "" {
		ip := net.ParseIP(s.LastIPv6)
		if ip == nil || ip.To4() != nil {
			return fmt.Errorf("last_ipv6 is not a valid IPv6 address")
		}
	}
	if s.LastUpdate != "" {
		if _, err := time.Parse(time.RFC3339Nano, s.LastUpdate); err != nil {
			return fmt.Errorf("last_update is not a valid RFC3339 timestamp")
		}
	}
	for key, record := range s.Records {
		if key == "" {
			return fmt.Errorf("record state key cannot be empty")
		}
		if record.LastIPv4 != "" {
			ip := net.ParseIP(record.LastIPv4)
			if ip == nil || ip.To4() == nil {
				return fmt.Errorf("record %q has invalid last_ipv4", key)
			}
		}
		if record.LastUpdate != "" {
			if _, err := time.Parse(time.RFC3339Nano, record.LastUpdate); err != nil {
				return fmt.Errorf("record %q has invalid last_update", key)
			}
		}
	}
	return nil
}

func RecordKey(provider, domain, host, recordType string) string {
	return provider + "|" + domain + "|" + host + "|" + recordType
}

func (s State) RecordSynced(key, ip string) bool {
	record, ok := s.Records[key]
	return ok && record.LastIPv4 == ip
}

func (s *State) UpdateRecord(key string, ip net.IP, now time.Time) error {
	if key == "" {
		return fmt.Errorf("record state key is required")
	}
	if ip == nil || ip.To4() == nil {
		return fmt.Errorf("IP is not a valid IPv4 address")
	}
	if s.Records == nil {
		s.Records = make(map[string]RecordState)
	}
	s.Records[key] = RecordState{
		LastIPv4:   ip.To4().String(),
		LastUpdate: now.UTC().Format(time.RFC3339Nano),
	}
	return nil
}

func (s *State) UpdateIPv4(ip net.IP, now time.Time) error {
	if ip == nil || ip.To4() == nil {
		return fmt.Errorf("IP is not a valid IPv4 address")
	}
	s.LastIPv4 = ip.To4().String()
	s.LastUpdate = now.UTC().Format(time.RFC3339Nano)
	return nil
}

func Save(path string, state State) error {
	if path == "" {
		return fmt.Errorf("state path is required")
	}
	if err := state.Validate(); err != nil {
		return fmt.Errorf("validate state: %w", err)
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return fmt.Errorf("create state directory: %w", err)
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("encode state: %w", err)
	}
	data = append(data, '\n')

	temp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create temporary state file: %w", err)
	}
	tempName := temp.Name()
	defer os.Remove(tempName)
	if err := temp.Chmod(0600); err != nil {
		temp.Close()
		return fmt.Errorf("set state permissions: %w", err)
	}
	if _, err := temp.Write(data); err != nil {
		temp.Close()
		return fmt.Errorf("write temporary state: %w", err)
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		return fmt.Errorf("sync temporary state: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close temporary state: %w", err)
	}
	if err := os.Rename(tempName, path); err != nil {
		return fmt.Errorf("replace state file: %w", err)
	}
	dirFile, err := os.Open(dir)
	if err != nil {
		return fmt.Errorf("open state directory for sync: %w", err)
	}
	defer dirFile.Close()
	if err := dirFile.Sync(); err != nil {
		return fmt.Errorf("sync state directory: %w", err)
	}
	return nil
}
