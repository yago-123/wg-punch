![Alt text](https://github.com/user-attachments/assets/7225f4e0-e949-470a-b07f-a9c9a4a81530)

## About
`wg-punch` is a library for seamless UDP hole punching using WireGuard, enabling **secure NAT traversal**. It operates with a 
userspace TCP/IP stack, facilitating peer-to-peer communication by punching through NATs and firewalls over UDP, while 
WireGuard establishes encrypted tunnels and overlay networks for private, secure connections.

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

	ctx, cancel := context.WithTimeout(context.Background(), TunnelHandshakeTimeout)
	defer cancel()

	// Configure the tunnel
	tunnelCfg := &tunnel.Config{
		PrivKey:           WGPrivKey,
		Iface:             WGLocalIfaceName,
		IfaceIPv4CIDR:     WGLocalIfaceAddrCIDR,
		ListenPort:        WGLocalListenPort,
		ReplacePeer:       true,
		CreateIface:       true,
		KeepAliveInterval: WGKeepAliveInterval,
	}

	remoteIP, remoteIPCIDR, err := net.ParseCIDR(WGRemoteIfaceAddrCIDR)
	if err != nil {
		logger.Errorf("failed to parse CIDR: %v", err)
		return
	}

	// Create the remote peer
	remotePeer := peer.Info{
		PublicKey: WGRemotePubKey,
		Endpoint: &net.UDPAddr{
			IP:   net.ParseIP(WGRemotePubEndpointIP),
			Port: WGRemoteListenPort,
		},
		AllowedIPs: []net.IPNet{
			{
				IP:   remoteIP,
				Mask: remoteIPCIDR.Mask,
			},
		},
	}

	tunnel, err := kernelwg.NewTunnel(tunnelCfg)
	if err != nil {
		logger.Errorf("failed to create tunnel: %v", err)
		return
	}

	if errStart := tunnel.Start(ctx, nil, remotePeer); errStart != nil {
		logger.Errorf("failed to start tunnel: %v", errStart)
		return
	}
	defer tunnel.Stop()

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
