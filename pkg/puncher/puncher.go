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

	if conn == nil {
		return func() {}, fmt.Errorf("UDP connection must be initialized in order to punch remote host")
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
			}
		}
	}()

	return cancelPunch, nil
}

// PublicAddr retrieves the public address of the local peer by using STUN servers. It is used to discover the public
// IP and port of the local peer, which is necessary for establishing a connection with the remote peer
func (p *puncher) PublicAddr(ctx context.Context, conn *net.UDPConn) (*net.UDPAddr, error) {
	return util.GetPublicEndpoint(ctx, conn, p.stunServers)
}
