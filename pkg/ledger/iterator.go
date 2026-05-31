package ledger

import "fmt"

// Iterator walks a chain from the tip back to genesis, following each block's
// PrevHash. It is the read path over a Store.
type Iterator struct {
	store Store
	next  []byte // hash of the block to return on the next call; empty when done
}

// Iterator returns an iterator positioned at the chain's tip.
func (c *Chain) Iterator() (*Iterator, error) {
	tip, err := c.store.Tip()
	if err != nil {
		return nil, err
	}
	return &Iterator{store: c.store, next: tip}, nil
}

// Next returns the next block (tip first, genesis last) and advances toward
// genesis. It returns (nil, nil) once genesis has been passed (a clean end). A
// non-nil error means the store is corrupt — e.g. a PrevHash with no stored block
// (ErrBlockNotFound).
func (it *Iterator) Next() (*Block, error) {
	if len(it.next) == 0 {
		return nil, nil
	}
	b, err := it.store.GetBlock(it.next)
	if err != nil {
		return nil, fmt.Errorf("iterator: %w", err)
	}
	it.next = b.PrevHash
	return b, nil
}
