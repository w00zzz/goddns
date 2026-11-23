package provider

import "context"

type UpdateRequest struct {
	Domain string
	Host   string
	Type   string
	IP     string
}

type DNSProvider interface {
	Name() string
	UpdateRecord(
		ctx context.Context,
		req UpdateRequest,
	) error
}
