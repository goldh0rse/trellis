package blockchain

import (
	"bytes"
	"encoding/hex"
	"strings"
	"testing"
)

func TestComputeHashDeterministic(t *testing.T) {
	alice := newTestWallet(t)
	bobby := newTestWallet(t)

	tx := NewTransaction(alice.PublicKey(), bobby.PublicKey(), 7)
	if err := tx.Sign(alice); err != nil {
		t.Fatalf("Sign: %v", err)
	}

	// Construct a block with fixed inputs (fixed Timestamp, fixed txs) so the
	// hash depends only on stored fields, not the wall clock.
	b := &Block{
		Timestamp:    1_700_000_000,
		Transactions: []*Transaction{tx},
		PrevHash:     []byte{0xde, 0xad, 0xbe, 0xef},
	}

	first := b.ComputeHash()
	second := b.ComputeHash()
	if !bytes.Equal(first, second) {
		t.Fatalf("ComputeHash not deterministic: %x != %x", first, second)
	}
}

func TestMineMeetsDifficulty(t *testing.T) {
	const difficulty = 3

	b := &Block{
		Timestamp:    1_700_000_000,
		Transactions: nil,
		PrevHash:     []byte{0xde, 0xad, 0xbe, 0xef},
	}

	attempts := b.Mine(difficulty)
	if attempts == 0 {
		t.Fatal("Mine should report at least one attempt")
	}

	// The mined hash must start with `difficulty` zero hex digits...
	if prefix := strings.Repeat("0", difficulty); !strings.HasPrefix(hex.EncodeToString(b.Hash), prefix) {
		t.Fatalf("mined hash %s does not start with %q", hex.EncodeToString(b.Hash), prefix)
	}
	// ...and Hash must be consistent with the stored Nonce.
	if !bytes.Equal(b.Hash, b.ComputeHash()) {
		t.Fatal("mined Hash does not match ComputeHash for the stored Nonce")
	}
}

// BenchmarkComputeHash measures SHA-256 block hashing over a single-transaction
// block — the cheap structural check IsValid runs before signature verification.
func BenchmarkComputeHash(b *testing.B) {
	alice := newTestWallet(b)
	bobby := newTestWallet(b)
	tx := NewTransaction(alice.PublicKey(), bobby.PublicKey(), 7)
	if err := tx.Sign(alice); err != nil {
		b.Fatalf("Sign: %v", err)
	}
	blk := &Block{
		Timestamp:    1_700_000_000,
		Transactions: []*Transaction{tx},
		PrevHash:     []byte{0xde, 0xad, 0xbe, 0xef},
	}

	b.ReportAllocs()
	for b.Loop() {
		_ = blk.ComputeHash()
	}
}
