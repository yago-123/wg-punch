package puncher

import (
	"context"
	"net"
	"time"

	"github.com/yago-123/wg-punch/pkg/util"
)

const (
	IntervalUDPPackets = 300 * time.Millisecond
)

type Puncher interface {
	Punch(ctx context.Context, localAddr string, remoteHint *net.UDPAddr) (*net.UDPConn, error)
	PublicAddr() (*net.UDPAddr, error)
}

type puncher struct {
	stunServers []string
}

func NewPuncher(stunServers []string) Puncher {
	return &puncher{
		stunServers: stunServers,
	}
}

func (p *puncher) Punch(ctx context.Context, localAddr string, remoteHint *net.UDPAddr) (*net.UDPConn, error) {
	// Listen for UDP packets on the local address
	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP(localAddr)})
	if err != nil {
		return nil, err
	}

	// Perform the punch
	if remoteHint != nil {
		// Try sending empty UDP packets to open NAT mappings
		go func() {
			ticker := time.NewTicker(IntervalUDPPackets)
			defer ticker.Stop()

			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					_, _ = conn.WriteToUDP([]byte("punch"), remoteHint)
				}
			}
		}()
	}

	return conn, nil
}

func (p *puncher) PublicAddr() (*net.UDPAddr, error) {
	return util.GetPublicEndpoint(p.stunServers)
}
