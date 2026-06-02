package ledger_test

import (
	"errors"
	"testing"

	"github.com/goldh0rse/trellis/pkg/ledger"
)

// minedExtension builds a valid mined block extending the given tip, carrying one
// signed transfer.
func minedExtension(t *testing.T, tip []byte, difficulty int) *ledger.Block {
	t.Helper()
	alice := newTestWallet(t)
	bobby := newTestWallet(t)
	tx := ledger.NewTransaction(alice.PublicKey(), bobby.PublicKey(), 1)
	if err := tx.Sign(alice); err != nil {
		t.Fatalf("Sign: %v", err)
	}
	b := ledger.NewBlock([]*ledger.Transaction{tx}, tip)
	b.Mine(difficulty)
	return b
}

func TestAcceptBlockExtendsTip(t *testing.T) {
	chain, store := buildChain(t) // genesis + 1 block, at testDifficulty
	tip := store.tip

	b := minedExtension(t, tip, testDifficulty)
	if err := chain.AcceptBlock(b); err != nil {
		t.Fatalf("AcceptBlock of a valid extension should succeed, got: %v", err)
	}
	if err := chain.IsValid(); err != nil {
		t.Fatalf("chain should be valid after accepting a block, got: %v", err)
	}
}

func TestAcceptBlockGenesisOnEmptyStore(t *testing.T) {
	bobby := newTestWallet(t)
	store := newMemStore()
	chain, err := ledger.LoadChain(store, testDifficulty)
	if err != nil {
		t.Fatalf("LoadChain: %v", err)
	}

	genesis := ledger.NewBlock([]*ledger.Transaction{ledger.NewCoinbaseTx(bobby.PublicKey(), 100)}, nil)
	genesis.Mine(testDifficulty)
	if err := chain.AcceptBlock(genesis); err != nil {
		t.Fatalf("AcceptBlock of a valid genesis should succeed, got: %v", err)
	}
	h, _ := chain.Height()
	if h != 1 {
		t.Fatalf("height = %d, want 1", h)
	}
}

func TestAcceptBlockRejectsNonExtending(t *testing.T) {
	chain, _ := buildChain(t)

	// Builds on a bogus parent, not the current tip.
	b := minedExtension(t, []byte("not the tip"), testDifficulty)
	if err := chain.AcceptBlock(b); !errors.Is(err, ledger.ErrNotExtending) {
		t.Fatalf("AcceptBlock error = %v, want ErrNotExtending", err)
	}
}

func TestAcceptBlockRejectsBadPoW(t *testing.T) {
	chain, store := buildChain(t)

	// Mine at difficulty 0 (no leading zeros required), then present it to a chain
	// expecting testDifficulty.
	b := minedExtension(t, store.tip, 0)
	if ledger.MeetsDifficulty(b.Hash, testDifficulty) {
		t.Skip("mined hash coincidentally meets difficulty; rerun")
	}
	if err := chain.AcceptBlock(b); err == nil {
		t.Fatal("AcceptBlock should reject a block failing the difficulty target")
	}
}

func TestAcceptBlockRejectsTamperedTx(t *testing.T) {
	chain, store := buildChain(t)
	b := minedExtension(t, store.tip, testDifficulty)

	b.Transactions[0].Amount++ // tamper after mining: breaks hash integrity

	if err := chain.AcceptBlock(b); err == nil {
		t.Fatal("AcceptBlock should reject a tampered block")
	}
}

func TestAcceptBlockRejectsCoinbaseOutsideGenesis(t *testing.T) {
	chain, store := buildChain(t)
	bobby := newTestWallet(t)

	b := ledger.NewBlock([]*ledger.Transaction{ledger.NewCoinbaseTx(bobby.PublicKey(), 50)}, store.tip)
	b.Mine(testDifficulty)

	if err := chain.AcceptBlock(b); err == nil {
		t.Fatal("AcceptBlock should reject a coinbase outside genesis")
	}
}

func TestAcceptBlockRejectsSecondGenesis(t *testing.T) {
	chain, _ := buildChain(t) // non-empty store
	bobby := newTestWallet(t)

	genesis := ledger.NewBlock([]*ledger.Transaction{ledger.NewCoinbaseTx(bobby.PublicKey(), 100)}, nil)
	genesis.Mine(testDifficulty)

	if err := chain.AcceptBlock(genesis); err == nil {
		t.Fatal("AcceptBlock should reject a second genesis on a non-empty chain")
	}
}
