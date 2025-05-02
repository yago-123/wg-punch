package userspacewg

import (
	"errors"
	"github.com/go-logr/logr"
	"github.com/yago-123/peer-hub/pkg/common"

	"net"

	"golang.zx2c4.com/wireguard/conn"
)

type ReceiveFunc func(bufs [][]byte, eps []UDPEndpoint) (n int, err error)

// UDPBind implements conn.Bind for a single pre-established UDP socket.
type UDPBind struct {
	conn   *net.UDPConn
	addr   *net.UDPAddr
	logger logr.Logger
}

// NewUDPBind creates a new UDPBind using an existing UDP connection.
func NewUDPBind(conn *net.UDPConn, addr *net.UDPAddr, logger logr.Logger) *UDPBind {
	return &UDPBind{
		conn:   conn,
		addr:   addr,
		logger: logger,
	}
}

// todo(): this implementation is flawed. Connections must be reused, not recreated.
// Open returns a ReceiveFunc slice for reading packets and reports the bound port.
// Since the UDP connection is pre-established, no new binding is performed (port is ignored).
func (b *UDPBind) Open(_ uint16) (fns []conn.ReceiveFunc, actualPort uint16, err error) {
	b.logger.Info("bind: Open called on existing UDP connection")

	// If the connection is nil or closed, we need to recreate it
	if b.conn == nil {
		// Recreate the connection
		conn, errListen := net.ListenUDP(common.UDPProtocol, b.addr)
		if errListen != nil {
			return nil, 0, errListen
		}
		b.conn = conn
	}

	localAddr, ok := b.conn.LocalAddr().(*net.UDPAddr)
	if !ok {
		return nil, 0, errors.New("invalid local address type")
	}

	// Define function for receiving packets. This func receives batches of packets. Fills buffer with incoming data
	// record how many bytes were read and capture sender address into endpoint type.
	recvFn := func(bufs [][]byte, sizes []int, eps []conn.Endpoint) (n int, err error) {
		// Check if the buffers, sizes, and endpoints slices are valid
		if len(bufs) == 0 || len(sizes) == 0 || len(eps) == 0 {
			return 0, nil
		}

		// Read from the UDP connection
		nRead, addr, err := b.conn.ReadFromUDP(bufs[0])
		if err != nil {
			return 0, err
		}

		// Fill the first endpoint with source address
		eps[0] = &UDPEndpoint{addr: addr}
		bufs[0] = bufs[0][:nRead]
		sizes[0] = nRead
		return 1, nil
	}

	return []conn.ReceiveFunc{recvFn}, uint16(localAddr.Port), nil
}

// Close closes the underlying UDP connection.
func (b *UDPBind) Close() error {
	b.logger.Info("bind: Close called on existing UDP connection")
	if b.conn == nil {
		return nil
	}
	err := b.conn.Close()
	b.conn = nil
	return err
}

// SetMark sets the SO_MARK option on the socket.
// This is a no-op in this implementation because net.UDPConn does not expose setting socket options.
func (b *UDPBind) SetMark(_ uint32) error {
	// Implementing SO_MARK would require access to syscall.RawConn
	// and manipulating the file descriptor manually.
	return nil
}

// Send sends the provided packet buffers to the specified endpoint.
// Only the first buffer is sent, since BatchSize returns 1.
func (b *UDPBind) Send(bufs [][]byte, ep conn.Endpoint) error {
	if len(bufs) == 0 {
		return nil
	}
	udpEp, ok := ep.(*UDPEndpoint)
	if !ok {
		return errors.New("invalid endpoint type")
	}
	_, err := b.conn.WriteToUDP(bufs[0], udpEp.addr)
	return err
}

// ParseEndpoint parses a string into a UDPEndpoint.
func (b *UDPBind) ParseEndpoint(s string) (conn.Endpoint, error) {
	addr, err := net.ResolveUDPAddr("udp", s)
	if err != nil {
		return nil, err
	}
	return &UDPEndpoint{addr: addr}, nil
}

// BatchSize returns the number of buffers expected by ReceiveFunc and Send.
// Since only single-packet operations are supported, BatchSize is 1.
func (b *UDPBind) BatchSize() int {
	return 1
}
