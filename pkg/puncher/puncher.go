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
	Punch(ctx context.Context, conn *net.UDPConn, remoteHint *net.UDPAddr) (*net.UDPConn, error)
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

func (p *puncher) Punch(ctx context.Context, conn *net.UDPConn, remoteHint *net.UDPAddr) (*net.UDPConn, error) {
	// If remoteHint is nil, return an error
	if remoteHint == nil {
		return nil, fmt.Errorf("remote hint required for punching")
	}

	// todo(): adjust
	if conn == nil {
		return nil, fmt.Errorf("conn required for punching")
	}

	p.logger.Info("punching remote host", "remoteHint", remoteHint.String())

	// Try sending empty UDP packets to open NAT mappings
	go func() {
		ticker := time.NewTicker(p.puncherInterval)
		defer ticker.Stop()

		for {
			select {
			// todo(): make sure this can be cancelled once the tunnel have handshaked
			case <-ctx.Done():
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

	return conn, nil
}

func (p *puncher) PublicAddr(ctx context.Context, conn *net.UDPConn) (*net.UDPAddr, error) {
	return util.GetPublicEndpoint(ctx, conn, p.stunServers)
}
