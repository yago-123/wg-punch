package userspacewg

import (
	"context"
	"github.com/yago-123/wg-punch/pkg/peer"
	"github.com/yago-123/wg-punch/pkg/wg"
	"net"
)

type userspaceWGTunnel struct {
}

func NewTunnel(cfg *wg.TunnelConfig) wg.Tunnel {
	return &userspaceWGTunnel{}
}

func (uwgt *userspaceWGTunnel) Start(ctx context.Context, conn *net.UDPConn, localPrivKey string, peer peer.Info) error {
	return nil
}

func (uwgt *userspaceWGTunnel) Stop() error {
	return nil
}
