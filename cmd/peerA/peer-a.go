package main

import (
	"context"
	"github.com/sirupsen/logrus"
	"net"
	"time"

	"github.com/yago-123/wg-punch/pkg/connect"
	"github.com/yago-123/wg-punch/pkg/puncher"
	"github.com/yago-123/wg-punch/pkg/rendez/client"
	"github.com/yago-123/wg-punch/pkg/wg"
)

const (
	ContextTimeout   = 30 * time.Second
	RendezvousServer = "http://rendezvous.yago.ninja:7777"

	LocalPeerID  = "peer2"
	RemotePeerID = "peer1"

	WGListenPort    = 51822
	WGIfaceName     = "wg2"
	WGIfaceAddrCIDR = "10.1.1.2/32"

	WGPrivKey = "APSapiXBpAH1vTAh4EIvSYxhsE9O1YYVcZJngjvNbVs="
	WGPubKey  = "AKeIblnKKC1H75w+qWuL7LsU8mRW08dManorFcHTGw0="
	
	WGKeepAliveInterval = 5 * time.Second
)

var stunServers = []string{
	"stun.l.google.com:19302",
	"stun1.l.google.com:19302",
}

func main() {
	logger := logrus.New()

	// STUN-based hole puncher
	puncher := puncher.NewPuncher(stunServers)

	tunnelCfg := &wg.TunnelConfig{
		PrivateKey:        WGPrivKey,
		Iface:             WGIfaceName,
		IfaceIPv4CIDR:     WGIfaceAddrCIDR,
		ListenPort:        WGListenPort,
		ReplacePeer:       true,
		CreateIface:       true,
		KeepAliveInterval: WGKeepAliveInterval,
	}

	// WireGuard interface using WireGuard
	tunnel := wg.NewTunnel(tunnelCfg)

	// Rendezvous server client (registers and discovers peer IPs)
	rendezvous := client.NewRendezvous(RendezvousServer)

	// Combine everything into the connector
	conn := connect.NewConnector(LocalPeerID, puncher, tunnel, rendezvous, 1*time.Second)

	// todo(): think about where to put the cancel of the tunnel itself
	defer tunnel.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), ContextTimeout)

	localAddr := &net.UDPAddr{IP: net.IPv4zero, Port: tunnelCfg.ListenPort}

	// Connect to peer using a shared peer ID (both sides use same ID)
	netConn, err := conn.Connect(ctx, localAddr, []string{WGIfaceAddrCIDR}, RemotePeerID, WGPrivKey, WGPubKey)
	if err != nil {
		logger.Errorf("failed to connect to peer: %v", err)
		return
	}

	defer cancel()
	defer netConn.Close()

	// Secure connection established! Use like any net.Conn
	_, err = netConn.Write([]byte("Hello from NAT punched WireGuard tunnel!\n"))
	if err != nil {
		logger.Errorf("error writing to UDP connection: %v", err)
		return
	}

	// todo(): wrap netConn in gRPC
}
