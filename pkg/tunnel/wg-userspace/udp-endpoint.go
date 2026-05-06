package userspacewg

import (
	"encoding/binary"
	"fmt"
	"net"
	"net/netip"
)

const udpPortByteLen = 2

func udpPortToUint16(port int) (uint16, error) {
	if port < 0 || port > int(^uint16(0)) {
		return 0, fmt.Errorf("udp port out of range: %d", port)
	}

	return uint16(port), nil
}

// UDPEndpoint implements the conn.Endpoint interface for UDP connections.
type UDPEndpoint struct {
	src  *net.UDPAddr // where the packet came from
	addr *net.UDPAddr // where to send packets
}

// ClearSrc clears the source address of the endpoint.
func (ep *UDPEndpoint) ClearSrc() {
	ep.src = nil
}

// SrcToString returns the source address as a string.
func (ep *UDPEndpoint) SrcToString() string {
	if ep.src != nil {
		return ep.src.String()
	}
	return ""
}

// DstToString returns the destination address as a string.
func (ep *UDPEndpoint) DstToString() string {
	return ep.addr.String()
}

// DstToBytes converts the destination address to a byte slice.
func (ep *UDPEndpoint) DstToBytes() []byte {
	ip := ep.addr.IP.To16()
	port := make([]byte, udpPortByteLen)
	portValue, err := udpPortToUint16(ep.addr.Port)
	if err != nil {
		return ip
	}
	binary.BigEndian.PutUint16(port, portValue)
	return append(ip, port...)
}

// DstIP returns the destination IP address.
func (ep *UDPEndpoint) DstIP() netip.Addr {
	addr, _ := netip.AddrFromSlice(ep.addr.IP)
	return addr
}

// SrcIP returns the source IP address.
func (ep *UDPEndpoint) SrcIP() netip.Addr {
	if ep.src == nil {
		return netip.Addr{}
	}
	addr, _ := netip.AddrFromSlice(ep.src.IP)
	return addr
}
