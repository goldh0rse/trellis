package ledger

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
)

// Chain is the blockchain, backed by a Store. It holds no blocks in memory; it
// reads and writes them through the Store, so the chain survives restarts.
// Difficulty is the Proof-of-Work target every block (including genesis) must meet.
type Chain struct {
	store      Store
	Difficulty int
}

// NewChain returns a chain backed by store. If the store is empty it mines a
// genesis block whose single coinbase issues `reward` coins to `to`. If the store
// already holds a chain, it is loaded as-is (genesis is NOT re-created).
func NewChain(store Store, to []byte, reward uint64, difficulty int) (*Chain, error) {
	tip, err := store.Tip()
	if err != nil {
		return nil, err
	}
	c := &Chain{store: store, Difficulty: difficulty}
	if len(tip) == 0 {
		genesis := NewBlock([]*Transaction{NewCoinbaseTx(to, reward)}, nil)
		genesis.Mine(difficulty)
		if err := store.AppendBlock(genesis); err != nil {
			return nil, err
		}
	}
	return c, nil
}

// LoadChain returns a chain over an existing or empty store WITHOUT seeding a
// genesis. Networked nodes use it: a fresh node starts empty and learns the
// genesis block from a peer (rather than minting a divergent one of its own).
func LoadChain(store Store, difficulty int) (*Chain, error) {
	return &Chain{store: store, Difficulty: difficulty}, nil
}

// Block returns the stored block with the given hash, or ErrBlockNotFound. It
// lets callers (e.g. the networking layer) read blocks without touching the Store.
func (c *Chain) Block(hash []byte) (*Block, error) {
	return c.store.GetBlock(hash)
}

// BlockHashes returns every block hash in genesis→tip order. The networking
// layer advertises and downloads blocks in this order so each one extends the
// previous when applied.
func (c *Chain) BlockHashes() ([][]byte, error) {
	it, err := c.Iterator()
	if err != nil {
		return nil, err
	}
	var tipToGenesis [][]byte
	for {
		b, err := it.Next()
		if err != nil {
			return nil, err
		}
		if b == nil {
			break
		}
		tipToGenesis = append(tipToGenesis, b.Hash)
	}
	// Reverse into genesis→tip order.
	hashes := make([][]byte, len(tipToGenesis))
	for i, h := range tipToGenesis {
		hashes[len(tipToGenesis)-1-i] = h
	}
	return hashes, nil
}

// AddBlock links a new block of transactions onto the current tip, mines it to
// the chain's difficulty, and persists it. It returns the new block and the
// number of hashing attempts mining took.
func (c *Chain) AddBlock(txs []*Transaction) (*Block, uint64, error) {
	tip, err := c.store.Tip()
	if err != nil {
		return nil, 0, err
	}
	if len(tip) == 0 {
		return nil, 0, errors.New("cannot add block: chain has no genesis")
	}
	block := NewBlock(txs, tip)
	attempts := block.Mine(c.Difficulty)
	if err := c.store.AppendBlock(block); err != nil {
		return nil, 0, err
	}
	return block, attempts, nil
}

// Height returns the number of blocks in the chain (genesis included).
func (c *Chain) Height() (int, error) {
	it, err := c.Iterator()
	if err != nil {
		return 0, err
	}
	n := 0
	for {
		b, err := it.Next()
		if err != nil {
			return 0, err
		}
		if b == nil {
			break
		}
		n++
	}
	return n, nil
}

// Tip returns the hash of the most recent block (empty if the chain is empty).
func (c *Chain) Tip() ([]byte, error) {
	return c.store.Tip()
}

// IsValid verifies the whole chain by walking from the tip back to genesis,
// trusting nothing. For each block, in order:
//  1. reachability — the block's Hash equals the link target we followed to reach
//     it (the tip for the first block, then each child's PrevHash);
//  2. cycle guard  — a hash seen twice means a corrupted store;
//  3. hash integrity — Hash equals a freshly recomputed ComputeHash (this also
//     subsumes the old "PrevHash matches the previous block" check, since PrevHash
//     and Nonce are part of ComputeHash);
//  4. proof of work — the hash meets the chain's difficulty;
//  5. transaction validity — every tx verifies, and coinbase transactions appear
//     only in genesis (the block whose PrevHash is empty).
//
// The walk must terminate at a genesis block; a dangling PrevHash surfaces as
// ErrBlockNotFound from the iterator. Returns the first failure, or nil if valid.
func (c *Chain) IsValid() error {
	tip, err := c.store.Tip()
	if err != nil {
		return err
	}
	if len(tip) == 0 {
		return errors.New("empty chain")
	}

	it, err := c.Iterator()
	if err != nil {
		return err
	}

	expected := tip
	seen := make(map[string]bool)
	sawGenesis := false

	for {
		b, err := it.Next()
		if err != nil {
			return err // includes ErrBlockNotFound: a broken/dangling link
		}
		if b == nil {
			break
		}

		// 1. Reachability: the fetched block must actually be the one we linked to.
		if !bytes.Equal(b.Hash, expected) {
			return fmt.Errorf("block %s: stored hash does not match link target", Short(b.Hash))
		}

		// 2. Cycle guard.
		key := hex.EncodeToString(b.Hash)
		if seen[key] {
			return fmt.Errorf("cycle detected at block %s", Short(b.Hash))
		}
		seen[key] = true

		// 3. Hash integrity.
		if !bytes.Equal(b.Hash, b.ComputeHash()) {
			return fmt.Errorf("block %s: hash mismatch (data tampered)", Short(b.Hash))
		}

		// 4. Proof of work.
		if !MeetsDifficulty(b.Hash, c.Difficulty) {
			return fmt.Errorf("block %s: hash does not meet difficulty %d", Short(b.Hash), c.Difficulty)
		}

		// 5. Transaction validity.
		isGenesis := len(b.PrevHash) == 0
		for j, tx := range b.Transactions {
			if err := tx.Verify(); err != nil {
				return fmt.Errorf("block %s tx %d: %w", Short(b.Hash), j, err)
			}
			if tx.IsCoinbase() && !isGenesis {
				return fmt.Errorf("block %s tx %d: unexpected coinbase outside genesis", Short(b.Hash), j)
			}
		}

		if isGenesis {
			sawGenesis = true
		}
		expected = b.PrevHash
	}

	if !sawGenesis {
		return errors.New("chain does not terminate at a genesis block")
	}
	return nil
}
