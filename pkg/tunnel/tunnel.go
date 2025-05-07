package tunnel

import (
	"context"
	"net"
	"time"

	"github.com/yago-123/wg-punch/pkg/peer"
)

type Tunnel interface {
	Start(ctx context.Context, conn *net.UDPConn, peer peer.Info, cancelPunch context.CancelFunc) error
	PublicKey() string
	ListenPort() int
	Stop(ctx context.Context) error
}

type Config struct {
	PrivKey           string
	Iface             string
	IfaceIPv4CIDR     string
	ListenPort        int
	ReplacePeer       bool
	CreateIface       bool
	KeepAliveInterval time.Duration
}
