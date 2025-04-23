package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/yago-123/wg-punch/pkg/puncher"

	"github.com/go-logr/logr"

	"github.com/yago-123/wg-punch/pkg/util"

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

	LocalPeerID  = "ww2"
	RemotePeerID = "ww1"

	WGLocalListenPort    = 51822
	WGLocalIfaceName     = "wg2"
	WGLocalIfaceAddr     = "10.1.1.2"
	WGLocalIfaceAddrCIDR = "10.1.1.2/32"

	// todo(): this should go away
	RemotePeerIP = "10.1.1.1"

	WGLocalPrivKey = "SEK/qGXalmKu3yPhkvZThcc8aQxordG5RkUz0/4jcFE="
	WGLocalPubKey  = "CZq8h1yJSHkbLHtguwr6im+V5TNRrrCjYj6Y+XOR6wI="

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
	go startTCPServer(logger)

	// Start TCP client after a delay to ensure server is ready
	time.Sleep(5 * time.Second)
	// todo(): adjust the IP to the one assigned by the rendezvous server
	go startTCPClient(RemotePeerIP, logger)

	// Block until Ctrl+C signal is received
	<-sigCh
}

func startTCPServer(logger logr.Logger) {
	serverAddr := fmt.Sprintf("%s:%d", WGLocalIfaceAddr, TCPServerPort)
	ln, err := net.Listen(util.TCPProtocol, serverAddr)
	if err != nil {
		logger.Error(err, "TCP server listen error", "address", serverAddr)
		return
	}
	defer ln.Close()

	logger.Info("TCP server ready", "address", serverAddr)

	for {
		conn, errServer := ln.Accept()
		if errServer != nil {
			logger.Error(errServer, "error accepting connection", "address", serverAddr)
			continue
		}

		go handleTCPConnection(conn, logger)
	}
}

func handleTCPConnection(c net.Conn, logger logr.Logger) {
	defer c.Close()
	buf := make([]byte, TCPMaxBuffer)

	for {
		n, err := c.Read(buf)
		if err != nil {
			if err == io.EOF {
				logger.Info("TCP client disconnected", "address", c.RemoteAddr().String())
			} else {
				logger.Info("TCP client read error", "address", c.RemoteAddr().String())
			}
			return
		}

		logger.Info("TCP received message", "remoteAddr", c.RemoteAddr().String(), "message", string(buf[:n]))
	}
}

func startTCPClient(remoteAddr string, logger logr.Logger) {
	remoteServerAddr := fmt.Sprintf("%s:%d", remoteAddr, TCPClientPort)
	conn, err := net.Dial(util.TCPProtocol, remoteServerAddr)
	if err != nil {
		logger.Error(err, "TCP client listen error", "address", remoteServerAddr)
		return
	}
	defer conn.Close()

	for {
		_, err = conn.Write([]byte("hello via TCP over WireGuard"))
		if err != nil {
			logger.Error(err, "TCP client write error", "address", remoteServerAddr)
			return
		}

		time.Sleep(5 * time.Second)
	}
}
