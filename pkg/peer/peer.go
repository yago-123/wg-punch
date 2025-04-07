package peer

import "net"

type PeerInfo struct {
	PublicKey  string
	Endpoint   *net.UDPAddr
	AllowedIPs []net.IPNet
}
