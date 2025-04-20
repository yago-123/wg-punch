package main

import (
	"context"
	"log"
	"net"
	"time"

	"github.com/yago-123/wg-punch/pkg/peer"
	"github.com/yago-123/wg-punch/pkg/wg"
)

const (
	ContextTimeout = 1 * time.Hour

	WGListenPort    = 51820
	WGIfaceName     = "wg0"
	WGIfaceAddrCIDR = "10.1.1.2/32"

	WGPubKey  = "h2iGtZoTXBl7hOF6vCt5bKemrBAEsjmqLHZuAUJi6is="
	WGPrivKey = "4EnHGpFp2eW+aRMK1VVWqUtorspluG5FP0/P+YnLCns="

	WGKeepAliveInterval = 5 * time.Second

	WGRemoteListenPort    = 51820
	WGRemotePubEndpointIP = "192.168.18.201"
	WGRemotePubKey        = "HhvuS5kX7kuqhlwnvbX7UjdFrjABQFShZ1q9qRSX9xI="
	WGRemotePeerCIDR      = "10.1.1.1/32"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), ContextTimeout)
	defer cancel()

	// Configure the tunnel
	tunnelCfg := &wg.TunnelConfig{
		PrivateKey:        WGPrivKey,
		Iface:             WGIfaceName,
		IfaceIPv4CIDR:     WGIfaceAddrCIDR,
		ListenPort:        WGListenPort,
		ReplacePeer:       true,
		CreateIface:       true,
		KeepAliveInterval: WGKeepAliveInterval,
	}

	remoteIP, remoteIPCIDR, err := net.ParseCIDR(WGRemotePeerCIDR)
	if err != nil {
		log.Printf("failed to parse CIDR: %v", err)
		return
	}

	// Create a peer
	remotePeer := peer.Info{
		PublicKey: WGRemotePubKey,
		Endpoint: &net.UDPAddr{
			IP:   net.ParseIP(WGRemotePubEndpointIP),
			Port: WGRemoteListenPort,
		},
		AllowedIPs: []net.IPNet{
			{
				IP:   remoteIP,
				Mask: remoteIPCIDR.Mask,
			},
		},
	}

	tunnel := wg.NewTunnel(tunnelCfg)

	if errStart := tunnel.Start(ctx, nil, tunnelCfg.PrivateKey, remotePeer); errStart != nil {
		log.Printf("failed to start tunnel: %v", errStart)
		return
	}

	log.Println("Tunnel is up! Press Ctrl+C to exit.")
	<-ctx.Done()

	if errTun := tunnel.Close(); errTun != nil {
		log.Printf("Error closing tunnel: %v", errTun)
		return
	}
}
