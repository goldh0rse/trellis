package ledger

import "errors"

// ErrBlockNotFound is returned by a Store when a requested block hash is absent.
// IsValid and the Iterator treat it as a broken link (a dangling PrevHash).
var ErrBlockNotFound = errors.New("block not found")

// Store is the ledger's persistence boundary. The ledger defines it (consumer
// side) and depends only on it; concrete implementations live elsewhere (e.g.
// pkg/storage for bbolt), keeping storage-engine details out of the domain.
//
// A Store keys blocks by their Hash and tracks a single "tip" — the hash of the
// most recently appended block.
type Store interface {
	// Tip returns the current tip hash, or an empty slice (with nil error) when
	// the store has no blocks yet.
	Tip() ([]byte, error)

	// GetBlock returns the block stored under hash, or ErrBlockNotFound if none.
	GetBlock(hash []byte) (*Block, error)

	// AppendBlock stores b under b.Hash and advances the tip to b.Hash. The two
	// writes must be atomic so a crash can never leave the tip pointing at a
	// block that was not stored.
	AppendBlock(b *Block) error
}
