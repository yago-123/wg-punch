package kernelwg

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"time"

	"github.com/yago-123/wg-punch/pkg/util"

	"github.com/vishvananda/netlink"
	"github.com/yago-123/wg-punch/pkg/peer"
	"github.com/yago-123/wg-punch/pkg/wg"
	"golang.zx2c4.com/wireguard/wgctrl"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

const (
	WireGuardLinkType = "wireguard"
)

type kernelWGTunnel struct {
	config   *wg.TunnelConfig
	listener *net.UDPConn
}

func NewTunnel(cfg *wg.TunnelConfig) wg.Tunnel {
	return &kernelWGTunnel{
		config: cfg,
	}
}

func (kwgt *kernelWGTunnel) Start(ctx context.Context, conn *net.UDPConn, peer peer.Info) error {
	client, err := wgctrl.New()
	if err != nil {
		return fmt.Errorf("failed to open wgctrl rendclient: %w", err)
	}
	defer client.Close()

	// todo(): move to native wgctrl key type
	privKey, err := wgtypes.ParseKey(kwgt.config.PrivateKey)
	if err != nil {
		return fmt.Errorf("invalid private key: %w", err)
	}

	remotePubKey, err := wgtypes.ParseKey(peer.PublicKey)
	if err != nil {
		return fmt.Errorf("invalid remote public key: %w", err)
	}

	cfg := wgtypes.Config{
		PrivateKey:   &privKey,
		ListenPort:   &kwgt.config.ListenPort,
		ReplacePeers: kwgt.config.ReplacePeer,
		Peers: []wgtypes.PeerConfig{
			{
				PublicKey:                   remotePubKey,
				Endpoint:                    peer.Endpoint,
				AllowedIPs:                  peer.AllowedIPs,
				PersistentKeepaliveInterval: &kwgt.config.KeepAliveInterval,
			},
		},
	}

	log.Printf("configuring WireGuard device %s with config: %+v", kwgt.config.Iface, cfg)

	if err = kwgt.ensureInterfaceExists(kwgt.config.Iface); err != nil {
		return fmt.Errorf("failed to ensure interface exists: %w", err)
	}

	if err = kwgt.assignAddressToIface(kwgt.config.Iface, kwgt.config.IfaceIPv4CIDR); err != nil {
		return fmt.Errorf("failed to assign address to interface: %w", err)
	}

	if err = kwgt.addPeerRoutes(kwgt.config.Iface, peer.AllowedIPs); err != nil {
		return fmt.Errorf("failed to add peer routes: %w", err)
	}

	// todo(): this check should go away
	if conn != nil {
		// Stop UDP connection so that WireGuard can take over
		if errConnUDP := conn.Close(); errConnUDP != nil {
			return fmt.Errorf("failed to close UDP connection: %w", errConnUDP)
		}
	}

	time.Sleep(200 * time.Millisecond)

	if errDevice := client.ConfigureDevice(kwgt.config.Iface, cfg); errDevice != nil {
		return fmt.Errorf("failed to configure device: %w", errDevice)
	}

	ctxInit, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
	defer cancel()

	go startHandshakeTriggerLoop(ctxInit, peer.Endpoint, 1*time.Second)

	if errHandshake := kwgt.waitForHandshake(ctx, client, remotePubKey); errHandshake != nil {
		return fmt.Errorf("failed to wait for handshake: %w", errHandshake)
	}

	kwgt.listener = conn
	return nil
}

func (kwgt *kernelWGTunnel) Stop() error {
	client, err := wgctrl.New()
	if err != nil {
		return fmt.Errorf("failed to open wgctrl rendclient: %w", err)
	}
	defer client.Close()

	// Clear all peers first
	if errConf := client.ConfigureDevice(kwgt.config.Iface, wgtypes.Config{
		ReplacePeers: true,
		Peers:        []wgtypes.PeerConfig{},
	}); errConf != nil {
		return fmt.Errorf("failed to clear WireGuard config: %w", errConf)
	}

	// Then delete the interface
	link, err := netlink.LinkByName(kwgt.config.Iface)
	if err != nil {
		return fmt.Errorf("failed to get link %s: %w", kwgt.config.Iface, err)
	}

	if errLink := netlink.LinkDel(link); errLink != nil {
		return fmt.Errorf("failed to delete link %s: %w", kwgt.config.Iface, errLink)
	}

	return nil
}

// ensureInterfaceExists checks if the WireGuard interface exists and creates it if not
func (kwgt *kernelWGTunnel) ensureInterfaceExists(iface string) error {
	if !kwgt.config.CreateIface {
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
func (kwgt *kernelWGTunnel) assignAddressToIface(iface, addrCIDR string) error {
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

// addPeerRoutes adds the allowed IPs of the peer to the WireGuard interface so that the kernel can route packets
func (kwgt *kernelWGTunnel) addPeerRoutes(iface string, allowedIPs []net.IPNet) error {
	link, err := netlink.LinkByName(iface)
	if err != nil {
		return fmt.Errorf("failed to get link %q: %w", iface, err)
	}

	for _, ipNet := range allowedIPs {
		route := &netlink.Route{
			LinkIndex: link.Attrs().Index,
			Dst:       &ipNet,
		}

		// Try to add the route, but don't fail if it already exists
		if errRoute := netlink.RouteAdd(route); errRoute != nil && !os.IsExist(errRoute) {
			return fmt.Errorf("failed to add route %s: %w", ipNet.String(), errRoute)
		}
	}

	return nil
}

// waitForHandshake waits for the handshake with the remote peer to be established
func (kwgt *kernelWGTunnel) waitForHandshake(ctx context.Context, wgClient *wgctrl.Client, remotePubKey wgtypes.Key) error {
	// todo(): make ticker configurable
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("context cancelled or deadline exceeded while waiting for handshake with peer %s: %w", remotePubKey, ctx.Err())

		case <-ticker.C:
			// Check if the device exists
			device, errDevice := wgClient.Device(kwgt.config.Iface)
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
			conn, err := net.DialUDP(util.UDPProtocol, nil, endpoint)
			if err != nil {
				// log.Printf("Error dialing UDP: %v", err)
				continue
			}

			_, err = conn.Write([]byte("hello wg"))
			conn.Close()

			if err != nil {
				// log.Printf("Error sending handshake to %s: %v", endpoint.String(), err)
			}
		}
	}
}
