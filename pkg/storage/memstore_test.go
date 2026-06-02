package storage_test

import (
	"errors"
	"testing"

	"github.com/goldh0rse/trellis/pkg/ledger"
	"github.com/goldh0rse/trellis/pkg/storage"
)

// MemStore must satisfy ledger.Store.
var _ ledger.Store = (*storage.MemStore)(nil)

func TestMemStoreTipEmpty(t *testing.T) {
	s := storage.NewMemStore()
	tip, err := s.Tip()
	if err != nil {
		t.Fatalf("Tip: %v", err)
	}
	if len(tip) != 0 {
		t.Fatalf("fresh MemStore tip = %x, want empty", tip)
	}
}

func TestMemStoreGetMissing(t *testing.T) {
	s := storage.NewMemStore()
	if _, err := s.GetBlock([]byte("nope")); !errors.Is(err, ledger.ErrBlockNotFound) {
		t.Fatalf("GetBlock(missing) error = %v, want ErrBlockNotFound", err)
	}
}

func TestMemStoreAppendAndGet(t *testing.T) {
	s := storage.NewMemStore()
	b := minedBlock(t) // helper from storage_test.go

	if err := s.AppendBlock(b); err != nil {
		t.Fatalf("AppendBlock: %v", err)
	}
	tip, _ := s.Tip()
	if string(tip) != string(b.Hash) {
		t.Fatalf("tip = %x, want %x", tip, b.Hash)
	}
	got, err := s.GetBlock(b.Hash)
	if err != nil {
		t.Fatalf("GetBlock: %v", err)
	}
	if got != b {
		t.Fatal("GetBlock did not return the stored block")
	}
}
