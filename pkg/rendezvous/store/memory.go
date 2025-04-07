package store

import (
	"github.com/yago-123/wg-punch/pkg/peer"
	"sync"
)

type MemoryStore struct {
	mu    sync.RWMutex
	peers map[string]peer.Info
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		peers: make(map[string]peer.Info),
	}
}

func (s *MemoryStore) Register(peerID string, info peer.Info) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.peers[peerID] = info
	return nil
}

func (s *MemoryStore) Lookup(peerID string) (peer.Info, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	info, ok := s.peers[peerID]
	return info, ok
}
