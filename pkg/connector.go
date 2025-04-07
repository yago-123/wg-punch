package wgpunch

import (
	"context"
	"net"
	"wg-punch/pkg/rendezvous/client"
)

type Connector struct {
	Puncher    Puncher
	Tunnel     Tunnel
	Rendezvous client.RendezvousClient
}

func NewConnector(p Puncher, t Tunnel, r client.RendezvousClient) *Connector {
	return &Connector{Puncher: p, Tunnel: t, Rendezvous: r}
}

func (c *Connector) Connect(ctx context.Context, peerKey string, pubKey string) (net.Conn, error) {
	return nil, nil
}
