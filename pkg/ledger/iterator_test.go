package ledger_test

import (
	"bytes"
	"testing"

	"github.com/goldh0rse/trellis/pkg/ledger"
)

func TestIteratorWalksTipToGenesis(t *testing.T) {
	alice := newTestWallet(t)
	bobby := newTestWallet(t)

	store := newMemStore()
	chain, err := ledger.NewChain(store, alice.PublicKey(), 100, testDifficulty)
	if err != nil {
		t.Fatalf("NewChain: %v", err)
	}
	for range 2 {
		tx := ledger.NewTransaction(alice.PublicKey(), bobby.PublicKey(), 1)
		if err := tx.Sign(alice); err != nil {
			t.Fatalf("Sign: %v", err)
		}
		if _, _, err := chain.AddBlock([]*ledger.Transaction{tx}); err != nil {
			t.Fatalf("AddBlock: %v", err)
		}
	}

	it, err := chain.Iterator()
	if err != nil {
		t.Fatalf("Iterator: %v", err)
	}

	var hashes [][]byte
	for {
		b, err := it.Next()
		if err != nil {
			t.Fatalf("Next: %v", err)
		}
		if b == nil {
			break // clean end
		}
		hashes = append(hashes, b.Hash)
	}

	// genesis + 2 blocks, returned tip-first.
	if len(hashes) != 3 {
		t.Fatalf("iterator returned %d blocks, want 3", len(hashes))
	}
	// The first returned block is the tip.
	tip, _ := chain.Tip()
	if !bytes.Equal(hashes[0], tip) {
		t.Fatal("iterator did not start at the tip")
	}
	// Each block links to the next via PrevHash (we walked tip -> genesis).
	for i := 0; i < len(hashes)-1; i++ {
		b, err := store.GetBlock(hashes[i])
		if err != nil {
			t.Fatalf("GetBlock: %v", err)
		}
		if !bytes.Equal(b.PrevHash, hashes[i+1]) {
			t.Fatalf("block %d PrevHash does not point to block %d", i, i+1)
		}
	}
	// The last block is genesis: empty PrevHash.
	last, err := store.GetBlock(hashes[len(hashes)-1])
	if err != nil {
		t.Fatalf("GetBlock: %v", err)
	}
	if len(last.PrevHash) != 0 {
		t.Fatal("last block should be genesis with empty PrevHash")
	}
}
