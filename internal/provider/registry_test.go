package provider

import (
	"context"
	"testing"
)

type fakeProvider struct{ name string }

func (p fakeProvider) Name() string { return p.name }

func (p fakeProvider) UpdateRecord(context.Context, UpdateRequest) error { return nil }

func TestRegistryRegisterAndGet(t *testing.T) {
	registry := NewRegistry()
	p := fakeProvider{name: "Simply"}

	if err := registry.Register(p); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	got, err := registry.Get(" simply ")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.Name() != "Simply" {
		t.Fatalf("Get() provider = %q, want Simply", got.Name())
	}
}

func TestRegistryRejectsInvalidRegistrations(t *testing.T) {
	registry := NewRegistry()

	if err := registry.Register(nil); err == nil {
		t.Fatal("Register(nil) expected an error")
	}
	if err := registry.Register(fakeProvider{}); err == nil {
		t.Fatal("Register() with empty name expected an error")
	}
	if err := registry.Register(fakeProvider{name: "simply"}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if err := registry.Register(fakeProvider{name: "Simply"}); err == nil {
		t.Fatal("duplicate Register() expected an error")
	}
}

func TestRegistryReturnsErrorForUnknownProvider(t *testing.T) {
	_, err := NewRegistry().Get("simply")
	if err == nil {
		t.Fatal("Get() unknown provider expected an error")
	}
}
