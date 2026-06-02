// Package storage persists the ledger to disk. It is the only package that
// imports go.etcd.io/bbolt and encoding/gob, keeping those storage-engine and
// serialization details out of the domain (mirroring how pkg/pqsig isolates the
// ML-DSA dependency). Bolt implements ledger.Store.
package storage

import (
	"bytes"
	"encoding/gob"
	"time"

	bolt "go.etcd.io/bbolt"

	"github.com/goldh0rse/trellis/pkg/ledger"
)

var (
	blocksBucket = []byte("blocks") // block hash -> gob-encoded *ledger.Block
	metaBucket   = []byte("meta")   // misc keys; currently just "tip"
	tipKey       = []byte("tip")    // -> hash of the most recent block
)

// Bolt is a bbolt-backed ledger.Store.
type Bolt struct {
	db *bolt.DB
}

// NewBolt opens (creating if needed) a bbolt database at path and ensures the
// required buckets exist. The caller owns the lifecycle and must Close it.
//
// bbolt holds an exclusive lock on the file, so a second process (e.g. a CLI
// query against a running node's database) fails fast with a timeout rather than
// blocking forever.
func NewBolt(path string) (*Bolt, error) {
	db, err := bolt.Open(path, 0600, &bolt.Options{Timeout: 2 * time.Second})
	if err != nil {
		return nil, err
	}
	err = db.Update(func(tx *bolt.Tx) error {
		if _, err := tx.CreateBucketIfNotExists(blocksBucket); err != nil {
			return err
		}
		_, err := tx.CreateBucketIfNotExists(metaBucket)
		return err
	})
	if err != nil {
		db.Close()
		return nil, err
	}
	return &Bolt{db: db}, nil
}

// Close releases the underlying database.
func (s *Bolt) Close() error {
	return s.db.Close()
}

// Tip returns the current tip hash, or an empty slice when the store is empty.
func (s *Bolt) Tip() ([]byte, error) {
	var tip []byte
	err := s.db.View(func(tx *bolt.Tx) error {
		v := tx.Bucket(metaBucket).Get(tipKey)
		if v != nil {
			// bbolt values are only valid within the transaction — copy out.
			tip = append([]byte(nil), v...)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return tip, nil
}

// GetBlock returns the block stored under hash, or ledger.ErrBlockNotFound.
func (s *Bolt) GetBlock(hash []byte) (*ledger.Block, error) {
	var block *ledger.Block
	err := s.db.View(func(tx *bolt.Tx) error {
		v := tx.Bucket(blocksBucket).Get(hash)
		if v == nil {
			return ledger.ErrBlockNotFound
		}
		// Decode inside the transaction: gob copies into the struct, so the
		// result is safe to use after the mmap'd bytes go away.
		b, err := decodeBlock(v)
		if err != nil {
			return err
		}
		block = b
		return nil
	})
	if err != nil {
		return nil, err
	}
	return block, nil
}

// AppendBlock stores b under b.Hash and advances the tip, atomically.
func (s *Bolt) AppendBlock(b *ledger.Block) error {
	data, err := encodeBlock(b)
	if err != nil {
		return err
	}
	return s.db.Update(func(tx *bolt.Tx) error {
		if err := tx.Bucket(blocksBucket).Put(b.Hash, data); err != nil {
			return err
		}
		return tx.Bucket(metaBucket).Put(tipKey, b.Hash)
	})
}

// encodeBlock gob-encodes a block. No gob.Register is needed: Block and
// Transaction contain only concrete exported fields (no interfaces).
func encodeBlock(b *ledger.Block) ([]byte, error) {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(b); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func decodeBlock(data []byte) (*ledger.Block, error) {
	var b ledger.Block
	if err := gob.NewDecoder(bytes.NewReader(data)).Decode(&b); err != nil {
		return nil, err
	}
	return &b, nil
}
