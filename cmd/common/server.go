package common

import (
	"fmt"
	"io"
	"net"

	"github.com/go-logr/logr"
	"github.com/yago-123/wg-punch/pkg/util"
)

const (
	TCPMaxBuffer = 1024
)

type TCPServer struct {
	localAddr string
	port      int
	ln        net.Listener
	logger    logr.Logger
}

func NewTCPServer(localAddr string, port int, logger logr.Logger) (*TCPServer, error) {
	serverAddr := fmt.Sprintf("%s:%d", localAddr, port)
	ln, err := net.Listen(util.TCPProtocol, serverAddr)
	if err != nil {
		logger.Error(err, "TCP server listen error", "address", serverAddr)
		return nil, fmt.Errorf("TCP server listen error: %w", err)
	}

	return &TCPServer{
		localAddr: localAddr,
		port:      port,
		ln:        ln,
		logger:    logger,
	}, nil
}

func (s *TCPServer) Start() {
	s.logger.Info("TCP started serving", "address", s.ln.Addr().String())

	go func() {
		for {
			conn, errServer := s.ln.Accept()
			if errServer != nil {
				s.logger.Error(errServer, "error accepting connection", "address", s.ln.Addr().String())
				continue
			}

			go s.handleTCPConnection(conn)
		}
	}()
}

func (s *TCPServer) Close() {
	s.ln.Close()
}

func (s *TCPServer) handleTCPConnection(c net.Conn) {
	defer c.Close()
	buf := make([]byte, TCPMaxBuffer)

	for {
		n, err := c.Read(buf)
		if err != nil {
			if err == io.EOF {
				s.logger.Info("TCP client disconnected", "address", c.RemoteAddr().String())
			} else {
				s.logger.Info("TCP client read error", "address", c.RemoteAddr().String())
			}
			return
		}

		s.logger.Info("TCP received message", "remoteAddr", c.RemoteAddr().String(), "message", string(buf[:n]))
	}
}
