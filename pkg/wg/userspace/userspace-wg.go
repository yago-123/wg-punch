package userspacewg

import (
	"context"
	"fmt"
	"net"
	"strings"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"

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
	logger  logr.Logger
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
	//
	tun, err := u.ensureTunInterfaceExists(u.config.Iface)
	if err != nil {
		return fmt.Errorf("failed to ensure TUN interface exists: %w", err)
	}

	// todo(): handle logger properly
	logger := device.NewLogger(device.LogLevelVerbose, "wireguard: ")

	// Spawn new virtual device that will handle packets in userspace
	tunDevice := device.NewDevice(tun, NewUDPBind(conn), logger)

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

	// Pass the configuration to the device via IPC
	if errIpc := tunDevice.IpcSetOperation(strings.NewReader(uapiConfig)); errIpc != nil {
		return fmt.Errorf("failed to set IPC operation: %w", errIpc)
	}

	// Bring up the TUN device
	if errDevice := tunDevice.Up(); errDevice != nil {
		tunDevice.Close()
		return fmt.Errorf("failed to bring up TUN device: %w", errDevice)
	}

	// todo: rethink
	go func() {
		<-ctx.Done()
		tunDevice.Close()
	}()

	return nil
}

func (u *userspaceWGTunnel) ListenPort() int {
	return u.config.ListenPort
}

func (u *userspaceWGTunnel) PublicKey() string {
	return ""
}

func (u *userspaceWGTunnel) Stop() error {
	return nil
}

func (u *userspaceWGTunnel) ensureTunInterfaceExists(iface string) (tun.Device, error) {
	// Check if the interface exists
	link, err := netlink.LinkByName(iface)
	if err == nil {
		// Interface exists, make sure it's a TUN device
		if link.Type() != TUNDeviceType {
			return nil, fmt.Errorf("interface %q exists but is not a TUN device (type %q)", iface, link.Type())
		}

		u.logger.Info("Interface already exists", "iface", iface)

		// If it already exists, we will "simulate" creating it given that there is no OpenTUN method
		// todo(): improve to make it more clear
		tunDev, errTun := tun.CreateTUN(iface, DefaultNetMTU)
		if errTun != nil {
			return nil, fmt.Errorf("failed to create TUN interface %q: %w", iface, errTun)
		}

		return tunDev, nil // Already there (you might want to open it if needed)
	}

	if _, ok := err.(netlink.LinkNotFoundError); !ok {
		return nil, fmt.Errorf("error checking interface %q: %w", iface, err)
	}

	// Interface truly missing, create it using TUN
	tunDev, err := tun.CreateTUN(iface, DefaultNetMTU)
	if err != nil {
		return nil, fmt.Errorf("failed to create TUN interface %q: %w", iface, err)
	}

	u.logger.Info("Created TUN interface", "iface", iface)
	return tunDev, nil
}
