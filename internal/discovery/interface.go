package discovery

import (
	"context"
	"net"
)

type IPSource interface {
	Name()	string
	GetIP(ctx context.Context)	(net.IP, error)
}
