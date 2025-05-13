![Alt text](https://github.com/user-attachments/assets/7225f4e0-e949-470a-b07f-a9c9a4a81530)

## About
`wg-punch` is a library for seamless **NAT hole punching** via UDP and `WireGuard`. It operates with a userspace TCP/IP 
stack, facilitating peer-to-peer communication by punching through NATs and firewalls over UDP, while WireGuard 
establishes L3 encrypted tunnels and overlay networks for private, secure connections.

## Extension
This library is designed to be customizable and extensible. It supports switching VPN tunnel implementations, allowing 
you to easily swap the current tunnel for others. The original implementation uses WireGuard in userspace, but you can 
switch to alternative algorithms, such as the [WireGuard kernel implementation (**wg-punch-kernel**)](https://github.com/yago-123/wg-punch-kernel), 
`OpenVPN`, `IPSec`, or any other tunneling protocol by extending the tunnel interface in `pkg/tunnel/tunnel.go`.

`todo()`: move `peer-hub` interface definition to this library aswell. 

Additionally, the library supports customizable synchronization by implementing the [Rendezvous client interface](https://github.com/yago-123/peer-hub/blob/19fd6d2b7af2f09cfc305ccb613efe06d3d0bb65/pkg/client/client.go#L19)
from `peer-hub` and integrating your own backend.

## Sample usage
```Go
package main

const (
	TunnelHandshakeTimeout = 30 * time.Second
	RendezvousServer       = "http://rendezvous.yago.ninja:7777"
	
	LocalPeerID  = "o1"
	RemotePeerID = "o2"
	
	WGLocalListenPort    = 51821
	WGLocalIfaceName     = "wg1"
	WGLocalIfaceAddr     = "10.1.1.1"
	WGLocalIfaceAddrCIDR = "10.1.1.1/32" 
	
	RemotePeerIP = "10.1.1.2"
)

func main() {
	// ... 

	puncherOptions := []puncher.Option{
		puncher.WithPuncherInterval(300 * time.Millisecond),
		puncher.WithSTUNServers(stunServers),
		puncher.WithLogger(logger),
	}
	// Create a puncher with the STUN servers
	p := puncher.NewPuncher(puncherOptions...)

	connectorOptions := []connect.Option{
		connect.WithRendezServer(RendezvousServer),
		connect.WithWaitInterval(1 * time.Second),
		connect.WithLogger(logger),
	}
	// Create a connector with the puncher
	conn := connect.NewConnector(LocalPeerID, p, connectorOptions...)

	ctxHandshake, cancel := context.WithTimeout(context.Background(), TunnelHandshakeTimeout)
	defer cancel()

	tunnelCfg := &tunnel.Config{
		PrivKey:           WGLocalPrivKey,
		Iface:             WGLocalIfaceName,
		IfaceIPv4CIDR:     WGLocalIfaceAddrCIDR,
		ListenPort:        WGLocalListenPort,
		ReplacePeer:       true,
		CreateIface:       true,
		KeepAliveInterval: WGKeepAliveInterval,
	}

	// Initialize WireGuard interface using WireGuard
	tunnel, err := wguserspace.New(tunnelCfg, logger)
	if err != nil {
		logger.Error(err, "failed to create tunnel", "localPeer", LocalPeerID)
		return
	}

	// Connect to peer using a shared peer ID (both sides use same ID)
	netConn, err := conn.Connect(ctxHandshake, tunnel, []string{WGLocalIfaceAddrCIDR}, RemotePeerID)
	if err != nil {
		logger.Error(err, "failed to connect to peer", "localPeer", LocalPeerID, "remotePeerID", RemotePeerID)
		return
	}

	// todo(): think about where to put the cancel of the tunnel itself
	defer tunnel.Stop(context.Background())
	defer netConn.Close()

	logger.Infof("Tunnel has been stablished! Press Ctrl+C to exit.")

	// ...
	// Start TCP server 
	tcpServer, err := common.NewTCPServer(WGLocalIfaceAddr, TCPServerPort, logger)
	if err != nil {
		logger.Error(err, "failed to create TCP server", "address", WGLocalIfaceAddr)
		return
	}
}
```

## Quickstart
Start `peer-hub` server: 
```Bash
$ 
```

Start peer A node: 
```Bash
$ go run cmd/peerA/peer-a.go 
```

Start peer B node: 
```Bash
$ go run cmd/peerB/peer-b.go 
```

## Detecting NAT type
```bash
$ sudo apt install stun-client
$ stun stun.l.google.com:19302
STUN client version 0.97
Primary: Independent Mapping, Independent Filter, preserves ports, will hairpin
Return value is 0x000003
```
