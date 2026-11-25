package provider

import (
	"fmt"
	"strings"
)

type Registry struct {
	providers map[string]DNSProvider
}

func NewRegistry() *Registry {
	return &Registry{providers: make(map[string]DNSProvider)}
}

func (r *Registry) Register(p DNSProvider) error {
	if p == nil {
		return fmt.Errorf("dns provider cannot be nil")
	}
	name := strings.ToLower(strings.TrimSpace(p.Name()))
	if name == "" {
		return fmt.Errorf("dns provider name is required")
	}
	if _, exists := r.providers[name]; exists {
		return fmt.Errorf("dns provider %q is already registered", name)
	}
	r.providers[name] = p
	return nil
}

func (r *Registry) Get(name string) (DNSProvider, error) {
	key := strings.ToLower(strings.TrimSpace(name))
	p, ok := r.providers[key]
	if !ok {
		return nil, fmt.Errorf("dns provider %q is not registered", name)
	}
	return p, nil
}
