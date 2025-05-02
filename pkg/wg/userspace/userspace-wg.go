package userspacewg

import (
	"context"
	"fmt"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	"net"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/vishvananda/netlink"
	"github.com/yago-123/wg-punch/pkg/peer"
	"github.com/yago-123/wg-punch/pkg/wg"
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
	config *wg.TunnelConfig

	privKey wgtypes.Key

	tunDevice *device.Device
	conn      *net.UDPConn

	logger logr.Logger
}

func New(cfg *wg.TunnelConfig, logger logr.Logger) (wg.Tunnel, error) {
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

func (u *userspaceWGTunnel) Start(ctx context.Context, conn *net.UDPConn, remotePeer peer.Info) error {
	tun, err := u.ensureTunInterfaceExists(u.config.Iface)
	if err != nil {
		return fmt.Errorf("failed to ensure TUN interface exists: %w", err)
	}

	// Create logger for the WireGuard device todo(): this needs rethinking

	logger := device.NewLogger(device.LogLevelVerbose, "wireguard: ")
	localAddr := &net.UDPAddr{IP: net.IPv4zero, Port: u.config.ListenPort}

	// Spawn new virtual device that will handle packets in userspace
	tunDevice := device.NewDevice(tun, NewUDPBind(conn, localAddr, u.logger), logger)

	//
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

	// Pass the configuration to the device via IPC
	if errIpc := tunDevice.IpcSetOperation(strings.NewReader(uapiConfig)); errIpc != nil {
		tunDevice.Close()
		conn.Close()
		return fmt.Errorf("failed to set IPC operation: %w", errIpc)
	}

	// Bring up the TUN device
	if errDevice := tunDevice.Up(); errDevice != nil {
		tunDevice.Close()
		conn.Close()
		return fmt.Errorf("failed to bring up TUN device: %w", errDevice)
	}

	if errHandshake := u.waitForHandshake(ctx, tunDevice, remotePeer.PublicKey); errHandshake != nil {
		tunDevice.Close()
		conn.Close()
		return fmt.Errorf("handshake did not complete: %w", errHandshake)
	}

	u.tunDevice = tunDevice
	u.conn = conn

	return nil
}

func (u *userspaceWGTunnel) ListenPort() int {
	return u.config.ListenPort
}

func (u *userspaceWGTunnel) PublicKey() string {
	return u.privKey.PublicKey().String()
}

func (u *userspaceWGTunnel) Stop() error {
	// todo(): handle errors and cleanup
	u.tunDevice.Close()
	// todo(): this might be nil (might be already closed via wireguard-go)
	u.conn.Close()
	return nil
}

func (u *userspaceWGTunnel) ensureTunInterfaceExists(iface string) (tun.Device, error) {
	// Try to delete the existing interface (optional safety)
	if link, err := netlink.LinkByName(iface); err == nil {
		u.logger.Info("Deleting pre-existing interface", "iface", iface)
		_ = netlink.LinkDel(link) // ignore error — best effort
	}

	// Now create it cleanly
	tunDev, err := tun.CreateTUN(iface, DefaultNetMTU)
	if err != nil {
		return nil, fmt.Errorf("failed to create TUN interface %q: %w", iface, err)
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
