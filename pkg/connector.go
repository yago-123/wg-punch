package wgpunch

import (
	"context"
	"net"
	"wg-punch/pkg/rendezvous/client"
)

type Connector struct {
	Puncher    Puncher
	Tunnel     Tunnel
	Rendezvous client.Rendezvous
}

func NewConnector(p Puncher, t Tunnel, r client.Rendezvous) *Connector {
	return &Connector{Puncher: p, Tunnel: t, Rendezvous: r}
}

func (c *Connector) Connect(_ context.Context, peerKey string, pubKey string) (net.Conn, error) {
	return nil, nil
}
