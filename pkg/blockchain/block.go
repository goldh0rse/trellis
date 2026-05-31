package blockchain

import (
	"crypto/sha256"
	"encoding/binary"
	"time"
)

// Block is a hash-linked container of transactions. Phase 1 has no Nonce or
// difficulty — Proof of Work is added in Phase 2.
type Block struct {
	Timestamp    int64          // Unix seconds, captured once at construction
	Transactions []*Transaction // pointers: txs are large (~2.4 KB signatures)
	PrevHash     []byte         // SHA-256 of the previous block; empty for genesis
	Hash         []byte         // SHA-256 of this block's header (see ComputeHash)
}

// txDigest folds the transaction set into a single SHA-256 digest by hashing
// the concatenation of every transaction ID in slice order. Each ID already
// commits to From/To/Amount, so this digest commits to both the contents and
// the ordering of the block's transactions. IDs are fixed 32-byte values, so no
// length prefix is needed to keep the stream unambiguous.
func (b *Block) txDigest() []byte {
	h := sha256.New()
	for _, tx := range b.Transactions {
		h.Write(tx.ID)
	}
	return h.Sum(nil)
}

// ComputeHash returns the SHA-256 of the block header:
//
//	uint64(Timestamp, big-endian) || PrevHash || txDigest()
//
// It is a pure function of the stored fields (it never reads its own Hash, nor
// the wall clock), so IsValid can recompute and compare it. Genesis contributes
// zero bytes for the empty PrevHash. Phase 2 will fold Nonce in here.
func (b *Block) ComputeHash() []byte {
	var ts [8]byte
	binary.BigEndian.PutUint64(ts[:], uint64(b.Timestamp))

	h := sha256.New()
	h.Write(ts[:])
	h.Write(b.PrevHash)
	h.Write(b.txDigest())
	return h.Sum(nil)
}

// NewBlock builds a block over txs linked to prevHash, stamps the time once,
// and computes its hash.
func NewBlock(txs []*Transaction, prevHash []byte) *Block {
	b := &Block{
		Timestamp:    time.Now().Unix(),
		Transactions: txs,
		PrevHash:     prevHash,
	}
	b.Hash = b.ComputeHash()
	return b
}
