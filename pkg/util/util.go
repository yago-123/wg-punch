package util

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/pion/stun"
)

const (
	UDPMaxBuffer       = 1500
	DefaultSTUNTimeout = 2 * time.Second
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
func GetPublicEndpoint(ctx context.Context, conn *net.UDPConn, servers []string) (*net.UDPAddr, error) {
	var lastErr error

	for _, server := range servers {
		endpoint, err := trySTUNServer(ctx, conn, server)
		if err == nil {
			return endpoint, nil
		}

		lastErr = err
	}

	return nil, fmt.Errorf("all STUN servers failed: %w", lastErr)
}

// todo(): adjust hardcoded values
func trySTUNServer(ctx context.Context, conn *net.UDPConn, server string) (*net.UDPAddr, error) {
	serverAddr, err := net.ResolveUDPAddr(UDPProtocol, server)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve STUN server %q: %w", server, err)
	}

	// Build STUN Binding Request
	req := stun.MustBuild(stun.TransactionID, stun.BindingRequest)

	// Send UDP request to STUN server
	if _, errWrite := conn.WriteToUDP(req.Raw, serverAddr); errWrite != nil {
		return nil, fmt.Errorf("failed to send STUN request to %s: %w", server, errWrite)
	}

	// Respect context deadline for read timeout
	deadline, hasDeadline := ctx.Deadline()
	if hasDeadline {
		_ = conn.SetReadDeadline(deadline)
	} else {
		_ = conn.SetReadDeadline(time.Now().Add(DefaultSTUNTimeout))
	}

	buf := make([]byte, UDPMaxBuffer)
	n, _, err := conn.ReadFromUDP(buf)
	if err != nil {
		return nil, fmt.Errorf("failed to read STUN response from %s: %w", server, err)
	}

	var res stun.Message
	res.Raw = buf[:n]
	if errDecode := res.Decode(); errDecode != nil {
		return nil, fmt.Errorf("failed to decode STUN response: %w", errDecode)
	}

	var xorAddr stun.XORMappedAddress
	if errAddr := xorAddr.GetFrom(&res); errAddr != nil {
		return nil, fmt.Errorf("failed to extract XOR-MAPPED-ADDRESS: %w", errAddr)
	}

	return &net.UDPAddr{
		IP:   xorAddr.IP,
		Port: xorAddr.Port,
	}, nil
}
