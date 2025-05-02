package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/yago-123/wg-punch/pkg/util"

	kernelwg "github.com/yago-123/wg-punch-kernel/kernel"

	"github.com/sirupsen/logrus"

	"github.com/yago-123/wg-punch/pkg/peer"
	"github.com/yago-123/wg-punch/pkg/wg"
)

const (
	TunnelHandshakeTimeout = 30 * time.Second

	TCPMaxBuffer  = 1024
	TCPServerPort = 8080
	TCPClientPort = 8080

	WGLocalListenPort    = 51822
	WGLocalIfaceName     = "wg2"
	WGLocalIfaceAddr     = "10.1.1.2"
	WGLocalIfaceAddrCIDR = "10.1.1.2/32"

	WGPrivKey = "4EnHGpFp2eW+aRMK1VVWqUtorspluG5FP0/P+YnLCns="
	// WGPubKey  = "h2iGtZoTXBl7hOF6vCt5bKemrBAEsjmqLHZuAUJi6is="

	WGKeepAliveInterval = 25 * time.Second

	WGRemoteListenPort    = 51821
	WGRemotePubEndpointIP = "192.168.18.201"
	WGRemotePubKey        = "HhvuS5kX7kuqhlwnvbX7UjdFrjABQFShZ1q9qRSX9xI="
	WGRemoteIfaceAddr     = "10.1.1.1"
	WGRemoteIfaceAddrCIDR = "10.1.1.1/32"

	DelayClientStart = 5 * time.Second
)

func main() {
	logger := logrus.New()

	// Create a channel to listen for signals
	sigCh := make(chan os.Signal, 1)

	// Notify the channel on SIGINT or SIGTERM
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	ctx, cancel := context.WithTimeout(context.Background(), TunnelHandshakeTimeout)
	defer cancel()

	// Configure the tunnel
	tunnelCfg := &wg.TunnelConfig{
		PrivKey:           WGPrivKey,
		Iface:             WGLocalIfaceName,
		IfaceIPv4CIDR:     WGLocalIfaceAddrCIDR,
		ListenPort:        WGLocalListenPort,
		ReplacePeer:       true,
		CreateIface:       true,
		KeepAliveInterval: WGKeepAliveInterval,
	}

	remoteIP, remoteIPCIDR, err := net.ParseCIDR(WGRemoteIfaceAddrCIDR)
	if err != nil {
		logger.Errorf("failed to parse CIDR: %v", err)
		return
	}

	// Create the remote peer
	remotePeer := peer.Info{
		PublicKey: WGRemotePubKey,
		Endpoint: &net.UDPAddr{
			IP:   net.ParseIP(WGRemotePubEndpointIP),
			Port: WGRemoteListenPort,
		},
		AllowedIPs: []net.IPNet{
			{
				IP:   remoteIP,
				Mask: remoteIPCIDR.Mask,
			},
		},
	}

	tunnel, err := kernelwg.NewTunnel(tunnelCfg)
	if err != nil {
		logger.Errorf("failed to create tunnel: %v", err)
		return
	}

	if errStart := tunnel.Start(ctx, nil, remotePeer); errStart != nil {
		logger.Errorf("failed to start tunnel: %v", errStart)
		return
	}
	defer tunnel.Stop()

	logger.Infof("Tunnel has been stablished! Press Ctrl+C to exit.")

	// Start TCP server and client poller
	go startTCPServer(logger)

	// Start TCP client after a delay to ensure server is ready
	time.Sleep(DelayClientStart)
	go startTCPClient(logger)

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
			if errors.Is(err, io.EOF) {
				logger.Infof("connection closed by %s", c.RemoteAddr())
			} else {
				logger.Errorf("read error from %s: %v", c.RemoteAddr(), err)
			}
			return
		}
		logger.Infof("received msg from %s: %s", c.RemoteAddr().String(), string(buf[:n]))
	}
}

func startTCPClient(logger *logrus.Logger) {
	remoteServerAddr := fmt.Sprintf("%s:%d", WGRemoteIfaceAddr, TCPClientPort)
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

		time.Sleep(DelayClientStart)
	}
}
