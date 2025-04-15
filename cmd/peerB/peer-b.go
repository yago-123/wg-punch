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

	WireGuardListenPort        = 51822
	WireGuardInterfaceName     = "wg2"
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
	conn := connect.NewConnector("peer-11", puncher, tunnel, rendezvous, 1*time.Second)

	// todo(): think about where to put the cancel of the tunnel itself
	defer tunnel.Close()

	ctx, cancel := context.WithTimeout(context.Background(), ContextTimeout)

	localAddr := &net.UDPAddr{IP: net.IPv4zero, Port: tunnelCfg.ListenPort}
	// Connect to peer using a shared peer ID (both sides use same ID)
	netConn, err := conn.Connect(ctx, localAddr, []string{"10.0.0.42/32"}, "peer-10", localPrivKey, localPubKey)
	if err != nil {
		log.Fatalf("failed to connect to peer: %v", err)
	}

	defer cancel()
	defer netConn.Close()

	if errDeadline := netConn.SetReadDeadline(time.Now().Add(ContextTimeout)); errDeadline != nil {
		log.Fatalf("failed to set read deadline: %v", errDeadline)
	}

	// Secure connection established! Use like any net.Conn
	var buf [1024]byte
	_, err = netConn.Read(buf[:])
	if err != nil {
		log.Println("error reading:", err)
	}

	log.Printf("Read from remote peer: %s", string(buf[:]))

	// todo(): wrap netConn in gRPC
}
