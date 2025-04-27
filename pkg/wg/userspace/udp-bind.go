package userspacewg

import (
	"golang.zx2c4.com/wireguard/conn"
	"net"
)

// UDPBind wraps a UDP connection and implements the conn.Bind interface.
type UDPBind struct {
	conn *net.UDPConn
}

func NewUDPBind(conn *net.UDPConn) *UDPBind {
	return &UDPBind{conn: conn}
}

// Open binds the UDP connection to a local address and port. It returns the actual port that was bound to.
func (b *UDPBind) Open(_ uint16, _ uint16) (conn.Endpoint, error) {
	local := b.conn.LocalAddr().(*net.UDPAddr)
	return &UDPEndpoint{addr: local}, nil
}

// SetMark is a stub for setting packet marks (fwmarks) on the UDP socket. In this implementation, SetMark is a
// no-op and does not modify the socket. It exists to satisfy the interface.
func (b *UDPBind) SetMark(_ uint32) error {
	return nil
}

// Send s a buffer to the specified endpoint. The buffer must not exceed the maximum size of the socket's send buffer.
func (b *UDPBind) Send(buf []byte, ep conn.Endpoint) error {
	_, err := b.conn.WriteToUDP(buf, ep.(*UDPEndpoint).addr)
	return err
}

// Receive reads a packet from the UDP connection and returns the number of bytes read, the source endpoint,
// and any error encountered.
func (b *UDPBind) Receive(buf []byte) (int, conn.Endpoint, error) {
	n, addr, err := b.conn.ReadFromUDP(buf)
	return n, &UDPEndpoint{addr: addr}, err
}
