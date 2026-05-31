package blockchain

import (
	"bytes"
	"errors"
	"fmt"
)

// Blockchain is the ordered chain of blocks. Phase 1 keeps it in memory; Phase 3
// swaps the slice for a bbolt-backed store. Difficulty is the Proof-of-Work
// target every block (including genesis) must meet.
type Blockchain struct {
	Blocks     []*Block
	Difficulty int
}

// NewBlockchain creates a chain seeded with a mined genesis block whose single
// coinbase transaction issues `reward` coins to `to`. The genesis block has an
// empty PrevHash and is mined to `difficulty`, like every other block.
func NewBlockchain(to []byte, reward uint64, difficulty int) *Blockchain {
	coinbase := NewCoinbaseTx(to, reward)
	genesis := NewBlock([]*Transaction{coinbase}, nil)
	genesis.Mine(difficulty)
	return &Blockchain{Blocks: []*Block{genesis}, Difficulty: difficulty}
}

// AddBlock links a new block of transactions onto the current tip, mines it to
// the chain's difficulty, and appends it. It returns the new block and the
// number of hashing attempts mining took. Phase 1 balance checks are still out
// of scope (Phase 4).
func (bc *Blockchain) AddBlock(txs []*Transaction) (*Block, uint64, error) {
	if len(bc.Blocks) == 0 {
		return nil, 0, errors.New("cannot add block to empty chain")
	}
	tip := bc.Blocks[len(bc.Blocks)-1]
	block := NewBlock(txs, tip.Hash)
	attempts := block.Mine(bc.Difficulty)
	bc.Blocks = append(bc.Blocks, block)
	return block, attempts, nil
}

// IsValid verifies the whole chain, trusting nothing. For each block it checks,
// in order:
//  1. hash integrity   — the stored Hash equals a freshly recomputed ComputeHash
//     (detects tampered block data, including a changed Nonce);
//  2. proof of work     — the hash meets the chain's difficulty (detects a Nonce
//     swapped in with a matching but unmined hash);
//  3. link integrity    — genesis has an empty PrevHash; every other block's
//     PrevHash equals the previous block's Hash;
//  4. transaction validity — every transaction verifies (coinbase must be
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

		// 2. Proof of work.
		if !meetsDifficulty(b.Hash, bc.Difficulty) {
			return fmt.Errorf("block %d: hash does not meet difficulty %d", i, bc.Difficulty)
		}

		// 3. Link integrity.
		if i == 0 {
			if len(b.PrevHash) != 0 {
				return errors.New("genesis block must have empty PrevHash")
			}
		} else if !bytes.Equal(b.PrevHash, bc.Blocks[i-1].Hash) {
			return fmt.Errorf("block %d: PrevHash does not match previous block hash", i)
		}

		// 4. Transaction validity.
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
