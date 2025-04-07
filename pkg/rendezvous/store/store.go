package store

import "wg-punch/pkg/peer"

type Store interface {
	Register(peerID string, info peer.Info) error
	Lookup(peerID string) (peer.Info, bool)
}
