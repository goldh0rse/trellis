package wallet

import "sort"

// Keyring is a collection of wallets backed by a KeyStore. It caches wallets in
// memory (reconstructed from their seeds) and writes new ones through to the
// store.
type Keyring struct {
	ks      KeyStore
	wallets map[string]*Wallet // address -> wallet
}

// OpenKeyring loads all wallets from the store into memory.
func OpenKeyring(ks KeyStore) (*Keyring, error) {
	seeds, err := ks.Load()
	if err != nil {
		return nil, err
	}
	wallets := make(map[string]*Wallet, len(seeds))
	for addr, seed := range seeds {
		w, err := FromSeed(seed)
		if err != nil {
			return nil, err
		}
		wallets[addr] = w
	}
	return &Keyring{ks: ks, wallets: wallets}, nil
}

// CreateWallet generates a new wallet, persists its seed, caches it, and returns
// the wallet and its address.
func (k *Keyring) CreateWallet() (*Wallet, string, error) {
	w, err := NewWallet()
	if err != nil {
		return nil, "", err
	}
	addr := w.Address()
	if err := k.ks.Save(addr, w.Seed()); err != nil {
		return nil, "", err
	}
	k.wallets[addr] = w
	return w, addr, nil
}

// Wallet returns the wallet for an address and whether it is held by this keyring.
func (k *Keyring) Wallet(address string) (*Wallet, bool) {
	w, ok := k.wallets[address]
	return w, ok
}

// Addresses returns all held addresses in sorted order (deterministic output).
func (k *Keyring) Addresses() []string {
	addrs := make([]string, 0, len(k.wallets))
	for addr := range k.wallets {
		addrs = append(addrs, addr)
	}
	sort.Strings(addrs)
	return addrs
}
