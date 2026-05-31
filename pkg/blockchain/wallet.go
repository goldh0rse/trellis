package blockchain

import (
	"crypto/sha256"
	"encoding/hex"

	mldsa "filippo.io/mldsa"
)

// Wallet wraps an ML-DSA-44 keypair — the post-quantum identity of an actor on
// the chain. Phase 1 keeps the key in memory only; persisting the 32-byte seed
// (sk.Bytes()) is a Phase 4 concern.
type Wallet struct {
	priv *mldsa.PrivateKey
}

// NewWallet generates a fresh ML-DSA-44 keypair.
func NewWallet() (*Wallet, error) {
	priv, err := mldsa.GenerateKey(mldsa.MLDSA44())
	if err != nil {
		return nil, err
	}
	return &Wallet{priv: priv}, nil
}

// PublicKey returns the raw 1312-byte ML-DSA-44 public-key encoding. This is
// exactly what goes into Transaction.From / Transaction.To, and it round-trips
// through mldsa.NewPublicKey during verification.
func (w *Wallet) PublicKey() []byte {
	return w.priv.PublicKey().Bytes()
}

// Address derives a short, display-friendly identifier from the public key:
// the SHA-256 of the public key bytes, hex-encoded and truncated to 16 chars.
//
// Hashing stays SHA-256 (it is already quantum-resistant). The truncation is
// purely cosmetic: the chain always keys transactions on the full public key
// (From/To), never on the address, so a truncated-address collision can never
// affect consensus or signature verification.
func (w *Wallet) Address() string {
	sum := sha256.Sum256(w.PublicKey())
	return hex.EncodeToString(sum[:])[:16]
}
