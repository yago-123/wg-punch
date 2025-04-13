package wg

import (
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/vishvananda/netlink"

	"golang.zx2c4.com/wireguard/wgctrl"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"

	"github.com/yago-123/wg-punch/pkg/peer"
)

const (
	WireGuardLinkType = "wireguard"
)

type Tunnel interface {
	Start(conn *net.UDPConn, localPrivKey string, peer peer.Info) error
	Close() error
}

type TunnelConfig struct {
	PrivateKey        string
	Iface             string
	ListenPort        int
	ReplacePeer       bool
	CreateIface       bool
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

func (wgt *wgTunnel) Start(conn *net.UDPConn, localPrivKey string, peer peer.Info) error {
	client, err := wgctrl.New()
	if err != nil {
		return fmt.Errorf("failed to open wgctrl client: %w", err)
	}
	defer client.Close()

	privKey, err := wgtypes.ParseKey(localPrivKey)
	if err != nil {
		return fmt.Errorf("invalid private key: %w", err)
	}

	remotePubKey, err := wgtypes.ParseKey(peer.PublicKey)
	if err != nil {
		return fmt.Errorf("invalid remote public key: %w", err)
	}

	cfg := wgtypes.Config{
		PrivateKey:   &privKey,
		ListenPort:   &wgt.config.ListenPort,
		ReplacePeers: wgt.config.ReplacePeer,
		Peers: []wgtypes.PeerConfig{
			{
				PublicKey:                   remotePubKey,
				Endpoint:                    peer.Endpoint,
				AllowedIPs:                  peer.AllowedIPs,
				PersistentKeepaliveInterval: &wgt.config.KeepAliveInterval,
			},
		},
	}

	if err = wgt.ensureInterfaceExists(wgt.config.Iface); err != nil {
		return fmt.Errorf("failed to ensure interface exists: %w", err)
	}

	if errDevice := client.ConfigureDevice(wgt.config.Iface, cfg); errDevice != nil {
		return fmt.Errorf("failed to configure device: %w", errDevice)
	}

	// todo(): pass wgctrl client to wait for Handshake
	if errHandshake := wgt.waitForHandshake(client, remotePubKey, time.Second*10); errHandshake != nil {
		return fmt.Errorf("failed to wait for handshake: %w", errHandshake)
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

	return client.ConfigureDevice(wgt.config.Iface, wgtypes.Config{
		ReplacePeers: wgt.config.ReplacePeer, // Clears all peers
	})
}

// ensureInterfaceExists checks if the WireGuard interface exists and creates it if not
func (wgt *wgTunnel) ensureInterfaceExists(iface string) error {
	if !wgt.config.CreateIface {
		return nil
	}

	// Check if the interface already exists
	_, err := netlink.LinkByName(iface)
	if err == nil {
		return nil
	}

	// Only proceed if the interface is truly missing
	// todo(): improve error handling
	if !strings.Contains(err.Error(), "Link not found") {
		return fmt.Errorf("error checking interface %q: %w", iface, err)
	}

	link := &netlink.GenericLink{
		LinkAttrs: netlink.LinkAttrs{Name: iface},
		LinkType:  WireGuardLinkType,
	}

	if err = netlink.LinkAdd(link); err != nil {
		return fmt.Errorf("failed to create WireGuard interface %q: %w", iface, err)
	}

	if err = netlink.LinkSetUp(link); err != nil {
		return fmt.Errorf("failed to bring up interface %q: %w", iface, err)
	}

	return nil
}

func (wgt *wgTunnel) waitForHandshake(wgClient *wgctrl.Client, remotePubKey wgtypes.Key, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		device, errDevice := wgClient.Device(wgt.config.Iface)
		if errDevice != nil {
			return fmt.Errorf("failed to get device info: %w", errDevice)
		}

		for _, peer := range device.Peers {
			if peer.PublicKey == remotePubKey {
				if !peer.LastHandshakeTime.IsZero() {
					return nil
				}
			}
		}

		// todo(): make configurable
		time.Sleep(500 * time.Millisecond)
	}

	return fmt.Errorf("timeout waiting for handshake with peer %s", remotePubKey)
}
