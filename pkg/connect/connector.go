package connect

import (
	"context"
	"fmt"
	"log"
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
	stunServers  []string
	logger       logr.Logger
}

func NewConnector(localPeerID string, p puncher.Puncher, tunnel wg.Tunnel, rendezClient client.Rendezvous, waitInterval time.Duration, opts ...Option) *Connector {
	c := &Connector{
		localPeerID:  localPeerID,
		puncher:      p,
		tunnel:       tunnel,
		rendezClient: rendezClient,
		waitInterval: waitInterval,
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
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

	log.Printf("Registered local peer %s with rendezvous server. Pub endpoint %s and allowed IPs %s", c.localPeerID, publicAddr.String(), allowedIPs)

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

	log.Printf("Connecting to remote peer %s with endpoint %s and allowed IPs %s", remotePeerID, endpoint.String(), remoteAllowedIPs)

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
