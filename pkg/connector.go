package wgpunch

import (
	"context"
	"net"

	"github.com/yago-123/wg-punch/pkg/puncher"
	"github.com/yago-123/wg-punch/pkg/wg"

	"github.com/yago-123/wg-punch/pkg/rendez/client"
)

type Connector struct {
	Puncher    puncher.Puncher
	Tunnel     wg.Tunnel
	Rendezvous client.Rendezvous
}

func NewConnector(p puncher.Puncher, t wg.Tunnel, r client.Rendezvous) *Connector {
	return &Connector{Puncher: p, Tunnel: t, Rendezvous: r}
}

func (c *Connector) Connect(_ context.Context, _, _ string) (net.Conn, error) {
	// ctx, peerKey, pubKey
	return nil, nil //nolint:nilnil //ignore this for now
}
