package wg

import (
	"context"
	"net"
	"time"

	"github.com/yago-123/wg-punch/pkg/peer"
)

type Tunnel interface {
	Start(ctx context.Context, conn *net.UDPConn, peer peer.Info) error
	PublicKey() string
	ListenPort() int
	Stop() error
}

type TunnelConfig struct {
	PrivKey           string
	Iface             string
	IfaceIPv4CIDR     string
	ListenPort        int
	ReplacePeer       bool
	CreateIface       bool
	KeepAliveInterval time.Duration
}
