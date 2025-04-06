package main

import (
	"context"
	"net"
)

type Connector struct {
	Puncher    Puncher
	Tunnel     Tunnel
	Rendezvous RendezvousClient
}

func NewConnector(p Puncher, t Tunnel, r RendezvousClient) *Connector {
	return &Connector{Puncher: p, Tunnel: t, Rendezvous: r}
}

func (c *Connector) Connect(ctx context.Context, peerKey string) (net.Conn, error) {
	return nil, nil
}
