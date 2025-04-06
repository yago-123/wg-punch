package main

import (
	"context"
	"net"
)

type RendezvousClient interface {
	Register(ctx context.Context, key string, port int) error
	Discover(ctx context.Context, key string) (*net.UDPAddr, error)
}

func NewRendezvousClient(url string) RendezvousClient {
	// returns HTTP or gRPC implementation
	return nil
}
