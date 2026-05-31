package storage_test

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/goldh0rse/trellis/pkg/storage"
	"github.com/goldh0rse/trellis/pkg/wallet"
)

// KeyFile must satisfy the wallet.KeyStore interface. Asserting it here (in the
// test) keeps the production storage package free of any wallet import.
var _ wallet.KeyStore = (*storage.KeyFile)(nil)

func TestKeyFileLoadMissingReturnsEmpty(t *testing.T) {
	kf := storage.NewKeyFile(filepath.Join(t.TempDir(), "wallets.dat"))
	m, err := kf.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(m) != 0 {
		t.Fatalf("Load on missing file = %v, want empty map", m)
	}
}

func TestKeyFileSaveLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "wallets.dat")
	kf := storage.NewKeyFile(path)

	seedA := bytes.Repeat([]byte{0xa1}, 32)
	seedB := bytes.Repeat([]byte{0xb2}, 32)
	if err := kf.Save("addrA", seedA); err != nil {
		t.Fatalf("Save A: %v", err)
	}
	if err := kf.Save("addrB", seedB); err != nil {
		t.Fatalf("Save B: %v", err)
	}

	// A fresh KeyFile on the same path sees both seeds.
	m, err := storage.NewKeyFile(path).Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(m) != 2 {
		t.Fatalf("loaded %d seeds, want 2", len(m))
	}
	if !bytes.Equal(m["addrA"], seedA) || !bytes.Equal(m["addrB"], seedB) {
		t.Fatal("seeds did not round-trip intact")
	}
}

func TestKeyFileSaveOverwrites(t *testing.T) {
	path := filepath.Join(t.TempDir(), "wallets.dat")
	kf := storage.NewKeyFile(path)

	if err := kf.Save("addr", bytes.Repeat([]byte{0x01}, 32)); err != nil {
		t.Fatalf("Save: %v", err)
	}
	updated := bytes.Repeat([]byte{0x02}, 32)
	if err := kf.Save("addr", updated); err != nil {
		t.Fatalf("Save overwrite: %v", err)
	}

	m, err := kf.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(m) != 1 || !bytes.Equal(m["addr"], updated) {
		t.Fatalf("overwrite failed: %v", m)
	}
}
