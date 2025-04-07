package main

import (
	"context"
	"encoding/base64"
	"log"
	"time"

	"github.com/yago-123/wg-punch/pkg/util"

	rendClient "github.com/yago-123/wg-punch/pkg/rendezvous/client"
	"github.com/yago-123/wg-punch/pkg/rendezvous/types"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

func main() {
	// Generate WireGuard keypair
	privKey, err := wgtypes.GeneratePrivateKey()
	if err != nil {
		log.Fatalf("failed to generate private key: %v", err)
	}
	pubKey := privKey.PublicKey()

	log.Println("Generated keys:")
	log.Printf("- Private: %s\n", base64.StdEncoding.EncodeToString(privKey[:]))
	log.Printf("- Public : %s\n", base64.StdEncoding.EncodeToString(pubKey[:]))

	// Create rendezvous client
	client := rendClient.NewRendezvous("http://rendezvous.yago.ninja:7777")

	// Register this peer
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	stunServers := []string{
		"stun.l.google.com:19302",
		"stun1.l.google.com:19302",
	}

	// Get public endpoint using STUN servers
	endpoint, err := util.GetPublicEndpoint(stunServers)
	if err != nil {
		log.Fatalf("failed to get public endpoint: %v", err)
	}

	err = client.Register(ctx, types.RegisterRequest{
		PeerID:     "peer-a",
		PublicKey:  base64.StdEncoding.EncodeToString(pubKey[:]),
		AllowedIPs: []string{"10.0.0.2/32"},
		Endpoint:   endpoint.String(),
	})
	if err != nil {
		log.Fatalf("register failed: %v", err)
	}
	log.Println("Registered successfully")

	// Discover remote peer
	ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, udpAddr, err := client.Discover(ctx, "peer-a")
	if err != nil {
		log.Fatalf("discover failed: %v", err)
	}

	log.Println("Discovered peer:")
	log.Printf("- PeerID    : %s", resp.PeerID)
	log.Printf("- PublicKey : %s", resp.PublicKey)
	log.Printf("- Endpoint  : %s", udpAddr)
	log.Printf("- AllowedIPs: %v", resp.AllowedIPs)

	// This is the point where you can initiate hole punching and WireGuard handshake
}
