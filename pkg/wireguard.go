package wgpunch

import (
	"context"
	"github.com/yago-123/wg-punch/pkg/peer"
	"net"
)

type Tunnel interface {
	Start(ctx context.Context, conn *net.UDPConn, localKey, remoteKey string, peer peer.Info) error
	Close() error
}

type TunnelConfig struct {
	PrivateKey string
	Interface  string
	ListenPort int
}

func NewTunnel(_ *TunnelConfig) Tunnel {
	// return implementation with wireguard-go
	return nil
}
