package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/yago-123/wg-punch/cmd/common"

	"github.com/yago-123/wg-punch/pkg/puncher"

	"github.com/go-logr/logr"
	kernelwg "github.com/yago-123/wg-punch/pkg/wg/kernel"

	"github.com/yago-123/wg-punch/pkg/connect"
	"github.com/yago-123/wg-punch/pkg/wg"
)

const (
	TCPServerPort = 8080
	TCPClientPort = 8080
	TCPMaxBuffer  = 1024

	TunnelHandshakeTimeout = 30 * time.Second
	RendezvousServer       = "http://rendezvous.yago.ninja:7777"

	LocalPeerID  = "ww1"
	RemotePeerID = "ww2"

	WGLocalListenPort    = 51821
	WGLocalIfaceName     = "wg1"
	WGLocalIfaceAddr     = "10.1.1.1"
	WGLocalIfaceAddrCIDR = "10.1.1.1/32"

	// todo(): this should go away
	RemotePeerIP = "10.1.1.2"

	WGLocalPrivKey = "APSapiXBpAH1vTAh4EIvSYxhsE9O1YYVcZJngjvNbVs="
	WGLocalPubKey  = "AKeIblnKKC1H75w+qWuL7LsU8mRW08dManorFcHTGw0="

	WGKeepAliveInterval = 5 * time.Second
)

var stunServers = []string{
	"stun.l.google.com:19302",
	"stun1.l.google.com:19302",
}

func main() {
	slogLogger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	logger := logr.FromSlogHandler(slogLogger.Handler())

	// Create a channel to listen for signals
	sigCh := make(chan os.Signal, 1)

	// Notify the channel on SIGINT or SIGTERM
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	puncherOptions := []puncher.Option{
		puncher.WithPuncherInterval(300 * time.Millisecond),
		puncher.WithSTUNServers(stunServers),
		puncher.WithLogger(logger),
	}
	// Create a puncher with the STUN servers
	p := puncher.NewPuncher(puncherOptions...)

	connectorOptions := []connect.Option{
		connect.WithRendezServer(RendezvousServer),
		connect.WithWaitInterval(1 * time.Second),
		connect.WithLogger(logger),
	}
	// Create a connector with the puncher
	conn := connect.NewConnector(LocalPeerID, p, connectorOptions...)

	ctxHandshake, cancel := context.WithTimeout(context.Background(), TunnelHandshakeTimeout)
	defer cancel()

	tunnelCfg := &wg.TunnelConfig{
		PrivKey:           WGLocalPrivKey,
		Iface:             WGLocalIfaceName,
		IfaceIPv4CIDR:     WGLocalIfaceAddrCIDR,
		ListenPort:        WGLocalListenPort,
		ReplacePeer:       true,
		CreateIface:       true,
		KeepAliveInterval: WGKeepAliveInterval,
	}

	// Initialize WireGuard interface using WireGuard
	tunnel, err := kernelwg.NewTunnel(tunnelCfg)
	if err != nil {
		logger.Error(err, "failed to create tunnel", "localPeer", LocalPeerID)
		return
	}

	// Connect to peer using a shared peer ID (both sides use same ID)
	netConn, err := conn.Connect(ctxHandshake, tunnel, []string{WGLocalIfaceAddrCIDR}, RemotePeerID)
	if err != nil {
		logger.Error(err, "failed to connect to peer", "localPeer", LocalPeerID, "remotePeerID", RemotePeerID)
		return
	}

	// todo(): think about where to put the cancel of the tunnel itself
	defer tunnel.Stop()
	defer netConn.Close()

	logger.Info("Tunnel has been stablished! Press Ctrl+C to exit.")

	// Start TCP server
	tcpServer, err := common.NewTCPServer(WGLocalIfaceAddr, TCPServerPort, logger)
	if err != nil {
		logger.Error(err, "failed to create TCP server", "address", WGLocalIfaceAddr)
		return
	}

	tcpServer.Start()
	defer tcpServer.Close()

	// Start TCP client after a delay to ensure server is ready
	time.Sleep(5 * time.Second)

	// Start TCP client that will query remote peer over WireGuard
	tcpClient, err := common.NewTCPClient(RemotePeerIP, TCPClientPort, logger)
	if err != nil {
		logger.Error(err, "failed to create TCP client", "address", RemotePeerIP)
		return
	}
	defer tcpClient.Close()

	go func() {
		for {
			tcpClient.Send("hello via TCP over WireGuard")
			time.Sleep(3 * time.Second)
		}
	}()

	// Block until Ctrl+C signal is received
	<-sigCh
}
