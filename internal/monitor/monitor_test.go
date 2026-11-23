package monitor

import (
	"context"
	"errors"
	"net"
	"path/filepath"
	"testing"
	"time"

	"github.com/w00zzz/goddns/internal/config"
	"github.com/w00zzz/goddns/internal/provider"
	"github.com/w00zzz/goddns/internal/storage"
)

type fakeProvider struct {
	requests []provider.UpdateRequest
	err      error
}

func (p *fakeProvider) Name() string { return "fake" }

func (p *fakeProvider) UpdateRecord(_ context.Context, req provider.UpdateRequest) error {
	p.requests = append(p.requests, req)
	return p.err
}

func monitorConfig(path string) config.Config {
	return config.Config{
		General: config.GeneralConfig{Interval: time.Minute},
		DNS:     config.DNSConfig{Domain: "example.com", Host: "home"},
		State:   config.StateConfig{Path: path},
	}
}

func TestCheckSynchronizesInitialState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	p := &fakeProvider{}
	m := NewWithDependencies(monitorConfig(path), p, func(context.Context) (net.IP, error) {
		return net.ParseIP("198.51.100.10"), nil
	})

	result, err := m.Check(context.Background())
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}
	if !result.Changed || !result.Updated {
		t.Fatalf("unexpected result: %+v", result)
	}
	if len(p.requests) != 1 || p.requests[0].IP != "198.51.100.10" {
		t.Fatalf("unexpected provider requests: %+v", p.requests)
	}
	state, err := storage.Load(path)
	if err != nil || state.LastIPv4 != "198.51.100.10" {
		t.Fatalf("saved state = %+v, error = %v", state, err)
	}
}

func TestCheckDoesNotUpdateUnchangedIP(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	if err := storage.Save(path, storage.State{LastIPv4: "198.51.100.10", LastUpdate: "2026-01-02T03:04:05Z"}); err != nil {
		t.Fatal(err)
	}
	p := &fakeProvider{}
	m := NewWithDependencies(monitorConfig(path), p, func(context.Context) (net.IP, error) {
		return net.ParseIP("198.51.100.10"), nil
	})

	result, err := m.Check(context.Background())
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}
	if result.Changed || result.Updated || len(p.requests) != 0 {
		t.Fatalf("unexpected result: %+v, requests: %v", result, p.requests)
	}
}

func TestCheckUpdatesChangedIP(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	if err := storage.Save(path, storage.State{LastIPv4: "198.51.100.10", LastUpdate: "2026-01-02T03:04:05Z"}); err != nil {
		t.Fatal(err)
	}
	p := &fakeProvider{}
	m := NewWithDependencies(monitorConfig(path), p, func(context.Context) (net.IP, error) {
		return net.ParseIP("198.51.100.20"), nil
	})

	result, err := m.Check(context.Background())
	if err != nil || !result.Changed || !result.Updated {
		t.Fatalf("result = %+v, error = %v", result, err)
	}
	if len(p.requests) != 1 || p.requests[0].Host != "home" {
		t.Fatalf("unexpected provider requests: %+v", p.requests)
	}
}

func TestCheckPreservesStateWhenProviderFails(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	old := storage.State{LastIPv4: "198.51.100.10", LastUpdate: "2026-01-02T03:04:05Z"}
	if err := storage.Save(path, old); err != nil {
		t.Fatal(err)
	}
	p := &fakeProvider{err: errors.New("provider unavailable")}
	m := NewWithDependencies(monitorConfig(path), p, func(context.Context) (net.IP, error) {
		return net.ParseIP("198.51.100.20"), nil
	})

	if _, err := m.Check(context.Background()); err == nil {
		t.Fatal("Check() expected provider error")
	}
	got, err := storage.Load(path)
	if err != nil || got != old {
		t.Fatalf("state = %+v, error = %v; want previous state", got, err)
	}
}

func TestCheckReturnsDiscoveryError(t *testing.T) {
	p := &fakeProvider{}
	m := NewWithDependencies(monitorConfig(filepath.Join(t.TempDir(), "state.json")), p, func(context.Context) (net.IP, error) {
		return nil, errors.New("consensus unavailable")
	})
	if _, err := m.Check(context.Background()); err == nil {
		t.Fatal("Check() expected discovery error")
	}
	if len(p.requests) != 0 {
		t.Fatal("provider should not be called after discovery failure")
	}
}

func TestCheckDoesNotSaveWhenVerificationFails(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	p := &fakeProvider{}
	m := NewWithDependencies(monitorConfig(path), p, func(context.Context) (net.IP, error) {
		return net.ParseIP("198.51.100.10"), nil
	})
	m.SetVerifier(func(context.Context, string, net.IP) error {
		return errors.New("DNS propagation pending")
	})

	if _, err := m.Check(context.Background()); err == nil {
		t.Fatal("Check() expected verification error")
	}
	if _, err := storage.Load(path); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("state error = %v, want ErrNotFound", err)
	}
}
