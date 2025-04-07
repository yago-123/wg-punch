package peer

import "net"

type Info struct {
	PublicKey  string
	Endpoint   *net.UDPAddr
	AllowedIPs []net.IPNet
}
