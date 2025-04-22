package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/yago-123/wg-punch/pkg/util"

	kernelwg "github.com/yago-123/wg-punch/pkg/wg/kernel"

	"github.com/sirupsen/logrus"

	"github.com/yago-123/wg-punch/pkg/connect"
	"github.com/yago-123/wg-punch/pkg/puncher"
	"github.com/yago-123/wg-punch/pkg/rendez/client"
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

	WGLocalPrivKey = "APSapiXBpAH1vTAh4EIvSYxhsE9O1YYVcZJngjvNbVs="
	WGLocalPubKey  = "AKeIblnKKC1H75w+qWuL7LsU8mRW08dManorFcHTGw0="

	WGKeepAliveInterval = 5 * time.Second
)

var stunServers = []string{
	"stun.l.google.com:19302",
	"stun1.l.google.com:19302",
}

func main() {
	logger := logrus.New()

	// Create a channel to listen for signals
	sigCh := make(chan os.Signal, 1)

	// Notify the channel on SIGINT or SIGTERM
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// STUN-based hole puncher
	puncher := puncher.NewPuncher(stunServers)

	tunnelCfg := &wg.TunnelConfig{
		PrivateKey:        WGLocalPrivKey,
		Iface:             WGLocalIfaceName,
		IfaceIPv4CIDR:     WGLocalIfaceAddrCIDR,
		ListenPort:        WGLocalListenPort,
		ReplacePeer:       true,
		CreateIface:       true,
		KeepAliveInterval: WGKeepAliveInterval,
	}

	// WireGuard interface using WireGuard
	tunnel := kernelwg.NewTunnel(tunnelCfg)

	// Rendezvous server (registers and discovers peer IPs)
	rendezvous := client.NewRendezvous(RendezvousServer)

	// Combine everything into the connector
	conn := connect.NewConnector(LocalPeerID, puncher, tunnel, rendezvous, 1*time.Second)

	// todo(): think about where to put the cancel of the tunnel itself
	defer tunnel.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), TunnelHandshakeTimeout)

	localAddr := &net.UDPAddr{IP: net.IPv4zero, Port: tunnelCfg.ListenPort}

	// Connect to peer using a shared peer ID (both sides use same ID)
	netConn, err := conn.Connect(ctx, localAddr, []string{WGLocalIfaceAddrCIDR}, RemotePeerID, WGLocalPubKey)
	if err != nil {
		logger.Errorf("failed to connect to peer: %v", err)
		return
	}

	defer cancel()
	defer netConn.Close()

	logger.Printf("Tunnel has been stablished! Press Ctrl+C to exit.")

	// Start TCP server
	go startTCPServer(logger)

	// Start TCP client after a delay to ensure server is ready
	time.Sleep(5 * time.Second)
	// todo(): adjust the IP to the one assigned by the rendezvous server
	go startTCPClient(logger, "10.1.1.2")

	// Block until Ctrl+C signal is received
	<-sigCh
}

func startTCPServer(logger *logrus.Logger) {
	serverAddr := fmt.Sprintf("%s:%d", WGLocalIfaceAddr, TCPServerPort)
	ln, err := net.Listen(util.TCPProtocol, serverAddr)
	if err != nil {
		logger.Errorf("TCP server listen error: %v", err)
		return
	}
	defer ln.Close()

	logger.Infof("TCP server ready on %s", serverAddr)

	for {
		conn, errServer := ln.Accept()
		if errServer != nil {
			logger.Errorf("accept error: %v", errServer)
			continue
		}

		go handleTCPConnection(conn, logger)
	}
}

func handleTCPConnection(c net.Conn, logger *logrus.Logger) {
	defer c.Close()
	buf := make([]byte, TCPMaxBuffer)

	for {
		n, err := c.Read(buf)
		if err != nil {
			if err == io.EOF {
				logger.Infof("connection closed by %s", c.RemoteAddr())
			} else {
				logger.Errorf("read error from %s: %v", c.RemoteAddr(), err)
			}
			return
		}
		logger.Infof("received msg from %s: %s", c.RemoteAddr().String(), string(buf[:n]))
	}
}

func startTCPClient(logger *logrus.Logger, remoteAddr string) {
	remoteServerAddr := fmt.Sprintf("%s:%d", remoteAddr, TCPClientPort)
	conn, err := net.Dial(util.TCPProtocol, remoteServerAddr)
	if err != nil {
		logger.Errorf("TCP dial error: %v", err)
		return
	}
	defer conn.Close()

	for {
		_, err = conn.Write([]byte("hello via TCP over WireGuard"))
		if err != nil {
			logger.Errorf("write error: %v", err)
			return
		}

		time.Sleep(5 * time.Second)
	}
}
