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

	WireGuardListenPort    = 51822
	WireGuardInterfaceName = "wg2"
	WireGuardIfaceAddrCIDR = "10.1.1.2/32"

	WireGuardKeepAliveInterval = 5 * time.Second
)

func main() {
	allowedIPs := []string{"10.1.1.1/32"}

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
	conn := connect.NewConnector("cd2", puncher, tunnel, rendezvous, 1*time.Second)

	// todo(): think about where to put the cancel of the tunnel itself
	defer tunnel.Close()

	ctx, cancel := context.WithTimeout(context.Background(), ContextTimeout)

	localAddr := &net.UDPAddr{IP: net.IPv4zero, Port: tunnelCfg.ListenPort}
	// Connect to peer using a shared peer ID (both sides use same ID)
	netConn, err := conn.Connect(ctx, localAddr, allowedIPs, "cd1", localPrivKey, localPubKey)
	if err != nil {
		log.Printf("failed to connect to peer: %v", err)
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
