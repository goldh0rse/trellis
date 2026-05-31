package wallet

import (
	"bytes"
	"testing"
)

func TestFromSeedRoundTrip(t *testing.T) {
	w := newTestWallet(t)
	seed := w.Seed()

	restored, err := FromSeed(seed)
	if err != nil {
		t.Fatalf("FromSeed: %v", err)
	}
	if restored.Address() != w.Address() {
		t.Fatalf("restored address %q != original %q", restored.Address(), w.Address())
	}
	if !bytes.Equal(restored.PublicKey(), w.PublicKey()) {
		t.Fatal("restored wallet has a different public key")
	}
}

// memKeyStore is an in-memory KeyStore fake for keyring tests.
type memKeyStore struct {
	m map[string][]byte
}

func newMemKeyStore() *memKeyStore { return &memKeyStore{m: make(map[string][]byte)} }

func (k *memKeyStore) Save(address string, seed []byte) error {
	k.m[address] = append([]byte(nil), seed...)
	return nil
}

func (k *memKeyStore) Load() (map[string][]byte, error) {
	return k.m, nil
}

func TestKeyringCreateAndLookup(t *testing.T) {
	ks := newMemKeyStore()
	ring, err := OpenKeyring(ks)
	if err != nil {
		t.Fatalf("OpenKeyring: %v", err)
	}

	w, addr, err := ring.CreateWallet()
	if err != nil {
		t.Fatalf("CreateWallet: %v", err)
	}
	if addr != w.Address() {
		t.Fatalf("CreateWallet returned address %q != wallet.Address() %q", addr, w.Address())
	}

	got, ok := ring.Wallet(addr)
	if !ok || got != w {
		t.Fatal("Wallet(addr) did not return the created wallet")
	}
	if _, ok := ring.Wallet("nonexistent"); ok {
		t.Fatal("Wallet returned ok for an unknown address")
	}
}

func TestKeyringAddressesSorted(t *testing.T) {
	ring, err := OpenKeyring(newMemKeyStore())
	if err != nil {
		t.Fatalf("OpenKeyring: %v", err)
	}
	for range 3 {
		if _, _, err := ring.CreateWallet(); err != nil {
			t.Fatalf("CreateWallet: %v", err)
		}
	}
	addrs := ring.Addresses()
	if len(addrs) != 3 {
		t.Fatalf("Addresses len = %d, want 3", len(addrs))
	}
	for i := 1; i < len(addrs); i++ {
		if addrs[i-1] > addrs[i] {
			t.Fatalf("Addresses not sorted: %v", addrs)
		}
	}
}

func TestKeyringPersistsAcrossReopen(t *testing.T) {
	ks := newMemKeyStore()
	ring, err := OpenKeyring(ks)
	if err != nil {
		t.Fatalf("OpenKeyring: %v", err)
	}
	_, addr, err := ring.CreateWallet()
	if err != nil {
		t.Fatalf("CreateWallet: %v", err)
	}

	// Reopen against the same store: the wallet must still be there.
	reopened, err := OpenKeyring(ks)
	if err != nil {
		t.Fatalf("reopen OpenKeyring: %v", err)
	}
	if _, ok := reopened.Wallet(addr); !ok {
		t.Fatal("wallet did not survive keyring reopen")
	}
}
