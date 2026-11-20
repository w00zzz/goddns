package simply

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/w00zzz/goddns/internal/provider"
)

func TestProviderUpdateRecord(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, password, ok := r.BasicAuth()
		if !ok || user != "account" || password != "api-key" {
			t.Errorf("unexpected BasicAuth: user=%q password=%q ok=%v", user, password, ok)
		}
		if got := r.URL.Query().Get("hostname"); got != "home.example.com" {
			t.Errorf("hostname = %q, want home.example.com", got)
		}
		if got := r.URL.Query().Get("domain"); got != "example.com" {
			t.Errorf("domain = %q, want example.com", got)
		}
		if got := r.URL.Query().Get("myip"); got != "198.51.100.10" {
			t.Errorf("myip = %q, want 198.51.100.10", got)
		}
		_, _ = w.Write([]byte("good 198.51.100.10"))
	}))
	defer server.Close()

	p := New("account", "api-key", server.Client())
	p.client.ddnsURL = server.URL

	err := p.UpdateRecord(context.Background(), provider.UpdateRequest{
		Domain: "example.com",
		Host:   "home",
		IP:     "198.51.100.10",
	})
	if err != nil {
		t.Fatalf("UpdateRecord() error = %v", err)
	}
}

func TestProviderAcceptsNoChange(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("nochg 198.51.100.10"))
	}))
	defer server.Close()

	p := New("account", "api-key", server.Client())
	p.client.ddnsURL = server.URL
	if err := p.UpdateRecord(context.Background(), provider.UpdateRequest{
		Domain: "example.com",
		Host:   "home.example.com",
		IP:     "198.51.100.10",
	}); err != nil {
		t.Fatalf("UpdateRecord() error = %v", err)
	}
}

func TestRESTProviderFindsAndUpdatesRecord(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer api-key" {
			t.Errorf("unexpected authorization header: %q", r.Header.Get("Authorization"))
		}
		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/dns/records/"):
			_ = json.NewEncoder(w).Encode(dnsRecordsResponse{Records: []dnsRecord{{ID: 42, Name: "home.example.com", Type: "A", TTL: 300}}})
		case r.Method == http.MethodPut && strings.HasSuffix(r.URL.Path, "/dns/records/42/"):
			var payload dnsRecordPayload
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Errorf("decode payload: %v", err)
			}
			if payload.Data != "198.51.100.20" || payload.Name != "home.example.com" || payload.Type != "A" {
				t.Errorf("unexpected payload: %+v", payload)
			}
			w.WriteHeader(http.StatusOK)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	p := NewConfigured(Config{Mode: ModeREST, APIKey: "api-key", Product: "example.com", HTTPClient: server.Client()})
	p.client.apiURL = server.URL
	if err := p.UpdateRecord(context.Background(), provider.UpdateRequest{Domain: "example.com", Host: "home", IP: "198.51.100.20"}); err != nil {
		t.Fatalf("REST UpdateRecord() error = %v", err)
	}
}

func TestProviderRejectsSimplyErrors(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   string
		want   string
	}{
		{name: "authentication", status: http.StatusOK, body: "badauth", want: "badauth"},
		{name: "missing host", status: http.StatusOK, body: "nohost", want: "nohost"},
		{name: "http status", status: http.StatusBadGateway, body: "temporary failure", want: "HTTP status"},
		{name: "empty response", status: http.StatusOK, body: "", want: "empty response"},
		{name: "large response", status: http.StatusOK, body: strings.Repeat("x", maxResponseSize+1), want: "exceeds"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer server.Close()

			p := New("account", "api-key", server.Client())
			p.client.ddnsURL = server.URL
			err := p.UpdateRecord(context.Background(), provider.UpdateRequest{
				Domain: "example.com",
				Host:   "home",
				IP:     "198.51.100.10",
			})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("UpdateRecord() error = %v, want message containing %q", err, tt.want)
			}
		})
	}
}

func TestProviderHonorsContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	p := New("account", "api-key", server.Client())
	p.client.ddnsURL = server.URL
	if err := p.UpdateRecord(ctx, provider.UpdateRequest{Domain: "example.com", Host: "home", IP: "198.51.100.10"}); err == nil {
		t.Fatal("UpdateRecord() expected context error")
	}
}
