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
	ContextTimeout = 1 * time.Hour

	WireGuardListenPort    = 51821
	WireGuardIfaceName     = "wg1"
	WireGuardIfaceAddrCIDR = "10.1.1.1/32"

	WireGuardKeepAliveInterval = 5 * time.Second
)

func main() {
	// Generate WireGuard keypair
	privKey, err := wgtypes.GeneratePrivateKey()
	if err != nil {
		log.Printf("failed to generate private key: %v", err)
		return
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
		Iface:             WireGuardIfaceName,
		IfaceIPv4CIDR:     WireGuardIfaceAddrCIDR,
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
	conn := connect.NewConnector("p11", puncher, tunnel, rendezvous, 1*time.Second)

	// todo(): think about where to put the cancel of the tunnel itself
	defer tunnel.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), ContextTimeout)

	localAddr := &net.UDPAddr{IP: net.IPv4zero, Port: tunnelCfg.ListenPort}
	// Connect to peer using a shared peer ID (both sides use same ID)
	netConn, err := conn.Connect(ctx, localAddr, []string{WireGuardIfaceAddrCIDR}, "p22", localPrivKey, localPubKey)
	if err != nil {
		log.Printf("failed to connect to peer: %v", err)
		return
	}

	defer cancel()
	defer netConn.Close()

	// Secure connection established! Use like any net.Conn
	_, err = netConn.Write([]byte("Hello from NAT punched WireGuard tunnel!\n"))
	if err != nil {
		log.Printf("error writing to UDP connection: %v", err)
		return
	}

	// todo(): wrap netConn in gRPC
}
