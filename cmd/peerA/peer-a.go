package main

import (
	"context"
	"encoding/base64"
	"log"
	"net"
	"time"

	"github.com/yago-123/wg-punch/pkg/connect"
	"github.com/yago-123/wg-punch/pkg/puncher"
	"github.com/yago-123/wg-punch/pkg/rendez/client"
	"github.com/yago-123/wg-punch/pkg/wg"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

const (
	ContextTimeout = 30 * time.Second

	WireGuardListenPort        = 51821
	WireGuardInterfaceName     = "wg1"
	WireGuardKeepAliveInterval = 5 * time.Second
)

func main() {
	// Generate WireGuard keypair
	privKey, err := wgtypes.GeneratePrivateKey()
	if err != nil {
		log.Fatalf("failed to generate private key: %v", err)
	}
	pubKey := privKey.PublicKey()

	// Your local WireGuard keys
	localPrivKey := base64.StdEncoding.EncodeToString(privKey[:])
	localPubKey := base64.StdEncoding.EncodeToString(pubKey[:])

	// Example STUN server list (used to discover public IP/port)
	var stunServers = []string{
		"stun.l.google.com:19302",
		"stun1.l.google.com:19302",
	}

	// STUN-based hole puncher
	puncher := puncher.NewPuncher(stunServers)

	tunnelCfg := &wg.TunnelConfig{
		PrivateKey:        localPrivKey,
		Iface:             WireGuardInterfaceName,
		ListenPort:        WireGuardListenPort,
		ReplacePeer:       true,
		CreateIface:       true,
		KeepAliveInterval: WireGuardKeepAliveInterval,
	}

	// WireGuard interface using WireGuard
	tunnel := wg.NewTunnel(tunnelCfg)

	// Rendezvous server client (registers and discovers peer IPs)
	rendezvous := client.NewRendezvous("http://rendezvous.yago.ninja:7777")

	// Combine everything into the connector
	conn := connect.NewConnector("peer-101", puncher, tunnel, rendezvous, 1*time.Second)

	// todo(): think about where to put the cancel of the tunnel itself
	defer tunnel.Close()

	ctx, cancel := context.WithTimeout(context.Background(), ContextTimeout)

	// Connect to peer using a shared peer ID (both sides use same ID)
	localAddr := &net.UDPAddr{IP: net.IPv4zero, Port: tunnelCfg.ListenPort}
	netConn, err := conn.Connect(ctx, localAddr, []string{"10.0.0.43/32"}, "peer-111", localPrivKey, localPubKey)
	if err != nil {
		log.Fatalf("failed to connect to peer: %v", err)
	}

	defer cancel()
	defer netConn.Close()

	// Secure connection established! Use like any net.Conn
	_, err = netConn.Write([]byte("Hello from NAT punched WireGuard tunnel!\n"))
	if err != nil {
		log.Fatalf("error writing to UDP connection: %v", err)
	}

	// todo(): wrap netConn in gRPC
}
