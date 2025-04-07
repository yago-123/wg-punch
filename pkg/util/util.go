package util

import (
	"fmt"
	"net"

	"github.com/pion/stun"
)

// GetPublicEndpoint tries the provided STUN servers to discover the public-facing IP.
// Returns the endpoint as "IP:port", or an error if all servers fail.
func GetPublicEndpoint(servers []string) (*net.UDPAddr, error) {
	var lastErr error

	for _, server := range servers {
		endpoint, err := trySTUNServer(server)
		if err == nil {
			return endpoint, nil
		}

		lastErr = err
	}

	return nil, fmt.Errorf("all STUN servers failed: %w", lastErr)
}

func trySTUNServer(server string) (*net.UDPAddr, error) {
	conn, err := net.Dial("udp", server)
	if err != nil {
		return nil, fmt.Errorf("error dialing STUN server %s: %w", server, err)
	}
	defer conn.Close()

	// Create a new STUN client
	client, err := stun.NewClient(conn)
	if err != nil {
		return nil, fmt.Errorf("error creating STUN client: %w", err)
	}
	defer client.Close()

	// Send a binding request to the STUN server for determining the public IP
	var xorAddr stun.XORMappedAddress
	if errStun := client.Do(stun.MustBuild(stun.TransactionID, stun.BindingRequest), func(res stun.Event) {
		if res.Error != nil {
			err = res.Error
			return
		}
		if getErr := xorAddr.GetFrom(res.Message); getErr != nil {
			err = fmt.Errorf("failed to get XOR-MAPPED-ADDRESS: %w", getErr)
		}
	}); errStun != nil {
		return nil, fmt.Errorf("STUN request to %s failed: %w", server, errStun)
	}

	return &net.UDPAddr{
		IP:   xorAddr.IP,
		Port: xorAddr.Port,
		Zone: "", // leave empty unless you have a link-local IPv6
	}, nil
}
