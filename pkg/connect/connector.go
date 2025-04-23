package connect

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/go-logr/logr"
	"github.com/yago-123/wg-punch/pkg/rendez"

	"github.com/yago-123/wg-punch/pkg/peer"
	"github.com/yago-123/wg-punch/pkg/puncher"
	"github.com/yago-123/wg-punch/pkg/rendez/client"
	"github.com/yago-123/wg-punch/pkg/util"
	"github.com/yago-123/wg-punch/pkg/wg"
)

type Connector struct {
	localPeerID  string
	puncher      puncher.Puncher
	tunnel       wg.Tunnel
	rendezClient client.Rendezvous
	waitInterval time.Duration
	logger       logr.Logger
}

func NewConnector(localPeerID string, tunnel wg.Tunnel, waitInterval time.Duration, opts ...Option) *Connector {
	cfg := newDefaultConfig()

	for _, opt := range opts {
		opt(cfg)
	}

	c := &Connector{
		localPeerID:  localPeerID,
		tunnel:       tunnel,
		waitInterval: waitInterval,
		logger:       cfg.logger,
	}

	// Rendezvous client (registers and discovers peer IPs)
	rendezClient := client.NewRendezvous(cfg.rendezServerURL)
	// STUN-based hole puncher
	puncher := puncher.NewPuncher(cfg.stunServers)

	c.rendezClient = rendezClient
	c.puncher = puncher

	return &Connector{
		localPeerID:  localPeerID,
		tunnel:       tunnel,
		rendezClient: rendezClient,
		puncher:      puncher,
		waitInterval: waitInterval,
		logger:       cfg.logger,
	}

}

func (c *Connector) Connect(ctx context.Context, localAddr *net.UDPAddr, allowedIPs []string, remotePeerID, localPubKey string) (net.Conn, error) {
	conn, err := net.ListenUDP(util.UDPProtocol, localAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to bind UDP: %w", err)
	}

	// Discover own public address via STUN
	publicAddr, err := c.puncher.PublicAddr(ctx, conn)
	if err != nil {
		return nil, fmt.Errorf("failed to get public addr: %w", err)
	}

	// Register local peer in rendezvous server
	localPeerInfo := rendez.RegisterRequest{
		PeerID:     c.localPeerID,
		PublicKey:  localPubKey,
		Endpoint:   publicAddr.String(),
		AllowedIPs: allowedIPs,
	}
	if errRendez := c.rendezClient.Register(ctx, localPeerInfo); errRendez != nil {
		return nil, fmt.Errorf("failed to register with rendezvous server: %w", errRendez)
	}

	c.logger.Info("Registered local peer", "peerID", c.localPeerID, "publicKey", localPubKey, "endpoint", publicAddr.String(), "allowedIPs", allowedIPs)

	// Wait for peer info from the rendezvous server
	remotePeerInfo, endpoint, err := c.rendezClient.WaitForPeer(ctx, remotePeerID, c.waitInterval)
	if err != nil {
		return nil, fmt.Errorf("failed to get peer info: %w", err)
	}

	// Create UDP connection on local public IP
	// todo() : adjust localAddr to be passed in a more clean way
	conn, err = c.puncher.Punch(ctx, conn, endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to punch: %w", err)
	}

	// Adjust allowedIPs from string to IP format
	remoteAllowedIPs, err := util.ConvertAllowedIPs(remotePeerInfo.AllowedIPs)
	if err != nil {
		return nil, fmt.Errorf("failed to convert allowed IPs: %w", err)
	}

	c.logger.Info("Connecting to remote peer", "peerID", remotePeerID, "endpoint", endpoint.String(), "allowedIPs", remoteAllowedIPs)

	// Start WireGuard tunnel
	if errTunnel := c.tunnel.Start(ctx, conn, peer.Info{
		PublicKey:  remotePeerInfo.PublicKey,
		Endpoint:   endpoint,
		AllowedIPs: remoteAllowedIPs,
	}); errTunnel != nil {
		return nil, fmt.Errorf("failed to start wireguard tunnel: %w", errTunnel)
	}

	// Return net.Conn (use the raw conn or wrap it)
	return conn, nil
}
