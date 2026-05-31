package wallet

// KeyStore persists wallet seeds, keyed by address. It is the wallet package's
// persistence boundary (consumer-side, like ledger.Store): the Keyring depends
// only on this interface, while the concrete file implementation lives in
// pkg/storage.
//
// Learning project: implementations store seeds unencrypted.
type KeyStore interface {
	// Save persists the 32-byte seed for an address (overwriting any existing).
	Save(address string, seed []byte) error

	// Load returns all stored address→seed pairs, or an empty map if none.
	Load() (map[string][]byte, error)
}
