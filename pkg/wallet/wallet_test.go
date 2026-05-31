package wallet

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/goldh0rse/trellis/pkg/pqsig"
)

func newTestWallet(tb testing.TB) *Wallet {
	tb.Helper()
	w, err := NewWallet()
	if err != nil {
		tb.Fatalf("NewWallet: %v", err)
	}
	return w
}

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

func TestWalletSign(t *testing.T) {
	w := newTestWallet(t)
	msg := []byte("transfer 30 to bob")

	sig, err := w.Sign(msg)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	// A wallet's signature must verify against its own public key.
	if err := pqsig.Verify(w.PublicKey(), msg, sig); err != nil {
		t.Fatalf("signature should verify against the wallet's public key, got: %v", err)
	}
}

// BenchmarkNewWallet measures wallet creation (an ML-DSA-44 keygen plus wrapping).
func BenchmarkNewWallet(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		if _, err := NewWallet(); err != nil {
			b.Fatalf("NewWallet: %v", err)
		}
	}
}
