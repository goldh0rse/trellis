package ledger_test

import (
	"fmt"
	"testing"

	"github.com/goldh0rse/trellis/pkg/ledger"
)

// testDifficulty keeps mining instant in tests while still exercising the PoW
// path (a few hundred attempts at most).
const testDifficulty = 2

// buildChain returns a chain (genesis: 100 coins to alice, plus one block holding
// a signed alice -> bobby : 30 transfer) and the backing in-memory store, so
// tamper tests can reach the stored blocks.
func buildChain(t testing.TB) (*ledger.Chain, *memStore) {
	t.Helper()
	alice := newTestWallet(t)
	bobby := newTestWallet(t)

	store := newMemStore()
	chain, err := ledger.NewChain(store, alice.PublicKey(), 100, testDifficulty)
	if err != nil {
		t.Fatalf("NewChain: %v", err)
	}

	tx := ledger.NewTransaction(alice.PublicKey(), bobby.PublicKey(), 30)
	if err := tx.Sign(alice); err != nil {
		t.Fatalf("Sign: %v", err)
	}
	if _, _, err := chain.AddBlock([]*ledger.Transaction{tx}); err != nil {
		t.Fatalf("AddBlock: %v", err)
	}
	return chain, store
}

func TestChainHappyPathIsValid(t *testing.T) {
	chain, _ := buildChain(t)
	if err := chain.IsValid(); err != nil {
		t.Fatalf("freshly built chain should be valid, got: %v", err)
	}
}

func TestChainTamperedAmountIsInvalid(t *testing.T) {
	chain, store := buildChain(t)

	// Forge the transfer by mutating the stored tip block in place.
	store.tipBlock().Transactions[0].Amount = 31

	if err := chain.IsValid(); err == nil {
		t.Fatal("chain with a tampered amount should be invalid, got nil")
	}
}

func TestChainTamperedPrevHashIsInvalid(t *testing.T) {
	chain, store := buildChain(t)

	// PrevHash is part of ComputeHash, so changing it breaks hash integrity.
	store.tipBlock().PrevHash = []byte("not the genesis hash")

	if err := chain.IsValid(); err == nil {
		t.Fatal("chain with a tampered PrevHash should be invalid, got nil")
	}
}

func TestTamperedNonceIsInvalid(t *testing.T) {
	chain, store := buildChain(t)

	store.tipBlock().Nonce++ // change the mined nonce without re-mining

	if err := chain.IsValid(); err == nil {
		t.Fatal("chain with a tampered nonce should be invalid, got nil")
	}
}

func TestInsufficientDifficultyIsInvalid(t *testing.T) {
	chain, _ := buildChain(t) // mined at testDifficulty

	// Raise the bar after the fact: blocks mined at testDifficulty are extremely
	// unlikely to satisfy a much higher target, so PoW validation must fail.
	chain.Difficulty = testDifficulty + 6

	if err := chain.IsValid(); err == nil {
		t.Fatal("chain whose blocks no longer meet difficulty should be invalid, got nil")
	}
}

func TestCoinbaseOutsideGenesisIsInvalid(t *testing.T) {
	alice := newTestWallet(t)
	bobby := newTestWallet(t)

	store := newMemStore()
	chain, err := ledger.NewChain(store, alice.PublicKey(), 100, testDifficulty)
	if err != nil {
		t.Fatalf("NewChain: %v", err)
	}
	// A coinbase verifies on its own, but is only allowed in genesis.
	if _, _, err := chain.AddBlock([]*ledger.Transaction{ledger.NewCoinbaseTx(bobby.PublicKey(), 50)}); err != nil {
		t.Fatalf("AddBlock: %v", err)
	}

	if err := chain.IsValid(); err == nil {
		t.Fatal("a coinbase outside genesis should make the chain invalid, got nil")
	}
}

func TestChainWithoutGenesisIsInvalid(t *testing.T) {
	bobby := newTestWallet(t)

	// A single block whose PrevHash points nowhere: the chain never reaches a
	// genesis, so validation must fail (here, a dangling parent link).
	store := newMemStore()
	orphan := ledger.NewBlock([]*ledger.Transaction{ledger.NewCoinbaseTx(bobby.PublicKey(), 100)}, []byte{0x01})
	orphan.Mine(testDifficulty)
	if err := store.AppendBlock(orphan); err != nil {
		t.Fatalf("AppendBlock: %v", err)
	}
	loaded, err := ledger.NewChain(store, bobby.PublicKey(), 0, testDifficulty)
	if err != nil {
		t.Fatalf("NewChain (load): %v", err)
	}

	if err := loaded.IsValid(); err == nil {
		t.Fatal("a chain that does not terminate at genesis should be invalid, got nil")
	}
}

func TestNewChainSeedsGenesisOnEmptyStore(t *testing.T) {
	alice := newTestWallet(t)
	store := newMemStore()

	chain, err := ledger.NewChain(store, alice.PublicKey(), 100, testDifficulty)
	if err != nil {
		t.Fatalf("NewChain: %v", err)
	}
	h, err := chain.Height()
	if err != nil {
		t.Fatalf("Height: %v", err)
	}
	if h != 1 {
		t.Fatalf("fresh chain height = %d, want 1 (genesis)", h)
	}
	if err := chain.IsValid(); err != nil {
		t.Fatalf("fresh chain should be valid, got: %v", err)
	}
}

func TestNewChainLoadsExistingStore(t *testing.T) {
	_, store := buildChain(t) // genesis + 1 block
	alice := newTestWallet(t)

	// Re-opening the same store must load it, not re-seed genesis.
	reloaded, err := ledger.NewChain(store, alice.PublicKey(), 100, testDifficulty)
	if err != nil {
		t.Fatalf("NewChain (reload): %v", err)
	}
	h, err := reloaded.Height()
	if err != nil {
		t.Fatalf("Height: %v", err)
	}
	if h != 2 {
		t.Fatalf("reloaded chain height = %d, want 2", h)
	}
	if err := reloaded.IsValid(); err != nil {
		t.Fatalf("reloaded chain should be valid, got: %v", err)
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
			store := newMemStore()
			// Difficulty 1 keeps mining negligible — this measures IsValid.
			chain, err := ledger.NewChain(store, alice.PublicKey(), 1_000_000, 1)
			if err != nil {
				b.Fatalf("NewChain: %v", err)
			}
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
