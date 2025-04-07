package wgpunch

import (
	"context"
	"net"
)

type Puncher interface {
	Punch(ctx context.Context, localAddr string, remoteHint *net.UDPAddr) (*net.UDPConn, error)
}

func NewPuncher(_ []string) Puncher {
	// stunServers
	// returns an implementation (e.g., pion-backed)
	return nil
}
