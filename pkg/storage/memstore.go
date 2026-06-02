package storage

import (
	"encoding/hex"
	"sync"

	"github.com/goldh0rse/trellis/pkg/ledger"
)

// MemStore is an in-memory ledger.Store. It is handy for tests and for ephemeral
// nodes that do not need durability. It is safe for concurrent use.
type MemStore struct {
	mu     sync.Mutex
	blocks map[string]*ledger.Block
	tip    []byte
}

// NewMemStore returns an empty in-memory store.
func NewMemStore() *MemStore {
	return &MemStore{blocks: make(map[string]*ledger.Block)}
}

// Tip returns the current tip hash, or an empty slice when the store is empty.
func (m *MemStore) Tip() ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.tip, nil
}

// GetBlock returns the block stored under hash, or ledger.ErrBlockNotFound.
func (m *MemStore) GetBlock(hash []byte) (*ledger.Block, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	b, ok := m.blocks[hex.EncodeToString(hash)]
	if !ok {
		return nil, ledger.ErrBlockNotFound
	}
	return b, nil
}

// AppendBlock stores b under b.Hash and advances the tip.
func (m *MemStore) AppendBlock(b *ledger.Block) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.blocks[hex.EncodeToString(b.Hash)] = b
	m.tip = b.Hash
	return nil
}
