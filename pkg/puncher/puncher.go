package puncher

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/go-logr/logr"

	"github.com/yago-123/wg-punch/pkg/util"
)

const (
	PunchMessage = "punch"
)

type Puncher interface {
	Punch(ctx context.Context, conn *net.UDPConn, remoteHint *net.UDPAddr) (context.CancelFunc, error)
	PublicAddr(ctx context.Context, conn *net.UDPConn) (*net.UDPAddr, error)
}

type puncher struct {
	puncherInterval time.Duration
	stunServers     []string
	logger          logr.Logger
}

func NewPuncher(opts ...Option) Puncher {
	cfg := newDefaultConfig()

	for _, opt := range opts {
		opt(cfg)
	}

	return &puncher{
		puncherInterval: cfg.puncherInterval,
		stunServers:     cfg.stunServers,
		logger:          cfg.logger,
	}
}

// Punch attempts to establish a UDP connection with the remote peer by sending empty UDP packets. The return value is
// a cancel function that must be called to stop the spawned punching process that happen in the background
func (p *puncher) Punch(ctx context.Context, conn *net.UDPConn, remoteHint *net.UDPAddr) (context.CancelFunc, error) {
	// If remoteHint is nil, return an error
	if remoteHint == nil {
		return func() {}, fmt.Errorf("remote hint required for punching")
	}

	// todo(): adjust
	if conn == nil {
		return func() {}, fmt.Errorf("conn required for punching")
	}

	p.logger.Info("punching remote host", "remoteHint", remoteHint.String())

	ctxPunch, cancelPunch := context.WithCancel(ctx)

	// Try sending empty UDP packets to open NAT mappings
	go func() {
		ticker := time.NewTicker(p.puncherInterval)
		defer ticker.Stop()

		for {
			select {
			// This context might be triggered if the handshake timeout expires or if the cancel func is called,
			// the cancel func must be called before the WireGuard tunnel is started so that the connection is
			// managed by a single entity
			case <-ctxPunch.Done():
				return
			case <-ticker.C:
				_, errConn := conn.WriteToUDP([]byte(PunchMessage), remoteHint)
				// The connection will be closed right before the WireGuard tunnel is started
				if errors.Is(errConn, net.ErrClosed) {
					return
				}

				// if errConn != nil {
				// todo(): handle
			}
		}
	}()

	return cancelPunch, nil
}

func (p *puncher) PublicAddr(ctx context.Context, conn *net.UDPConn) (*net.UDPAddr, error) {
	return util.GetPublicEndpoint(ctx, conn, p.stunServers)
}
