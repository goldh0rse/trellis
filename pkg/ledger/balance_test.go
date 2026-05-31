package ledger_test

import (
	"testing"

	"github.com/goldh0rse/trellis/pkg/ledger"
	"github.com/goldh0rse/trellis/pkg/wallet"
)

func TestBalanceReflectsGenesisAndTransfers(t *testing.T) {
	alice := newTestWallet(t)
	bobby := newTestWallet(t)

	store := newMemStore()
	chain, err := ledger.NewChain(store, alice.PublicKey(), 100, testDifficulty)
	if err != nil {
		t.Fatalf("NewChain: %v", err)
	}

	// Genesis credited alice with 100.
	assertBalance(t, chain, alice, 100)
	assertBalance(t, chain, bobby, 0)

	// alice -> bobby : 30
	tx := ledger.NewTransaction(alice.PublicKey(), bobby.PublicKey(), 30)
	if err := tx.Sign(alice); err != nil {
		t.Fatalf("Sign: %v", err)
	}
	if _, _, err := chain.AddBlock([]*ledger.Transaction{tx}); err != nil {
		t.Fatalf("AddBlock: %v", err)
	}

	assertBalance(t, chain, alice, 70)
	assertBalance(t, chain, bobby, 30)

	// bobby -> alice : 10 (spans two blocks; replay order independent)
	tx2 := ledger.NewTransaction(bobby.PublicKey(), alice.PublicKey(), 10)
	if err := tx2.Sign(bobby); err != nil {
		t.Fatalf("Sign: %v", err)
	}
	if _, _, err := chain.AddBlock([]*ledger.Transaction{tx2}); err != nil {
		t.Fatalf("AddBlock: %v", err)
	}

	assertBalance(t, chain, alice, 80)
	assertBalance(t, chain, bobby, 20)
}

func assertBalance(t *testing.T, chain *ledger.Chain, w *wallet.Wallet, want uint64) {
	t.Helper()
	got, err := chain.Balance(w.PublicKey())
	if err != nil {
		t.Fatalf("Balance: %v", err)
	}
	if got != want {
		t.Fatalf("balance = %d, want %d", got, want)
	}
}
