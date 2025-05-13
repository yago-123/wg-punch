package userspacewg

import (
	"context"
	"encoding/hex"
	"fmt"
	"net"
	"strings"
	"time"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"

	"github.com/go-logr/logr"
	"github.com/vishvananda/netlink"

	"github.com/yago-123/wg-punch/pkg/peer"
	"github.com/yago-123/wg-punch/pkg/tunnel"
	tunnelUtil "github.com/yago-123/wg-punch/pkg/tunnel/util"
	
	"golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/tun"
)

const (
	DefaultNetMTU = 1420
	TUNDeviceType = "tun"
)

/*
[WireGuard-Go core]

	     ↓
	conn.Bind (your custom Bind)
	     ↓
	fake netstack UDPConn
	     ↓
	real outbound packets for NAT punching
*/

type userspaceWGTunnel struct {
	privKey   wgtypes.Key
	tunDevice *device.Device
	conn      *net.UDPConn

	config *tunnel.Config
	logger logr.Logger
}

func New(cfg *tunnel.Config, logger logr.Logger) (tunnel.Tunnel, error) {
	privKey, err := wgtypes.ParseKey(cfg.PrivKey)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}

	return &userspaceWGTunnel{
		privKey: privKey,
		config:  cfg,
		logger:  logger,
	}, nil
}

func (u *userspaceWGTunnel) Start(ctx context.Context, conn *net.UDPConn, remotePeer peer.Info, cancelPunch context.CancelFunc) error {
	tun, err := u.ensureTunInterfaceExists(u.config.Iface)
	if err != nil {
		return fmt.Errorf("failed to ensure TUN interface exists: %w", err)
	}

	// Create logger for the WireGuard device todo(): this needs rethinking
	logger := device.NewLogger(device.LogLevelVerbose, "wireguard: ")
	localAddr := &net.UDPAddr{IP: net.IPv4zero, Port: u.config.ListenPort}

	// Cancel the punching process so that it doesn't interfere with the new connection
	cancelPunch()

	// Spawn new virtual device that will handle packets in userspace
	tunDevice := device.NewDevice(tun, NewUDPBind(conn, localAddr, u.logger), logger)

	remotePubKey, err := wgtypes.ParseKey(remotePeer.PublicKey)
	if err != nil {
		return fmt.Errorf("invalid remote public key: %w", err)
	}

	wgConfig := wgtypes.Config{
		PrivateKey:   &u.privKey,
		ListenPort:   &u.config.ListenPort,
		ReplacePeers: u.config.ReplacePeer,
		Peers: []wgtypes.PeerConfig{
			{
				PublicKey:                   remotePubKey,
				Endpoint:                    remotePeer.Endpoint,
				AllowedIPs:                  remotePeer.AllowedIPs,
				PersistentKeepaliveInterval: &u.config.KeepAliveInterval,
			},
		},
	}

	uapiConfig, err := ConvertWgTypesToUAPI(wgConfig)
	if err != nil {
		return fmt.Errorf("failed to convert wgtypes.Config to UAPI: %w", err)
	}

	u.logger.Info("Creating uapi configuration", "config", uapiConfig)

	if err = tunnelUtil.AssignAddressToIface(u.config.Iface, u.config.IfaceIPv4CIDR); err != nil {
		cleanup(tunDevice, conn)
		return fmt.Errorf("failed to assign address to interface %s: %w", u.config.Iface, err)
	}

	if err = tunnelUtil.AddPeerRoutes(u.config.Iface, remotePeer.AllowedIPs); err != nil {
		cleanup(tunDevice, conn)
		return fmt.Errorf("failed to add peer routes to interface %s: %w", u.config.Iface, err)
	}

	//Pass the configuration to the device via IPC
	if errIpc := tunDevice.IpcSetOperation(strings.NewReader(uapiConfig)); errIpc != nil {
		cleanup(tunDevice, conn)
		return fmt.Errorf("failed to set IPC operation: %w", errIpc)
	}

	// Bring up the TUN device
	if errDevice := tunDevice.Up(); errDevice != nil {
		cleanup(tunDevice, conn)
		return fmt.Errorf("failed to bring up TUN device: %w", errDevice)
	}

	// todo(): handle the hex encoding of the public key better
	if errHandshake := u.waitForHandshake(ctx, tunDevice, hex.EncodeToString(remotePubKey[:])); errHandshake != nil {
		cleanup(tunDevice, conn)
		return fmt.Errorf("handshake did not complete: %w", errHandshake)
	}

	u.tunDevice = tunDevice
	u.conn = conn

	return nil
}

// TODO(): TEMPORARY FUNCTION, come up with a more elegant solution
func cleanup(dev *device.Device, conn *net.UDPConn) {
	if dev != nil {
		dev.Close()
	}
	if conn != nil {
		conn.Close()
	}
}

func (u *userspaceWGTunnel) ListenPort() int {
	return u.config.ListenPort
}

func (u *userspaceWGTunnel) PublicKey() string {
	return u.privKey.PublicKey().String()
}

func (u *userspaceWGTunnel) Stop(ctx context.Context) error {
	// todo(): handle errors and cleanup
	u.tunDevice.Close()
	// todo(): this might be nil (might be already closed via wireguard-go)
	u.conn.Close()

	// todo(): handle iface link deletion
	return nil
}

func (u *userspaceWGTunnel) ensureTunInterfaceExists(iface string) (tun.Device, error) {
	// if !u.config.CreateIface {
	// 	return nil, fmt.Errorf("TUN interface creation is disabled")
	// }

	// Try to delete the existing interface (optional safety)
	// todo(): this is like this just for testing, remove it later
	link, err := netlink.LinkByName(iface)
	if err == nil {
		u.logger.Info("Deleting pre-existing interface", "iface", iface)
		_ = netlink.LinkDel(link) // ignore error — best effort
	}

	// Only proceed if the interface is truly missing
	// todo(): improve error handling
	if !strings.Contains(err.Error(), "Link not found") {
		return nil, fmt.Errorf("error checking interface %s: %w", iface, err)
	}

	// Now create it cleanly
	tunDev, err := tun.CreateTUN(iface, DefaultNetMTU)
	if err != nil {
		return nil, fmt.Errorf("failed to create TUN interface %s: %w", iface, err)
	}

	link, err = netlink.LinkByName(iface)
	if err != nil {
		return nil, fmt.Errorf("failed to lookup interface %s: %w", iface, err)
	}

	// Set the interface up
	if errSetup := netlink.LinkSetUp(link); errSetup != nil {
		return nil, fmt.Errorf("failed to bring interface %s up: %w", iface, errSetup)
	}

	u.logger.Info("Created TUN interface", "iface", iface)
	return tunDev, nil
}

// waitForHandshake waits for the handshake to complete with the given public key
func (u *userspaceWGTunnel) waitForHandshake(ctx context.Context, dev *device.Device, peerPubKey string) error {
	// todo(): this polling interval should be configurable
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("context canceled or timed out while waiting for handshake: %w", ctx.Err())

		case <-ticker.C:
			var buf strings.Builder
			if err := dev.IpcGetOperation(&buf); err != nil {
				return fmt.Errorf("failed to get device status: %w", err)
			}

			if hasHandshakeOccurred(buf.String(), peerPubKey) {
				return nil
			}
		}
	}
}

// hasHandshakeOccurred checks if the handshake has occurred with the given public key
func hasHandshakeOccurred(status, pubKey string) bool {
	lines := strings.Split(status, "\n")
	found := false
	for _, line := range lines {
		if strings.HasPrefix(line, "public_key=") && strings.HasSuffix(line, pubKey) {
			found = true
		}
		if found && strings.HasPrefix(line, "last_handshake_time_sec=") {
			if !strings.HasSuffix(line, "=0") {
				return true
			}
		}
	}
	return false
}
