package main

import (
	"context"
	"log"
	"time"
	"wg-punch/pkg/rendezvous/client"

	wgpunch "wg-punch/pkg"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Example STUN server list (used to discover public IP/port)
	stunServers := []string{
		"stun.l.google.com:19302",
		"stun1.l.google.com:19302",
	}

	// Your local WireGuard keys
	localPrivateKey := "P6kF7P6BxvE+EZU+12vMwId3V//8NwYOr6ErFZgYEVw=" // base64-encoded 32-byte key
	peerPublicKey := "t3A7Z5bX84b3qI++Aj3r4pA0+FeYbtdwuvfq+YOXGH4="   // peer's base64 public key

	// STUN-based hole puncher
	puncher := wgpunch.NewPuncher(stunServers)

	// WireGuard interface using wireguard-go in userspace
	tunnel := wgpunch.NewTunnel(wgpunch.TunnelConfig{
		PrivateKey: localPrivateKey,
		ListenPort: 51820,
		Interface:  "wg0",
	})

	// Rendezvous server client (registers and discovers peer IPs)
	rendezvous := client.NewRendezvousClient("https://rendezvous.example.com")

	// Combine everything into the connector
	conn := wgpunch.NewConnector(puncher, tunnel, rendezvous)

	// Connect to peer using a shared peer ID (both sides use same ID)
	netConn, err := conn.Connect(ctx, "peer-id-123", peerPublicKey)
	if err != nil {
		log.Fatal("failed to connect to peer:", err)
	}
	defer netConn.Close()

	// Secure connection established! Use like any net.Conn
	_, err = netConn.Write([]byte("Hello from NAT punched WireGuard tunnel!\n"))
	if err != nil {
		log.Println("error writing:", err)
	}

	// todo(): wrap netConn in gRPC
}
