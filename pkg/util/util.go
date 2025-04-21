package util

import (
	"context"
	"fmt"
	"net"

	"github.com/pion/stun"
)

// ConvertAllowedIPs takes a slice of CIDR strings and converts them to a slice of net.IPNet.
// It returns an error if any string is not a valid CIDR.
func ConvertAllowedIPs(allowedIPs []string) ([]net.IPNet, error) {
	var result []net.IPNet

	for _, cidr := range allowedIPs {
		_, ipNet, err := net.ParseCIDR(cidr)
		if err != nil {
			return nil, fmt.Errorf("invalid CIDR %q: %w", cidr, err)
		}
		result = append(result, *ipNet)
	}

	return result, nil
}

// GetPublicEndpoint attempts to discover the public-facing UDP address of the local machine by querying a list of STUN
// servers. It sends a STUN Binding Request through the provided UDP connection and returns the first successful
// response.
func GetPublicEndpoint(ctx context.Context, servers []string, conn *net.UDPConn) (*net.UDPAddr, error) {
	var lastErr error

	for _, server := range servers {
		endpoint, err := trySTUNServer(ctx, server, conn)
		if err == nil {
			return endpoint, nil
		}

		lastErr = err
	}

	return nil, fmt.Errorf("all STUN servers failed: %w", lastErr)
}

func trySTUNServer(ctx context.Context, server string, conn *net.UDPConn) (*net.UDPAddr, error) {
	client, err := stun.NewClient(conn)
	if err != nil {
		return nil, fmt.Errorf("error creating STUN client: %w", err)
	}
	defer client.Close()

	resultCh := make(chan *net.UDPAddr, 1)
	errCh := make(chan error, 1)

	go func() {
		var xorAddr stun.XORMappedAddress
		err = client.Do(stun.MustBuild(stun.TransactionID, stun.BindingRequest), func(res stun.Event) {
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
	case err = <-errCh:
		return nil, err
	case addr := <-resultCh:
		return addr, nil
	}
}
