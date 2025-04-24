package userspacewg

import (
	"context"
	"net"

	"github.com/yago-123/wg-punch/pkg/peer"
	"github.com/yago-123/wg-punch/pkg/wg"
)

type userspaceWGTunnel struct {
}

func NewTunnel(cfg *wg.TunnelConfig) wg.Tunnel {
	return &userspaceWGTunnel{}
}

func (uwgt *userspaceWGTunnel) Start(_ context.Context, _ *net.UDPConn, _ peer.Info) error {
	return nil
}

func (uwgt *userspaceWGTunnel) PublicKey() string {
	return ""
}

func (uwgt *userspaceWGTunnel) ListenPort() int {
	return 0
}

func (uwgt *userspaceWGTunnel) Stop() error {
	return nil
}
