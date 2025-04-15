package connect

import (
	"context"
	"fmt"
	"log"
	"net"
	"time"

	"github.com/yago-123/wg-punch/pkg/peer"
	"github.com/yago-123/wg-punch/pkg/puncher"
	"github.com/yago-123/wg-punch/pkg/rendez/client"
	"github.com/yago-123/wg-punch/pkg/rendez/types"
	"github.com/yago-123/wg-punch/pkg/util"
	"github.com/yago-123/wg-punch/pkg/wg"
)

type Connector struct {
	localPeerID  string
	puncher      puncher.Puncher
	tunnel       wg.Tunnel
	rendezClient client.Rendezvous
	waitInterval time.Duration
}

func NewConnector(localPeerID string, p puncher.Puncher, tunnel wg.Tunnel, rendezClient client.Rendezvous, waitInterval time.Duration) *Connector {
	return &Connector{
		localPeerID:  localPeerID,
		puncher:      p,
		tunnel:       tunnel,
		rendezClient: rendezClient,
		waitInterval: waitInterval,
	}
}

func (c *Connector) Connect(ctx context.Context, localAddr *net.UDPAddr, allowedIPs []string, remotePeerID, localPrivKey, localPubKey string) (net.Conn, error) {
	// Discover own public address via STUN
	publicAddr, err := c.puncher.PublicAddr(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get public addr: %w", err)
	}

	// Register local peer in rendezvous server
	if errRendez := c.rendezClient.Register(ctx, types.RegisterRequest{
		PeerID:     c.localPeerID,
		PublicKey:  localPubKey,
		Endpoint:   publicAddr.String(),
		AllowedIPs: allowedIPs,
	}); errRendez != nil {
		return nil, fmt.Errorf("failed to register with rendezvous server: %w", errRendez)
	}

	log.Printf("Registered local peer %s with public address %s and allowed IPs %s", c.localPeerID, publicAddr.String(), allowedIPs)

	// Wait for peer info from the rendezvous server
	peerInfo, endpoint, err := c.rendezClient.WaitForPeer(ctx, remotePeerID, c.waitInterval)
	if err != nil {
		return nil, fmt.Errorf("failed to get peer info: %w", err)
	}

	// Adjust allowedIPs from string to IP format
	allowedIPsPeer, err := util.ConvertAllowedIPs(peerInfo.AllowedIPs)
	if err != nil {
		return nil, fmt.Errorf("failed to convert allowed IPs: %w", err)
	}

	// Create UDP connection on local public IP
	// todo() : adjust localAddr to be passed in a more clean way
	conn, err := c.puncher.Punch(ctx, localAddr, endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to punch: %w", err)
	}

	// Start WireGuard tunnel
	if errTunnel := c.tunnel.Start(conn, localPrivKey, peer.Info{
		PublicKey:  peerInfo.PublicKey,
		Endpoint:   endpoint,
		AllowedIPs: allowedIPsPeer,
	}); errTunnel != nil {
		return nil, fmt.Errorf("failed to start wireguard tunnel: %w", errTunnel)
	}

	// Return net.Conn (use the raw conn or wrap it)
	return conn, nil
}
