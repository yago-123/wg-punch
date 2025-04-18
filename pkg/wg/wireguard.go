package wg

import (
	"context"
	"fmt"
	"log"
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
	Start(ctx context.Context, conn *net.UDPConn, localPrivKey string, peer peer.Info) error
	Close() error
}

type TunnelConfig struct {
	PrivateKey        string
	Iface             string
	IfaceIPv4CIDR     string
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

func (wgt *wgTunnel) Start(ctx context.Context, conn *net.UDPConn, localPrivKey string, peer peer.Info) error {
	client, err := wgctrl.New()
	if err != nil {
		return fmt.Errorf("failed to open wgctrl client: %w", err)
	}
	defer client.Close()

	// todo(): move to native wgctrl key type
	privKey, err := wgtypes.ParseKey(localPrivKey)
	if err != nil {
		return fmt.Errorf("invalid private key: %w", err)
	}

	remotePubKey, err := wgtypes.ParseKey(peer.PublicKey)
	if err != nil {
		return fmt.Errorf("invalid remote public key: %w", err)
	}

	// todo() adjust logging to avoid unnecessary info
	log.Printf("Public key used by local peer: %s", peer.PublicKey)
	log.Printf("Endpoint being used by WG: %s", peer.Endpoint.String())

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

	log.Printf("Starting WireGuard tunnel on interface %q to endpoint %s", wgt.config.Iface, peer.Endpoint)

	if err = wgt.ensureInterfaceExists(wgt.config.Iface); err != nil {
		return fmt.Errorf("failed to ensure interface exists: %w", err)
	}

	if err = wgt.assignAddressToIface(wgt.config.Iface, wgt.config.IfaceIPv4CIDR); err != nil {
		return fmt.Errorf("failed to assign address to interface: %w", err)
	}

	if errDevice := client.ConfigureDevice(wgt.config.Iface, cfg); errDevice != nil {
		return fmt.Errorf("failed to configure device: %w", errDevice)
	}

	if errHandshake := wgt.waitForHandshake(ctx, client, remotePubKey); errHandshake != nil {
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

	// Create the WireGuard interface
	if err = netlink.LinkAdd(link); err != nil {
		return fmt.Errorf("failed to create WireGuard interface %q: %w", iface, err)
	}

	// Bring the interface up
	if err = netlink.LinkSetUp(link); err != nil {
		return fmt.Errorf("failed to bring up interface %q: %w", iface, err)
	}

	return nil
}

// assignAddressToIface assigns the internal IP address to the WireGuard interface in CIDR notation in order to allow
// communications between peers
func (wgt *wgTunnel) assignAddressToIface(iface, addrCIDR string) error {
	// Lookup interface link by name
	link, err := netlink.LinkByName(iface)
	if err != nil {
		return fmt.Errorf("failed to get link %s: %w", iface, err)
	}

	// Parse address CIDR to assign to the interface
	addr, err := netlink.ParseAddr(addrCIDR)
	if err != nil {
		return fmt.Errorf("failed to parse address %s: %w", addrCIDR, err)
	}

	// Assign address to the interface
	if errAddr := netlink.AddrAdd(link, addr); errAddr != nil {
		return fmt.Errorf("failed to assign address: %w", errAddr)
	}

	return nil
}

// waitForHandshake waits for the handshake with the remote peer to be established
func (wgt *wgTunnel) waitForHandshake(ctx context.Context, wgClient *wgctrl.Client, remotePubKey wgtypes.Key) error {
	// todo(): make ticker configurable
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("context cancelled or deadline exceeded while waiting for handshake with peer %s: %w", remotePubKey, ctx.Err())

		case <-ticker.C:
			// Check if the device exists
			device, errDevice := wgClient.Device(wgt.config.Iface)
			if errDevice != nil {
				return fmt.Errorf("failed to get device info: %w", errDevice)
			}

			// Check if the peer is present in the device
			if hasHandshake(device, remotePubKey) {
				return nil
			}
		}
	}
}

// hasHandshake checks if the peer has a handshake with the given public key
func hasHandshake(device *wgtypes.Device, remotePubKey wgtypes.Key) bool {
	for _, peer := range device.Peers {
		if peer.PublicKey == remotePubKey && !peer.LastHandshakeTime.IsZero() {
			return true
		}
	}
	return false
}
