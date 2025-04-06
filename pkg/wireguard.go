package wgpunch

import (
	"context"
	"net"
)

type Tunnel interface {
	Start(ctx context.Context, conn *net.UDPConn, localKey, remoteKey string, peer PeerInfo) error
	Close() error
}

type TunnelConfig struct {
	PrivateKey string
	Interface  string
	ListenPort int
}

func NewTunnel(cfg TunnelConfig) Tunnel {
	// return implementation with wireguard-go
	return nil
}
