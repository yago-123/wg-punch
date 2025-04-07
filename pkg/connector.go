package wgpunch

import (
	"context"
	"github.com/yago-123/wg-punch/pkg/rendezvous/client"
	"net"
)

type Connector struct {
	Puncher    Puncher
	Tunnel     Tunnel
	Rendezvous client.Rendezvous
}

func NewConnector(p Puncher, t Tunnel, r client.Rendezvous) *Connector {
	return &Connector{Puncher: p, Tunnel: t, Rendezvous: r}
}

func (c *Connector) Connect(_ context.Context, _, _ string) (net.Conn, error) {
	// ctx, peerKey, pubKey
	return nil, nil //nolint:nilnil //ignore this for now
}
