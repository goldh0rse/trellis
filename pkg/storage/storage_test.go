package storage_test

import (
	"errors"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/goldh0rse/trellis/pkg/ledger"
	"github.com/goldh0rse/trellis/pkg/storage"
	"github.com/goldh0rse/trellis/pkg/wallet"
)

// newBolt opens a Bolt store on a throwaway temp path and closes it on cleanup.
func newBolt(t *testing.T) *storage.Bolt {
	t.Helper()
	s, err := storage.NewBolt(filepath.Join(t.TempDir(), "trellis.db"))
	if err != nil {
		t.Fatalf("NewBolt: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

// minedBlock builds a mined block carrying one signed transaction.
func minedBlock(t *testing.T) *ledger.Block {
	t.Helper()
	alice, err := wallet.NewWallet()
	if err != nil {
		t.Fatalf("NewWallet: %v", err)
	}
	bobby, err := wallet.NewWallet()
	if err != nil {
		t.Fatalf("NewWallet: %v", err)
	}
	tx := ledger.NewTransaction(alice.PublicKey(), bobby.PublicKey(), 5)
	if err := tx.Sign(alice); err != nil {
		t.Fatalf("Sign: %v", err)
	}
	b := ledger.NewBlock([]*ledger.Transaction{tx}, nil)
	b.Mine(1)
	return b
}

func TestTipEmptyOnFreshDB(t *testing.T) {
	s := newBolt(t)
	tip, err := s.Tip()
	if err != nil {
		t.Fatalf("Tip: %v", err)
	}
	if len(tip) != 0 {
		t.Fatalf("fresh DB tip = %x, want empty", tip)
	}
}

func TestGetMissingReturnsErrBlockNotFound(t *testing.T) {
	s := newBolt(t)
	_, err := s.GetBlock([]byte("no such hash"))
	if !errors.Is(err, ledger.ErrBlockNotFound) {
		t.Fatalf("GetBlock(missing) error = %v, want ErrBlockNotFound", err)
	}
}

func TestBoltPersistsAcrossReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "trellis.db")
	alice, err := wallet.NewWallet()
	if err != nil {
		t.Fatalf("NewWallet: %v", err)
	}
	bobby, err := wallet.NewWallet()
	if err != nil {
		t.Fatalf("NewWallet: %v", err)
	}

	// First session: build genesis + one signed block, then close.
	s1, err := storage.NewBolt(path)
	if err != nil {
		t.Fatalf("NewBolt: %v", err)
	}
	chain, err := ledger.NewChain(s1, alice.PublicKey(), 100, 1)
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
	wantTip, _ := chain.Tip()
	if err := s1.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Second session: reopen the same file. NewChain must load, not re-seed.
	s2, err := storage.NewBolt(path)
	if err != nil {
		t.Fatalf("reopen NewBolt: %v", err)
	}
	defer s2.Close()
	reloaded, err := ledger.NewChain(s2, alice.PublicKey(), 100, 1)
	if err != nil {
		t.Fatalf("reopen NewChain: %v", err)
	}

	h, err := reloaded.Height()
	if err != nil {
		t.Fatalf("Height: %v", err)
	}
	if h != 2 {
		t.Fatalf("reopened chain height = %d, want 2 (genesis + 1)", h)
	}
	gotTip, _ := reloaded.Tip()
	if !reflect.DeepEqual(gotTip, wantTip) {
		t.Fatalf("reopened tip = %x, want %x", gotTip, wantTip)
	}
	if err := reloaded.IsValid(); err != nil {
		t.Fatalf("reopened chain should be valid, got: %v", err)
	}
}

func TestAppendAndGetRoundTrip(t *testing.T) {
	s := newBolt(t)
	b := minedBlock(t)

	if err := s.AppendBlock(b); err != nil {
		t.Fatalf("AppendBlock: %v", err)
	}

	// Tip advances to the appended block.
	tip, err := s.Tip()
	if err != nil {
		t.Fatalf("Tip: %v", err)
	}
	if !reflect.DeepEqual(tip, b.Hash) {
		t.Fatalf("tip = %x, want %x", tip, b.Hash)
	}

	// The block round-trips through gob with full fidelity (including the
	// ~2420-byte signature).
	got, err := s.GetBlock(b.Hash)
	if err != nil {
		t.Fatalf("GetBlock: %v", err)
	}
	if !reflect.DeepEqual(got, b) {
		t.Fatal("round-tripped block does not equal the original")
	}
}
