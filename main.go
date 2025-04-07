package main

import (
	"context"
	"encoding/base64"
	wgpunch "github.com/yago-123/wg-punch/pkg"
	"github.com/yago-123/wg-punch/pkg/rendezvous/client"
	"log"
	"time"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

const (
	Timeout             = 30 * time.Second
	WireGuardListenPort = 51820
)

func main() {
	// Generate WireGuard keypair
	privKey, err := wgtypes.GeneratePrivateKey()
	if err != nil {
		log.Fatalf("failed to generate private key: %v", err)
	}
	pubKey := privKey.PublicKey()

	// Your local WireGuard keys
	localPrivateKey := base64.StdEncoding.EncodeToString(privKey[:])
	peerPublicKey := base64.StdEncoding.EncodeToString(pubKey[:])

	// Example STUN server list (used to discover public IP/port)
	var stunServers = []string{
		"stun.l.google.com:19302",
		"stun1.l.google.com:19302",
	}

	// STUN-based hole puncher
	puncher := wgpunch.NewPuncher(stunServers)

	// WireGuard interface using wireguard-go in userspace
	tunnel := wgpunch.NewTunnel(&wgpunch.TunnelConfig{
		PrivateKey: localPrivateKey,
		ListenPort: WireGuardListenPort,
		Interface:  "wg0",
	})

	// Rendezvous server client (registers and discovers peer IPs)
	rendezvous := client.NewRendezvousClient("https://rendezvous.com")

	// Combine everything into the connector
	conn := wgpunch.NewConnector(puncher, tunnel, rendezvous)

	ctx, cancel := context.WithTimeout(context.Background(), Timeout)

	// Connect to peer using a shared peer ID (both sides use same ID)
	netConn, err := conn.Connect(ctx, "peer-id-123", peerPublicKey)
	if err != nil {
		log.Fatal("failed to connect to peer:", err)
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
