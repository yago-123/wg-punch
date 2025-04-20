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
	log.Printf("Allowed IPs for peer %s: %s", peer.PublicKey, peer.AllowedIPs)

	if err = wgt.ensureInterfaceExists(wgt.config.Iface); err != nil {
		return fmt.Errorf("failed to ensure interface exists: %w", err)
	}

	if err = wgt.assignAddressToIface(wgt.config.Iface, wgt.config.IfaceIPv4CIDR); err != nil {
		return fmt.Errorf("failed to assign address to interface: %w", err)
	}

	// todo(): this might need to be replaced with wireguard-go + netstack
	// Close UDP connection so that WireGuard can take over
	if errConnUDP := conn.Close(); errConnUDP != nil {
		return fmt.Errorf("failed to close UDP connection: %w", errConnUDP)
	}

	time.Sleep(200 * time.Millisecond)

	if errDevice := client.ConfigureDevice(wgt.config.Iface, cfg); errDevice != nil {
		return fmt.Errorf("failed to configure device: %w", errDevice)
	}

	ctxInit, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
	defer cancel()

	go startHandshakeTriggerLoop(ctxInit, peer.Endpoint, 1*time.Second)

	if errHandshake := wgt.waitForHandshake(ctx, client, remotePubKey); errHandshake != nil {
		return fmt.Errorf("failed to wait for handshake: %w", errHandshake)
	}

	log.Printf("WireGuard tunnel established with peer %s", peer.PublicKey)

	wgt.listener = conn
	return nil
}

func (wgt *wgTunnel) Close() error {
	client, err := wgctrl.New()
	if err != nil {
		return fmt.Errorf("failed to open wgctrl client: %w", err)
	}
	defer client.Close()

	// Clear all peers first
	if errConf := client.ConfigureDevice(wgt.config.Iface, wgtypes.Config{
		ReplacePeers: true,
		Peers:        []wgtypes.PeerConfig{},
	}); errConf != nil {
		return fmt.Errorf("failed to clear WireGuard config: %w", errConf)
	}

	// Then delete the interface
	link, err := netlink.LinkByName(wgt.config.Iface)
	if err != nil {
		return fmt.Errorf("failed to get link %s: %w", wgt.config.Iface, err)
	}

	if errLink := netlink.LinkDel(link); errLink != nil {
		return fmt.Errorf("failed to delete link %s: %w", wgt.config.Iface, errLink)
	}

	return nil
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

	// todo(): move this into a separate function
	// Check if the address already exists on the interface
	existingAddrs, err := netlink.AddrList(link, netlink.FAMILY_ALL)
	if err != nil {
		return fmt.Errorf("failed to list addresses on %s: %w", iface, err)
	}

	for _, a := range existingAddrs {
		if a.IP.Equal(addr.IP) && a.Mask.String() == addr.Mask.String() {
			return nil // already exists, don't reassign
		}
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

// todo(): remove
func startHandshakeTriggerLoop(ctx context.Context, endpoint *net.UDPAddr, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			log.Printf("Sending handshake to %s", endpoint.String())

			conn, err := net.DialUDP("udp", nil, endpoint)
			if err != nil {
				log.Printf("Error dialing UDP: %v", err)
				continue
			}

			_, err = conn.Write([]byte("hello wg"))
			conn.Close()

			if err != nil {
				log.Printf("Error sending handshake to %s: %v", endpoint.String(), err)
			}
		}
	}
}
