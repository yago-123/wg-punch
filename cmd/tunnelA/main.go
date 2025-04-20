package main

import (
	"context"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/yago-123/wg-punch/pkg/peer"
	"github.com/yago-123/wg-punch/pkg/wg"
)

const (
	ContextTimeout = 30 * time.Second

	WGListenPort    = 51821
	WGIfaceName     = "wg1"
	WGIfaceAddrCIDR = "10.1.1.1/32"

	WGPrivKey = "0Ejy2JRTmtIOu10ThPYGWonhQMhQt8IqdaUtyP8xR3A="
	// WGPubKey  = "HhvuS5kX7kuqhlwnvbX7UjdFrjABQFShZ1q9qRSX9xI="

	WGKeepAliveInterval = 25 * time.Second

	WGRemoteListenPort    = 51822
	WGRemotePubEndpointIP = "127.0.0.1"
	WGRemotePubKey        = "h2iGtZoTXBl7hOF6vCt5bKemrBAEsjmqLHZuAUJi6is="
	WGRemotePeerCIDR      = "10.1.1.2/32"
)

func main() {
	logger := logrus.New()

	// Create a channel to listen for signals
	sigCh := make(chan os.Signal, 1)

	// Notify the channel on SIGINT or SIGTERM
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

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
		logger.Errorf("failed to parse CIDR: %v", err)
		return
	}

	// Create the remote peer
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
		logger.Errorf("failed to start tunnel: %v", errStart)
		return
	}

	defer tunnel.Stop()

	logger.Infof("Tunnel has been stablished! Press Ctrl+C to exit.")

	// Block until Ctrl+C signal is received
	<-sigCh
}
