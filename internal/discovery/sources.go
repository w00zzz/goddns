package discovery

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

const (
	defaultHTTPTimeout = 10 * time.Second
	maxIPResponseSize  = 4 * 1024
)

type HTTPSource struct {
	name   string
	url    string
	client *http.Client
}

func NewHTTPSource(name, endpoint string, client *http.Client) *HTTPSource {
	if client == nil {
		client = &http.Client{Timeout: defaultHTTPTimeout}
	}
	if strings.TrimSpace(name) == "" {
		name = endpoint
	}
	return &HTTPSource{name: name, url: endpoint, client: client}
}

func (s *HTTPSource) Name() string {
	return s.name
}

func (s *HTTPSource) GetIP(ctx context.Context) (net.IP, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected HTTP status: %s", resp.Status)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxIPResponseSize+1))
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	if len(body) > maxIPResponseSize {
		return nil, fmt.Errorf("response exceeds %d bytes", maxIPResponseSize)
	}

	ip := net.ParseIP(strings.TrimSpace(string(body)))
	if ip == nil || ip.To4() == nil {
		return nil, fmt.Errorf("response is not a valid IPv4 address")
	}
	return ip.To4(), nil
}

func NewSources(endpoints []string, client *http.Client) []IPSource {
	sources := make([]IPSource, 0, len(endpoints))
	for _, endpoint := range endpoints {
		sources = append(sources, NewHTTPSource(endpoint, endpoint, client))
	}
	return sources
}
