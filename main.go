package main

import (
	"context"
	"encoding/base64"
	"log"
	"time"

	"github.com/yago-123/wg-punch/pkg/puncher"

	"github.com/yago-123/wg-punch/pkg/wg"

	wgpunch "github.com/yago-123/wg-punch/pkg"
	"github.com/yago-123/wg-punch/pkg/rendez/client"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

const (
	ContextTimeout = 30 * time.Second

	WireGuardListenPort        = 51820
	WireGuardInterfaceName     = "wg0"
	WireGuardKeepAliveInterval = 25 * time.Second
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

	// WireGuard interface using WireGuard
	tunnel := wg.NewTunnel(&wg.TunnelConfig{
		PrivateKey:        localPrivKey,
		Interface:         WireGuardInterfaceName,
		ListenPort:        WireGuardListenPort,
		ReplacePeer:       true,
		KeepAliveInterval: WireGuardKeepAliveInterval,
	})

	// Rendezvous server client (registers and discovers peer IPs)
	rendezvous := client.NewRendezvous("http://rendezvous.yago.ninja:7777")

	// Combine everything into the connector
	conn := wgpunch.NewConnector("local-peer", puncher, tunnel, rendezvous, 1*time.Second)

	// todo(): think about where to put the cancel of the tunnel itself
	defer tunnel.Close()

	ctx, cancel := context.WithTimeout(context.Background(), ContextTimeout)

	// Connect to peer using a shared peer ID (both sides use same ID)
	netConn, err := conn.Connect(ctx, "peer-id-123", localPrivKey, localPubKey)
	if err != nil {
		log.Fatalf("failed to connect to peer: %w", err)
	}

	defer cancel()
	defer netConn.Close()

	// Secure connection established! Use like any net.Conn
	_, err = netConn.Write([]byte("Hello from NAT punched WireGuard tunnel!\n"))
	if err != nil {
		log.Println("error writing:", err)
	}

	// todo(): wrap netConn in gRPC
}
