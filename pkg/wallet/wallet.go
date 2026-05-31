// Package wallet provides a post-quantum identity: an ML-DSA-44 keypair (via
// pkg/pqsig) plus a display address. A Wallet can sign messages, which lets it
// satisfy the ledger's Signer interface without the ledger knowing about wallets.
package wallet

import (
	"crypto/sha256"
	"encoding/hex"

	"github.com/goldh0rse/trellis/pkg/pqsig"
)

// Wallet wraps an ML-DSA-44 keypair. Phase 1 keeps the key in memory only;
// persisting it is a later concern.
type Wallet struct {
	priv *pqsig.PrivateKey
}

// NewWallet generates a fresh keypair.
func NewWallet() (*Wallet, error) {
	priv, err := pqsig.GenerateKey()
	if err != nil {
		return nil, err
	}
	return &Wallet{priv: priv}, nil
}

// FromSeed restores a wallet from a 32-byte seed (as returned by Seed).
func FromSeed(seed []byte) (*Wallet, error) {
	priv, err := pqsig.NewPrivateKeyFromSeed(seed)
	if err != nil {
		return nil, err
	}
	return &Wallet{priv: priv}, nil
}

// Seed returns the wallet's 32-byte seed for persistence.
//
// Learning project: seeds are stored unencrypted — never use this to secure
// anything real.
func (w *Wallet) Seed() []byte {
	return w.priv.Seed()
}

// PublicKey returns the raw public-key bytes used as a transaction's From/To.
func (w *Wallet) PublicKey() []byte {
	return w.priv.PublicKey()
}

// Sign produces a signature over message. This is what lets a Wallet satisfy the
// ledger's Signer interface.
func (w *Wallet) Sign(message []byte) ([]byte, error) {
	return w.priv.Sign(message)
}

// Address derives a short, display-friendly identifier from the public key: the
// SHA-256 of the public-key bytes, hex-encoded and truncated to 16 chars.
//
// Hashing stays SHA-256 (already quantum-resistant). The truncation is cosmetic:
// the chain always keys transactions on the full public key, never the address,
// so a truncated-address collision cannot affect consensus or verification.
func (w *Wallet) Address() string {
	sum := sha256.Sum256(w.PublicKey())
	return hex.EncodeToString(sum[:])[:16]
}
