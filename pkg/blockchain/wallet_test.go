package blockchain

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func TestWalletAddress(t *testing.T) {
	w := newTestWallet(t)
	addr := w.Address()

	// Address is the first 16 hex chars of SHA-256(publicKey).
	sum := sha256.Sum256(w.PublicKey())
	want := hex.EncodeToString(sum[:])[:16]
	if addr != want {
		t.Fatalf("Address() = %q, want %q", addr, want)
	}
	if len(addr) != 16 {
		t.Fatalf("Address() length = %d, want 16", len(addr))
	}

	// Deterministic for the same wallet.
	if addr2 := w.Address(); addr2 != addr {
		t.Fatalf("Address() not deterministic: %q != %q", addr, addr2)
	}

	// Different wallets yield different addresses.
	if other := newTestWallet(t).Address(); other == addr {
		t.Fatal("two distinct wallets produced the same address")
	}
}

// BenchmarkNewWallet measures ML-DSA-44 key generation — the most expensive
// wallet operation, and notably heavier than classical (e.g. ECDSA) keygen.
func BenchmarkNewWallet(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		if _, err := NewWallet(); err != nil {
			b.Fatalf("NewWallet: %v", err)
		}
	}
}
