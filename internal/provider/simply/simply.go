package simply

import (
	"context"
	"net/http"

	"github.com/w00zzz/goddns/internal/provider"
)

type Provider struct {
	client *Client
}

func New(account, apiKey string, httpClient *http.Client) *Provider {
	return &Provider{client: NewClient(account, apiKey, httpClient)}
}

func (p *Provider) Name() string {
	return "simply"
}

func (p *Provider) UpdateRecord(ctx context.Context, req provider.UpdateRequest) error {
	return p.client.update(ctx, req.Domain, req.Host, req.IP)
}
