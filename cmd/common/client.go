package common

import (
	"context"
	"fmt"
	"net"
	"strconv"

	"github.com/go-logr/logr"
	"github.com/yago-123/wg-punch/pkg/util"
)

type TCPClient struct {
	conn   net.Conn
	logger logr.Logger
}

func NewTCPClient(remoteAddr string, remotePort int, logger logr.Logger) (*TCPClient, error) {
	remoteServerAddr := net.JoinHostPort(remoteAddr, strconv.Itoa(remotePort))
	var dialer net.Dialer
	conn, err := dialer.DialContext(context.Background(), util.TCPProtocol, remoteServerAddr)
	if err != nil {
		logger.Error(err, "TCP client listen error", "address", remoteServerAddr)
		return nil, fmt.Errorf("TCP client listen error: %w", err)
	}

	return &TCPClient{
		conn:   conn,
		logger: logger,
	}, nil
}

func (c *TCPClient) Send(msg string) {
	_, err := c.conn.Write([]byte(msg))
	if err != nil {
		c.logger.Error(err, "TCP client write error", "message", msg)
	}

	c.logger.Info("TCP client sent", "message", msg)
}

func (c *TCPClient) Close() {
	c.conn.Close()
}
