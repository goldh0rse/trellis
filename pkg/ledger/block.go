package ledger

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"strings"
	"time"
)

// Block is a hash-linked container of transactions. Its hash is only considered
// valid once the block has been mined: Nonce is varied until the hash meets the
// chain's difficulty (Proof of Work).
type Block struct {
	Timestamp    int64          // Unix seconds, captured once at construction
	Transactions []*Transaction // pointers: txs are large (~2.4 KB signatures)
	PrevHash     []byte         // SHA-256 of the previous block; empty for genesis
	Nonce        uint64         // Proof-of-Work counter, set by Mine
	Hash         []byte         // SHA-256 of this block's header (see ComputeHash)
}

// txDigest folds the transaction set into a single SHA-256 digest by hashing the
// concatenation of every transaction ID in slice order. Each ID already commits
// to From/To/Amount, so this digest commits to both the contents and ordering of
// the block's transactions. IDs are fixed 32-byte values, so no length prefix is
// needed to keep the stream unambiguous.
func (b *Block) txDigest() []byte {
	h := sha256.New()
	for _, tx := range b.Transactions {
		h.Write(tx.ID)
	}
	return h.Sum(nil)
}

// ComputeHash returns the SHA-256 of the block header:
//
//	uint64(Timestamp, BE) || PrevHash || txDigest() || uint64(Nonce, BE)
//
// It is a pure function of the stored fields (it never reads its own Hash, nor
// the wall clock), so IsValid can recompute and compare it. Nonce is part of the
// hashed bytes so that varying it during mining changes the hash. Genesis
// contributes zero bytes for the empty PrevHash.
func (b *Block) ComputeHash() []byte {
	var ts [8]byte
	binary.BigEndian.PutUint64(ts[:], uint64(b.Timestamp))

	var nonce [8]byte
	binary.BigEndian.PutUint64(nonce[:], b.Nonce)

	h := sha256.New()
	h.Write(ts[:])
	h.Write(b.PrevHash)
	h.Write(b.txDigest())
	h.Write(nonce[:])
	return h.Sum(nil)
}

// NewBlock builds a block over txs linked to prevHash and stamps the time once.
// The block is unmined: Nonce is 0 and Hash reflects that. Call Mine to find a
// nonce that satisfies the target difficulty.
func NewBlock(txs []*Transaction, prevHash []byte) *Block {
	b := &Block{
		Timestamp:    time.Now().Unix(),
		Transactions: txs,
		PrevHash:     prevHash,
	}
	b.Hash = b.ComputeHash()
	return b
}

// Mine performs Proof of Work: it increments Nonce until the block's hash has at
// least `difficulty` leading zero hex digits, storing the winning hash in Hash.
// It returns the number of hashing attempts made. A difficulty of 0 accepts the
// first hash. Keep difficulty low (3–4) for a learning project so it stays fast.
func (b *Block) Mine(difficulty int) uint64 {
	target := strings.Repeat("0", difficulty)
	var attempts uint64
	for {
		attempts++
		b.Hash = b.ComputeHash()
		if strings.HasPrefix(hex.EncodeToString(b.Hash), target) {
			return attempts
		}
		b.Nonce++
	}
}

// MeetsDifficulty reports whether a hash satisfies the difficulty target, i.e.
// its hex encoding begins with `difficulty` zeros. Exported so the networking
// layer can validate the Proof of Work of blocks received from peers.
func MeetsDifficulty(hash []byte, difficulty int) bool {
	return strings.HasPrefix(hex.EncodeToString(hash), strings.Repeat("0", difficulty))
}
