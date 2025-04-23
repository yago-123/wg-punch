# wg-punch
![Alt text](https://github.com/user-attachments/assets/7225f4e0-e949-470a-b07f-a9c9a4a81530)

## Detecting NAT type
```bash
$ sudo apt install stun-client
$ stun stun.l.google.com:19302
STUN client version 0.97
Primary: Independent Mapping, Independent Filter, preserves ports, will hairpin
Return value is 0x000003
```

## Sample usage (in progress)
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
    
    // todo(): this should go away
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

	tunnelCfg := &wg.TunnelConfig{
		PrivKey:           WGLocalPrivKey,
		Iface:             WGLocalIfaceName,
		IfaceIPv4CIDR:     WGLocalIfaceAddrCIDR,
		ListenPort:        WGLocalListenPort,
		ReplacePeer:       true,
		CreateIface:       true,
		KeepAliveInterval: WGKeepAliveInterval,
	}

	// Initialize WireGuard interface using WireGuard
	tunnel, err := kernelwg.NewTunnel(tunnelCfg)
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

	// ...
	// Start TCP server
	tcpServer, err := common.NewTCPServer(WGLocalIfaceAddr, TCPServerPort, logger)
    if err != nil {
        logger.Error(err, "failed to create TCP server", "address", WGLocalIfaceAddr)
        return
    }
}
```

# Quickstart 
