package dnscheck

import (
	"context"
	"net"
	"strings"
	"testing"
	"time"
)

type fakeResolver struct {
	results [][]net.IP
	calls   int
}

func (r *fakeResolver) LookupIP(context.Context, string, string) ([]net.IP, error) {
	result := r.results[r.calls]
	if r.calls < len(r.results)-1 {
		r.calls++
	}
	return result, nil
}

func TestVerify(t *testing.T) {
	resolver := &fakeResolver{results: [][]net.IP{{net.ParseIP("198.51.100.10")}}}
	if err := Verify(context.Background(), resolver, "home.example.com", net.ParseIP("198.51.100.10")); err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
}

func TestVerifyRejectsDifferentIP(t *testing.T) {
	resolver := &fakeResolver{results: [][]net.IP{{net.ParseIP("198.51.100.20")}}}
	err := Verify(context.Background(), resolver, "home.example.com", net.ParseIP("198.51.100.10"))
	if err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("Verify() error = %v, want mismatch error", err)
	}
}

func TestWaitForIPRetries(t *testing.T) {
	resolver := &fakeResolver{results: [][]net.IP{
		{net.ParseIP("198.51.100.20")},
		{net.ParseIP("198.51.100.10")},
	}}
	if err := WaitForIP(context.Background(), resolver, "home.example.com", net.ParseIP("198.51.100.10"), time.Millisecond); err != nil {
		t.Fatalf("WaitForIP() error = %v", err)
	}
	if resolver.calls != 1 {
		t.Fatalf("resolver calls = %d, want 1 retry", resolver.calls)
	}
}

func TestVerifyValidatesArguments(t *testing.T) {
	if err := Verify(context.Background(), nil, "", net.ParseIP("198.51.100.10")); err == nil {
		t.Fatal("Verify() expected hostname error")
	}
	if err := Verify(context.Background(), nil, "home.example.com", net.ParseIP("2001:db8::1")); err == nil {
		t.Fatal("Verify() expected IPv4 error")
	}
}
