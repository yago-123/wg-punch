package wgpunch

import (
	"context"
	"fmt"
	"github.com/yago-123/wg-punch/pkg/peer"
	"net"
	"time"

	"github.com/yago-123/wg-punch/pkg/puncher"
	"github.com/yago-123/wg-punch/pkg/wg"

	"github.com/yago-123/wg-punch/pkg/rendez/client"
)

type Connector struct {
	puncher      puncher.Puncher
	tunnel       wg.Tunnel
	rendezClient client.Rendezvous
}

func NewConnector(p puncher.Puncher, tunnel wg.Tunnel, rendezClient client.Rendezvous) *Connector {
	return &Connector{
		puncher:      p,
		tunnel:       tunnel,
		rendezClient: rendezClient}
}

func (c *Connector) Connect(ctx context.Context, peerID, localPrivateKey string) (net.Conn, error) {
	// Discover own public address via STUN
	publicAddr, err := c.puncher.PublicAddr(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get public addr: %w", err)
	}

	// Register to rendezvous server
	// todo(): this might not be necessary
	// err = c.rendezClient.Register(ctx, types.RegisterRequest{})
	// if err != nil {
	// 	return nil, fmt.Errorf("failed to register with rendezvous server: %w", err)
	// }

	// todo(): figure how to pass this argument
	interval := 1 * time.Second

	// Wait for peer info from the rendezvous server
	peerInfo, endpoint, err := c.rendezClient.WaitForPeer(ctx, peerID, interval)
	if err != nil {
		return nil, fmt.Errorf("failed to get peer info: %w", err)
	}

	// Create UDP connection on local public IP
	conn, err := c.puncher.Punch(ctx, publicAddr.IP.String(), endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to punch: %w", err)
	}

	// Start WireGuard tunnel
	err = c.tunnel.Start(conn, localPrivateKey, peer.Info{
		PublicKey:  peerInfo.PublicKey,
		Endpoint:   endpoint,
		AllowedIPs: peerInfo.AllowedIPs,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to start wireguard tunnel: %w", err)
	}

	// Return net.Conn (use the raw conn or wrap it)
	return conn, nil
}
