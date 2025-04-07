package wg

import (
	"context"
	"fmt"
	"net"
	"time"

	"golang.zx2c4.com/wireguard/wgctrl"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"

	"github.com/yago-123/wg-punch/pkg/peer"
)

type Tunnel interface {
	Start(ctx context.Context, conn *net.UDPConn, localPrivKey string, peer peer.Info) error
	Close() error
}

type TunnelConfig struct {
	PrivateKey        string
	Interface         string
	ListenPort        int
	ReplacePeer       bool
	KeepAliveInterval time.Duration
}

type wgTunnel struct {
	config   *TunnelConfig
	listener *net.UDPConn
}

func NewTunnel(cfg *TunnelConfig) Tunnel {
	return &wgTunnel{
		config: cfg,
	}
}

func (wgt *wgTunnel) Start(ctx context.Context, conn *net.UDPConn, localPrivKey string, peer peer.Info) error {
	client, err := wgctrl.New()
	if err != nil {
		return fmt.Errorf("failed to open wgctrl client: %w", err)
	}
	defer client.Close()

	privKey, err := wgtypes.ParseKey(localPrivKey)
	if err != nil {
		return fmt.Errorf("invalid private key: %w", err)
	}

	pubKey, err := wgtypes.ParseKey(peer.PublicKey)
	if err != nil {
		return fmt.Errorf("invalid remote public key: %w", err)
	}

	cfg := wgtypes.Config{
		PrivateKey:   &privKey,
		ListenPort:   &wgt.config.ListenPort,
		ReplacePeers: wgt.config.ReplacePeer,
		Peers: []wgtypes.PeerConfig{
			{
				PublicKey:                   pubKey,
				Endpoint:                    peer.Endpoint,
				AllowedIPs:                  peer.AllowedIPs,
				PersistentKeepaliveInterval: &wgt.config.KeepAliveInterval,
			},
		},
	}

	if errDevice := client.ConfigureDevice(wgt.config.Interface, cfg); errDevice != nil {
		return fmt.Errorf("failed to configure device: %w", errDevice)
	}

	wgt.listener = conn
	return nil
}

func (wgt *wgTunnel) Close() error {
	client, err := wgctrl.New()
	if err != nil {
		return fmt.Errorf("failed to open wgctrl client: %w", err)
	}
	defer client.Close()

	return client.ConfigureDevice(wgt.config.Interface, wgtypes.Config{
		ReplacePeers: wgt.config.ReplacePeer, // Clears all peers
	})
}
