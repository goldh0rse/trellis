package ledger_test

import (
	"encoding/hex"

	"github.com/goldh0rse/trellis/pkg/ledger"
)

// memStore is an in-memory ledger.Store fake for tests — no disk required. It
// stores block pointers, so a test can fetch a block and mutate it in place to
// simulate tampering, exactly as the old in-memory chain tests did.
type memStore struct {
	blocks map[string]*ledger.Block
	tip    []byte
}

func newMemStore() *memStore {
	return &memStore{blocks: make(map[string]*ledger.Block)}
}

func (m *memStore) Tip() ([]byte, error) { return m.tip, nil }

func (m *memStore) GetBlock(hash []byte) (*ledger.Block, error) {
	b, ok := m.blocks[hex.EncodeToString(hash)]
	if !ok {
		return nil, ledger.ErrBlockNotFound
	}
	return b, nil
}

func (m *memStore) AppendBlock(b *ledger.Block) error {
	m.blocks[hex.EncodeToString(b.Hash)] = b
	m.tip = b.Hash
	return nil
}

// tipBlock returns the current tip block — convenience for tamper tests.
func (m *memStore) tipBlock() *ledger.Block {
	return m.blocks[hex.EncodeToString(m.tip)]
}
