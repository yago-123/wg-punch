package store

import "wg-punch/pkg/peer"

type Store interface {
	Register(peerID string, info peer.PeerInfo) error
	Lookup(peerID string) (peer.PeerInfo, bool)
}
