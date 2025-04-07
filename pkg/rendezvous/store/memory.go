package store

import (
	"sync"
	"wg-punch/pkg/peer"
)

type MemoryStore struct {
	mu    sync.RWMutex
	peers map[string]peer.PeerInfo
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		peers: make(map[string]peer.PeerInfo),
	}
}

func (s *MemoryStore) Register(peerID string, info peer.PeerInfo) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.peers[peerID] = info
	return nil
}

func (s *MemoryStore) Lookup(peerID string) (peer.PeerInfo, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	info, ok := s.peers[peerID]
	return info, ok
}
