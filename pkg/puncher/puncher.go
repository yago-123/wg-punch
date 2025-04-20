package puncher

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"time"

	"github.com/yago-123/wg-punch/pkg/util"
)

const (
	IntervalUDPPackets = 300 * time.Millisecond
)

type Puncher interface {
	Punch(ctx context.Context, localAddr *net.UDPAddr, remoteHint *net.UDPAddr) (*net.UDPConn, error)
	PublicAddr(ctx context.Context) (*net.UDPAddr, error)
}

type puncher struct {
	stunServers []string
}

func NewPuncher(stunServers []string) Puncher {
	return &puncher{
		stunServers: stunServers,
	}
}

func (p *puncher) Punch(ctx context.Context, localAddr *net.UDPAddr, remoteHint *net.UDPAddr) (*net.UDPConn, error) {
	// If remoteHint is nil, return an error
	if remoteHint == nil {
		return nil, fmt.Errorf("remote hint required for punching")
	}

	// todo() remove
	log.Printf("Punching remote peer %s", remoteHint)
	log.Printf("Using local address %s", localAddr)

	// Listen for UDP packets on the local address
	conn, err := net.ListenUDP("udp", localAddr)
	if err != nil {
		return nil, err
	}

	// Try sending empty UDP packets to open NAT mappings
	go func() {
		ticker := time.NewTicker(IntervalUDPPackets)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				_, errConn := conn.WriteToUDP([]byte("punch"), remoteHint)
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

func (p *puncher) PublicAddr(ctx context.Context) (*net.UDPAddr, error) {
	return util.GetPublicEndpoint(ctx, p.stunServers)
}
