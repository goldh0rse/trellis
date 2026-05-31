package ledger_test

import (
	"fmt"
	"testing"

	"github.com/goldh0rse/trellis/pkg/ledger"
)

// testDifficulty keeps mining instant in tests while still exercising the PoW
// path (a few hundred attempts at most).
const testDifficulty = 2

// buildChain returns a chain with genesis (100 coins to alice) plus one block
// holding a signed alice -> bobby : 30 transfer.
func buildChain(t testing.TB) *ledger.Chain {
	t.Helper()
	alice := newTestWallet(t)
	bobby := newTestWallet(t)

	chain := ledger.NewChain(alice.PublicKey(), 100, testDifficulty)

	tx := ledger.NewTransaction(alice.PublicKey(), bobby.PublicKey(), 30)
	if err := tx.Sign(alice); err != nil {
		t.Fatalf("Sign: %v", err)
	}
	if _, _, err := chain.AddBlock([]*ledger.Transaction{tx}); err != nil {
		t.Fatalf("AddBlock: %v", err)
	}
	return chain
}

func TestChainHappyPathIsValid(t *testing.T) {
	chain := buildChain(t)
	if err := chain.IsValid(); err != nil {
		t.Fatalf("freshly built chain should be valid, got: %v", err)
	}
}

func TestChainTamperedAmountIsInvalid(t *testing.T) {
	chain := buildChain(t)

	chain.Blocks[1].Transactions[0].Amount = 31 // forge the transfer

	if err := chain.IsValid(); err == nil {
		t.Fatal("chain with a tampered amount should be invalid, got nil")
	}
}

func TestChainBrokenLinkIsInvalid(t *testing.T) {
	chain := buildChain(t)

	chain.Blocks[1].PrevHash = []byte("not the genesis hash")

	if err := chain.IsValid(); err == nil {
		t.Fatal("chain with a broken PrevHash link should be invalid, got nil")
	}
}

func TestAddBlockEmptyChain(t *testing.T) {
	c := &ledger.Chain{} // no genesis, no tip
	if _, _, err := c.AddBlock(nil); err == nil {
		t.Fatal("AddBlock on an empty chain should fail, got nil")
	}
}

func TestGenesisNonEmptyPrevHashIsInvalid(t *testing.T) {
	bobby := newTestWallet(t)

	// A genesis block whose hash is correctly computed over a non-empty PrevHash:
	// step 1 (hash recompute) passes, so step 3's genesis rule fires.
	genesis := ledger.NewBlock([]*ledger.Transaction{ledger.NewCoinbaseTx(bobby.PublicKey(), 100)}, []byte{0x01})
	c := &ledger.Chain{Blocks: []*ledger.Block{genesis}}

	if err := c.IsValid(); err == nil {
		t.Fatal("genesis with a non-empty PrevHash should be invalid, got nil")
	}
}

func TestCoinbaseOutsideGenesisIsInvalid(t *testing.T) {
	alice := newTestWallet(t)
	bobby := newTestWallet(t)

	chain := ledger.NewChain(alice.PublicKey(), 100, testDifficulty)
	// A coinbase verifies on its own, but is only allowed in genesis.
	if _, _, err := chain.AddBlock([]*ledger.Transaction{ledger.NewCoinbaseTx(bobby.PublicKey(), 50)}); err != nil {
		t.Fatalf("AddBlock: %v", err)
	}

	if err := chain.IsValid(); err == nil {
		t.Fatal("a coinbase outside genesis should make the chain invalid, got nil")
	}
}

func TestTamperedNonceIsInvalid(t *testing.T) {
	chain := buildChain(t)

	// Change the mined nonce without re-mining: the stored Hash no longer matches
	// ComputeHash, so the chain is invalid.
	chain.Blocks[1].Nonce++

	if err := chain.IsValid(); err == nil {
		t.Fatal("chain with a tampered nonce should be invalid, got nil")
	}
}

func TestInsufficientDifficultyIsInvalid(t *testing.T) {
	chain := buildChain(t) // mined at testDifficulty

	// Raise the bar after the fact: blocks mined at testDifficulty are extremely
	// unlikely to satisfy a much higher target, so PoW validation must fail.
	chain.Difficulty = testDifficulty + 6

	if err := chain.IsValid(); err == nil {
		t.Fatal("chain whose blocks no longer meet difficulty should be invalid, got nil")
	}
}

// BenchmarkIsValid measures full-chain validation across several chain lengths.
// Validation cost is dominated by the per-transaction signature verification, so
// it scales roughly linearly with the number of blocks.
func BenchmarkIsValid(b *testing.B) {
	for _, n := range []int{1, 10, 100} {
		b.Run(fmt.Sprintf("blocks=%d", n), func(b *testing.B) {
			alice := newTestWallet(b)
			bobby := newTestWallet(b)
			// Difficulty 1 keeps mining negligible — this benchmark measures
			// IsValid, not Proof of Work.
			chain := ledger.NewChain(alice.PublicKey(), 1_000_000, 1)
			for range n {
				tx := ledger.NewTransaction(alice.PublicKey(), bobby.PublicKey(), 1)
				if err := tx.Sign(alice); err != nil {
					b.Fatalf("Sign: %v", err)
				}
				if _, _, err := chain.AddBlock([]*ledger.Transaction{tx}); err != nil {
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
