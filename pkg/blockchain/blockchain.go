package blockchain

import (
	"bytes"
	"errors"
	"fmt"
)

// Blockchain is the ordered chain of blocks. Phase 1 keeps it in memory; Phase 3
// swaps the slice for a bbolt-backed store.
type Blockchain struct {
	Blocks []*Block
}

// NewBlockchain creates a chain seeded with a genesis block whose single
// coinbase transaction issues `reward` coins to `to`. The genesis block has an
// empty PrevHash.
func NewBlockchain(to []byte, reward uint64) *Blockchain {
	coinbase := NewCoinbaseTx(to, reward)
	genesis := NewBlock([]*Transaction{coinbase}, nil)
	return &Blockchain{Blocks: []*Block{genesis}}
}

// AddBlock links a new block of transactions onto the current tip and appends
// it. Phase 1 does no mining and no balance checks (those arrive in Phases 2
// and 4). It returns the new block for convenience.
func (bc *Blockchain) AddBlock(txs []*Transaction) (*Block, error) {
	if len(bc.Blocks) == 0 {
		return nil, errors.New("cannot add block to empty chain")
	}
	tip := bc.Blocks[len(bc.Blocks)-1]
	block := NewBlock(txs, tip.Hash)
	bc.Blocks = append(bc.Blocks, block)
	return block, nil
}

// IsValid verifies the whole chain, trusting nothing. For each block it checks,
// in order:
//  1. hash integrity  — the stored Hash equals a freshly recomputed ComputeHash
//     (detects tampered block data);
//  2. link integrity  — genesis has an empty PrevHash; every other block's
//     PrevHash equals the previous block's Hash (detects broken/reordered links);
//  3. transaction validity — every transaction verifies (coinbase must be
//     unsigned; others must carry a valid ML-DSA signature), and coinbase
//     transactions may only appear in genesis.
//
// The cheap structural checks run first so a tampered chain short-circuits
// before the expensive signature verification. Returns the first failure as a
// wrapped error, or nil if the entire chain is valid.
func (bc *Blockchain) IsValid() error {
	for i, b := range bc.Blocks {
		// 1. Hash integrity.
		if !bytes.Equal(b.Hash, b.ComputeHash()) {
			return fmt.Errorf("block %d: hash mismatch (data tampered)", i)
		}

		// 2. Link integrity.
		if i == 0 {
			if len(b.PrevHash) != 0 {
				return errors.New("genesis block must have empty PrevHash")
			}
		} else if !bytes.Equal(b.PrevHash, bc.Blocks[i-1].Hash) {
			return fmt.Errorf("block %d: PrevHash does not match previous block hash", i)
		}

		// 3. Transaction validity.
		for j, tx := range b.Transactions {
			if err := tx.Verify(); err != nil {
				return fmt.Errorf("block %d tx %d: %w", i, j, err)
			}
			if tx.IsCoinbase() && i != 0 {
				return fmt.Errorf("block %d tx %d: unexpected coinbase outside genesis", i, j)
			}
		}
	}
	return nil
}
