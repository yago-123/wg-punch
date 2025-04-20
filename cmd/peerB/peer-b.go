package main

import (
	"context"
	"log"
	"net"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/yago-123/wg-punch/pkg/connect"
	"github.com/yago-123/wg-punch/pkg/puncher"
	"github.com/yago-123/wg-punch/pkg/rendez/client"
	"github.com/yago-123/wg-punch/pkg/wg"
)

const (
	ContextTimeout   = 30 * time.Second
	RendezvousServer = "http://rendezvous.yago.ninja:7777"

	LocalPeerID  = "peer1"
	RemotePeerID = "peer2"

	WGListenPort    = 51821
	WGIfaceName     = "wg1"
	WGIfaceAddrCIDR = "10.1.1.1/32"

	WGPrivKey = "SEK/qGXalmKu3yPhkvZThcc8aQxordG5RkUz0/4jcFE="
	WGPubKey  = "CZq8h1yJSHkbLHtguwr6im+V5TNRrrCjYj6Y+XOR6wI="

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

	// todo(): do not reuse the timeout for the read deadline
	if errDeadline := netConn.SetReadDeadline(time.Now().Add(ContextTimeout)); errDeadline != nil {
		log.Printf("failed to set read deadline: %v", errDeadline)
		return
	}

	// Secure connection established! Use like any net.Conn
	var buf [1024]byte
	_, err = netConn.Read(buf[:])
	if err != nil {
		log.Println("error reading:", err)
		return
	}

	log.Printf("Read from remote peer: %s", string(buf[:]))

	// todo(): wrap netConn in gRPC
}
