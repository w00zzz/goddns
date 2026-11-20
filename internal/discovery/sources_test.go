package discovery

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHTTPSourceGetIP(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(" 198.51.100.10\n"))
	}))
	defer server.Close()

	source := NewHTTPSource("test", server.URL, server.Client())
	ip, err := source.GetIP(context.Background())
	if err != nil {
		t.Fatalf("GetIP() error = %v", err)
	}
	if got := ip.String(); got != "198.51.100.10" {
		t.Fatalf("GetIP() = %s, want 198.51.100.10", got)
	}
	if source.Name() != "test" {
		t.Fatalf("Name() = %q, want test", source.Name())
	}
}

func TestHTTPSourceGetIPErrors(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   string
		want   string
	}{
		{name: "http status", status: http.StatusBadGateway, body: "198.51.100.10", want: "unexpected HTTP status"},
		{name: "invalid IP", status: http.StatusOK, body: "not an IP", want: "valid IPv4"},
		{name: "IPv6", status: http.StatusOK, body: "2001:db8::1", want: "valid IPv4"},
		{name: "large response", status: http.StatusOK, body: strings.Repeat("x", maxIPResponseSize+1), want: "exceeds"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer server.Close()

			_, err := NewHTTPSource("test", server.URL, server.Client()).GetIP(context.Background())
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("GetIP() error = %v, want message containing %q", err, tt.want)
			}
		})
	}
}

func TestHTTPSourceHonorsContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := NewHTTPSource("test", server.URL, server.Client()).GetIP(ctx)
	if err == nil {
		t.Fatal("GetIP() expected context cancellation error")
	}
}
