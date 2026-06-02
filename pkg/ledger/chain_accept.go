package ledger

import (
	"bytes"
	"errors"
	"fmt"
)

// ErrNotExtending is returned by AcceptBlock when a block does not build directly
// on the current tip. Under sync-by-extension this is normal (e.g. a competing
// block at the same height) and the caller should drop it, not treat it as an
// error condition.
var ErrNotExtending = errors.New("block does not extend current tip")

// AcceptBlock validates a pre-mined block received from a peer and, if valid,
// appends it. Unlike AddBlock it never mines — the peer already did the work.
//
// Sync-by-extension consensus: a block is accepted only if it is a valid genesis
// on an empty store, or it strictly extends the current tip. The chain only ever
// grows by one; it is never replaced, so a shorter or non-extending chain can
// never win. Every block is fully validated (hash, Proof of Work, transactions)
// before it is stored.
func (c *Chain) AcceptBlock(b *Block) error {
	if b == nil || len(b.Hash) == 0 {
		return errors.New("nil or unhashed block")
	}

	// Hash integrity (also covers PrevHash/Nonce, which feed ComputeHash).
	if !bytes.Equal(b.Hash, b.ComputeHash()) {
		return fmt.Errorf("block %s: hash mismatch", Short(b.Hash))
	}
	// Proof of work.
	if !MeetsDifficulty(b.Hash, c.Difficulty) {
		return fmt.Errorf("block %s: does not meet difficulty %d", Short(b.Hash), c.Difficulty)
	}

	tip, err := c.store.Tip()
	if err != nil {
		return err
	}

	if len(tip) == 0 {
		// Genesis: must link to nothing.
		if len(b.PrevHash) != 0 {
			return errors.New("first block must be a genesis (empty PrevHash)")
		}
		if err := verifyTxs(b, true); err != nil {
			return err
		}
		return c.store.AppendBlock(b)
	}

	// Extension: must build directly on the current tip.
	if len(b.PrevHash) == 0 {
		return errors.New("unexpected genesis on a non-empty chain")
	}
	if !bytes.Equal(b.PrevHash, tip) {
		return ErrNotExtending
	}
	if err := verifyTxs(b, false); err != nil {
		return err
	}
	return c.store.AppendBlock(b)
}

// verifyTxs verifies every transaction in a block. Coinbase transactions are
// permitted only in genesis.
func verifyTxs(b *Block, isGenesis bool) error {
	for j, tx := range b.Transactions {
		if err := tx.Verify(); err != nil {
			return fmt.Errorf("block %s tx %d: %w", Short(b.Hash), j, err)
		}
		if tx.IsCoinbase() && !isGenesis {
			return fmt.Errorf("block %s tx %d: unexpected coinbase outside genesis", Short(b.Hash), j)
		}
	}
	return nil
}
