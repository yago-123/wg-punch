package util

import (
	"context"
	"fmt"
	"net"

	"github.com/pion/stun"
)

// GetPublicEndpoint tries the provided STUN servers to discover the public-facing IP.
// Returns the endpoint as "IP:port", or an error if all servers fail.
func GetPublicEndpoint(ctx context.Context, servers []string) (*net.UDPAddr, error) {
	var lastErr error

	for _, server := range servers {
		endpoint, err := trySTUNServer(ctx, server)
		if err == nil {
			return endpoint, nil
		}

		lastErr = err
	}

	return nil, fmt.Errorf("all STUN servers failed: %w", lastErr)
}

func trySTUNServer(ctx context.Context, server string) (*net.UDPAddr, error) {
	dialer := &net.Dialer{}
	conn, err := dialer.DialContext(ctx, "udp", server)
	if err != nil {
		return nil, fmt.Errorf("error dialing STUN server %s: %w", server, err)
	}
	defer conn.Close()

	client, err := stun.NewClient(conn)
	if err != nil {
		return nil, fmt.Errorf("error creating STUN client: %w", err)
	}
	defer client.Close()

	resultCh := make(chan *net.UDPAddr, 1)
	errCh := make(chan error, 1)

	go func() {
		var xorAddr stun.XORMappedAddress
		err := client.Do(stun.MustBuild(stun.TransactionID, stun.BindingRequest), func(res stun.Event) {
			if res.Error != nil {
				errCh <- res.Error
				return
			}
			if getErr := xorAddr.GetFrom(res.Message); getErr != nil {
				errCh <- fmt.Errorf("failed to get XOR-MAPPED-ADDRESS: %w", getErr)
				return
			}
			resultCh <- &net.UDPAddr{
				IP:   xorAddr.IP,
				Port: xorAddr.Port,
			}
		})

		if err != nil {
			errCh <- fmt.Errorf("STUN request to %s failed: %w", server, err)
		}
	}()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case err := <-errCh:
		return nil, err
	case addr := <-resultCh:
		return addr, nil
	}
}
