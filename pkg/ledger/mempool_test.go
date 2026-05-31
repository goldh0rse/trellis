package ledger_test

import (
	"testing"

	"github.com/goldh0rse/trellis/pkg/ledger"
)

func TestMempoolAddValidatesSignature(t *testing.T) {
	alice := newTestWallet(t)
	bobby := newTestWallet(t)
	mp := ledger.NewMempool()

	// Unsigned transfer is rejected (Add runs Verify).
	unsigned := ledger.NewTransaction(alice.PublicKey(), bobby.PublicKey(), 5)
	if err := mp.Add(unsigned); err == nil {
		t.Fatal("Add should reject an unsigned transaction, got nil")
	}

	// Signed transfer is accepted.
	tx := ledger.NewTransaction(alice.PublicKey(), bobby.PublicKey(), 5)
	if err := tx.Sign(alice); err != nil {
		t.Fatalf("Sign: %v", err)
	}
	if err := mp.Add(tx); err != nil {
		t.Fatalf("Add of a signed tx should succeed, got: %v", err)
	}
	if got := mp.Transactions(); len(got) != 1 {
		t.Fatalf("Transactions len = %d, want 1", len(got))
	}
}

func TestMempoolTransactionsIsCopy(t *testing.T) {
	bobby := newTestWallet(t)
	mp := ledger.NewMempool()
	if err := mp.Add(ledger.NewCoinbaseTx(bobby.PublicKey(), 1)); err != nil {
		t.Fatalf("Add: %v", err)
	}

	snapshot := mp.Transactions()
	mp.Clear()
	if len(snapshot) != 1 {
		t.Fatal("Transactions() result was mutated by Clear()")
	}
	if len(mp.Transactions()) != 0 {
		t.Fatal("Clear did not empty the mempool")
	}
}
