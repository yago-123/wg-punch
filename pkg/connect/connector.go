package connect

import (
	"context"
	"net"

	"github.com/yago-123/peer-hub/pkg/types"

	errors "github.com/yago-123/wg-punch/pkg/error"

	"github.com/go-logr/logr"

	"github.com/yago-123/peer-hub/pkg/client"
	"github.com/yago-123/wg-punch/pkg/peer"
	"github.com/yago-123/wg-punch/pkg/puncher"
	"github.com/yago-123/wg-punch/pkg/util"
	"github.com/yago-123/wg-punch/pkg/wg"
)

type Connector struct {
	localPeerID string
	puncher     puncher.Puncher
	rendClient  client.Rendezvous
	logger      logr.Logger
}

func NewConnector(localPeerID string, puncher puncher.Puncher, opts ...Option) *Connector {
	cfg := newDefaultConfig()

	for _, opt := range opts {
		opt(cfg)
	}

	// Rendezvous client (registers and discovers peer IPs)
	rendClient := client.New(cfg.rendezServerURL, cfg.waitInterval)

	return &Connector{
		localPeerID: localPeerID,
		rendClient:  rendClient,
		puncher:     puncher,
		logger:      cfg.logger,
	}
}

// Connect handles the connection process between two peers. From registering the peer until the handshake is done.
func (c *Connector) Connect(ctx context.Context, tunnel wg.Tunnel, allowedIPs []string, remotePeerID string) (net.Conn, error) {
	localAddr := &net.UDPAddr{IP: net.IPv4zero, Port: tunnel.ListenPort()}

	conn, err := net.ListenUDP(util.UDPProtocol, localAddr)
	if err != nil {
		return nil, errors.Wrap(errors.ErrBindingUDP, err)
	}

	// Discover own public address via STUN
	publicAddr, err := c.puncher.PublicAddr(ctx, conn)
	if err != nil {
		return nil, errors.Wrap(errors.ErrPubAddrRetrieve, err)
	}

	// Register local peer in rendezvous server
	localPeerInfo := types.RegisterRequest{
		PeerID:     c.localPeerID,
		PublicKey:  tunnel.PublicKey(),
		Endpoint:   publicAddr.String(),
		AllowedIPs: allowedIPs,
	}
	if errRendez := c.rendClient.Register(ctx, localPeerInfo); errRendez != nil {
		return nil, errors.Wrap(errors.ErrRegisterPeer, errRendez)
	}

	c.logger.Info("Registered local peer", "peerID", c.localPeerID, "publicKey", tunnel.PublicKey(), "endpoint", publicAddr.String(), "allowedIPs", allowedIPs)

	// Wait for peer info from the rendezvous server
	remotePeerInfo, endpoint, err := c.rendClient.WaitForPeer(ctx, remotePeerID)
	if err != nil {
		return nil, errors.Wrap(errors.ErrWaitForPeer, err)
	}

	// Create UDP connection on local public IP
	cancelPunch, errPunch := c.puncher.Punch(ctx, conn, endpoint)
	if errPunch != nil {
		return nil, errors.Wrap(errors.ErrPunchingNAT, errPunch)
	}

	// Adjust allowedIPs from string to IP format
	remoteAllowedIPs, err := util.ConvertAllowedIPs(remotePeerInfo.AllowedIPs)
	if err != nil {
		return nil, errors.Wrap(errors.ErrConvertAllowed, err)
	}

	c.logger.Info("Connecting to remote peer", "peerID", remotePeerID, "endpoint", endpoint.String(), "allowedIPs", remoteAllowedIPs)

	// todo(): pass this argument to the tunnel so that it can be triggered RIGHT BEFORE the connection takeover
	cancelPunch()

	// Start WireGuard tunnel
	if errTunnel := tunnel.Start(ctx, conn, peer.Info{
		PublicKey:  remotePeerInfo.PublicKey,
		Endpoint:   endpoint,
		AllowedIPs: remoteAllowedIPs,
	}); errTunnel != nil {
		return nil, errors.Wrap(errors.ErrTunnelStart, errTunnel)
	}

	// Return net.Conn (use the raw conn or wrap it)
	return conn, nil
}
