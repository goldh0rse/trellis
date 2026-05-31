// Package pqsig is the project's post-quantum signature boundary. It is the only
// package that imports filippo.io/mldsa, so swapping in the stdlib crypto/mldsa
// (slated for Go 1.27) will be a one-file change.
//
// It wraps ML-DSA-44 (FIPS 204 / CRYSTALS-Dilithium). Public keys are passed
// around as raw bytes, which is what callers persist on a transaction.
package pqsig

import mldsa "filippo.io/mldsa"

// PrivateKey is an ML-DSA-44 private key. Keep it in memory only for now;
// persisting the seed is a later concern.
type PrivateKey struct {
	sk *mldsa.PrivateKey
}

// GenerateKey creates a fresh ML-DSA-44 keypair.
func GenerateKey() (*PrivateKey, error) {
	sk, err := mldsa.GenerateKey(mldsa.MLDSA44())
	if err != nil {
		return nil, err
	}
	return &PrivateKey{sk: sk}, nil
}

// PublicKey returns the raw 1312-byte ML-DSA-44 public-key encoding. It is what
// callers store (e.g. on a transaction) and round-trips through Verify.
func (k *PrivateKey) PublicKey() []byte {
	return k.sk.PublicKey().Bytes()
}

// Sign produces an ML-DSA-44 signature over message. (ML-DSA hashes internally
// per FIPS 204, so do not pre-hash; the io.Reader is ignored, hence nil.)
func (k *PrivateKey) Sign(message []byte) ([]byte, error) {
	return k.sk.Sign(nil, message, nil)
}

// Verify reports whether signature is a valid ML-DSA-44 signature of message
// under publicKey (the raw bytes from PublicKey). It returns nil when valid, and
// a non-nil error when the signature is invalid or publicKey is malformed.
func Verify(publicKey, message, signature []byte) error {
	pk, err := mldsa.NewPublicKey(mldsa.MLDSA44(), publicKey)
	if err != nil {
		return err
	}
	return mldsa.Verify(pk, message, signature, nil)
}
