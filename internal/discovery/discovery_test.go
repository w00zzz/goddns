package discovery

import (
	"context"
	"errors"
	"net"
	"strings"
	"testing"
)

type fakeSource struct {
	name string
	ip   string
	err  error
}

func (s fakeSource) Name() string { return s.name }

func (s fakeSource) GetIP(ctx context.Context) (net.IP, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s.err != nil {
		return nil, s.err
	}
	return net.ParseIP(s.ip), nil
}

func TestDiscoverReturnsConsensualIP(t *testing.T) {
	sources := []IPSource{
		fakeSource{name: "one", ip: "198.51.100.10"},
		fakeSource{name: "two", ip: "198.51.100.10"},
		fakeSource{name: "three", ip: "198.51.100.20"},
	}

	ip, err := Discover(context.Background(), sources, 2)
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}
	if got := ip.String(); got != "198.51.100.10" {
		t.Fatalf("Discover() = %s, want 198.51.100.10", got)
	}
}

func TestDiscoverIgnoresFailedSources(t *testing.T) {
	sources := []IPSource{
		fakeSource{name: "one", ip: "198.51.100.10"},
		fakeSource{name: "two", err: errors.New("source unavailable")},
		fakeSource{name: "three", ip: "198.51.100.10"},
	}

	if _, err := Discover(context.Background(), sources, 2); err != nil {
		t.Fatalf("Discover() error = %v", err)
	}
}

func TestDiscoverRejectsAmbiguousResults(t *testing.T) {
	sources := []IPSource{
		fakeSource{name: "one", ip: "198.51.100.10"},
		fakeSource{name: "two", ip: "198.51.100.20"},
		fakeSource{name: "three", ip: "198.51.100.30"},
	}

	_, err := Discover(context.Background(), sources, 2)
	if err == nil || !strings.Contains(err.Error(), "consensus not reached") {
		t.Fatalf("Discover() error = %v, want consensus error", err)
	}
}

func TestDiscoverValidatesArguments(t *testing.T) {
	source := fakeSource{name: "one", ip: "198.51.100.10"}

	for _, minimum := range []int{0, 2} {
		if _, err := Discover(context.Background(), []IPSource{source}, minimum); err == nil {
			t.Fatalf("Discover() with minimum %d expected an error", minimum)
		}
	}
	if _, err := Discover(context.Background(), nil, 1); err == nil {
		t.Fatal("Discover() without sources expected an error")
	}
}

func TestDiscoverHonorsContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := Discover(ctx, []IPSource{fakeSource{name: "one", ip: "198.51.100.10"}}, 1)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Discover() error = %v, want context.Canceled", err)
	}
}
