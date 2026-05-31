package blockchain

import (
	"fmt"
	"testing"
)

// buildChain returns a chain with genesis (100 coins to alice) plus one block
// holding a signed alice -> bobby : 30 transfer.
func buildChain(t testing.TB) (*Blockchain, *Wallet, *Wallet) {
	t.Helper()
	alice := newTestWallet(t)
	bobby := newTestWallet(t)

	chain := NewBlockchain(alice.PublicKey(), 100)

	tx := NewTransaction(alice.PublicKey(), bobby.PublicKey(), 30)
	if err := tx.Sign(alice); err != nil {
		t.Fatalf("Sign: %v", err)
	}
	if _, err := chain.AddBlock([]*Transaction{tx}); err != nil {
		t.Fatalf("AddBlock: %v", err)
	}
	return chain, alice, bobby
}

func TestChainHappyPathIsValid(t *testing.T) {
	chain, _, _ := buildChain(t)
	if err := chain.IsValid(); err != nil {
		t.Fatalf("freshly built chain should be valid, got: %v", err)
	}
}

func TestChainTamperedAmountIsInvalid(t *testing.T) {
	chain, _, _ := buildChain(t)

	chain.Blocks[1].Transactions[0].Amount = 31 // forge the transfer

	if err := chain.IsValid(); err == nil {
		t.Fatal("chain with a tampered amount should be invalid, got nil")
	}
}

func TestChainBrokenLinkIsInvalid(t *testing.T) {
	chain, _, _ := buildChain(t)

	chain.Blocks[1].PrevHash = []byte("not the genesis hash")

	if err := chain.IsValid(); err == nil {
		t.Fatal("chain with a broken PrevHash link should be invalid, got nil")
	}
}

// BenchmarkIsValid measures full-chain validation across several chain lengths.
// Validation cost is dominated by the per-transaction ML-DSA verification, so it
// scales roughly linearly with the number of blocks.
func BenchmarkIsValid(b *testing.B) {
	for _, n := range []int{1, 10, 100} {
		b.Run(fmt.Sprintf("blocks=%d", n), func(b *testing.B) {
			alice := newTestWallet(b)
			bobby := newTestWallet(b)
			chain := NewBlockchain(alice.PublicKey(), 1_000_000)
			for range n {
				tx := NewTransaction(alice.PublicKey(), bobby.PublicKey(), 1)
				if err := tx.Sign(alice); err != nil {
					b.Fatalf("Sign: %v", err)
				}
				if _, err := chain.AddBlock([]*Transaction{tx}); err != nil {
					b.Fatalf("AddBlock: %v", err)
				}
			}

			b.ReportAllocs()
			for b.Loop() {
				if err := chain.IsValid(); err != nil {
					b.Fatalf("IsValid: %v", err)
				}
			}
		})
	}
}
