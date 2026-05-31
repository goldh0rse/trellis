package blockchain

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"

	mldsa "filippo.io/mldsa"
)

// Transaction is an account-style transfer: From sends Amount to To. From and
// To are raw ML-DSA-44 public keys (1312 bytes each). A transaction with an
// empty From is a coinbase: a signature-free issuance of new coins (genesis or,
// later, a block reward) — this is how coins enter circulation.
type Transaction struct {
	From      []byte // sender public key; empty == coinbase
	To        []byte // recipient public key
	Amount    uint64 // account model: no negative sends, so unsigned
	Signature []byte // ML-DSA-44 signature over signingPayload(); empty for coinbase
	ID        []byte // SHA-256 of signingPayload(); identity/display, derivable
}

// signingPayload returns the canonical bytes the signature covers. It commits
// to From, To and Amount — but deliberately NOT to Signature or ID (those are
// derived from it). Every variable-length field is length-prefixed and all
// integers are big-endian, so the byte stream is unambiguous:
//
//	uint32(len(From)) || From || uint32(len(To)) || To || uint64(Amount)
func (tx *Transaction) signingPayload() []byte {
	var b bytes.Buffer
	var lp [4]byte

	binary.BigEndian.PutUint32(lp[:], uint32(len(tx.From)))
	b.Write(lp[:])
	b.Write(tx.From)

	binary.BigEndian.PutUint32(lp[:], uint32(len(tx.To)))
	b.Write(lp[:])
	b.Write(tx.To)

	var amt [8]byte
	binary.BigEndian.PutUint64(amt[:], tx.Amount)
	b.Write(amt[:])

	return b.Bytes()
}

// hashID computes the transaction's SHA-256 id over the signing payload.
func (tx *Transaction) hashID() []byte {
	sum := sha256.Sum256(tx.signingPayload())
	return sum[:]
}

// NewTransaction builds an unsigned transfer. The caller must Sign it (with the
// sender's wallet) before adding it to a block.
func NewTransaction(from, to []byte, amount uint64) *Transaction {
	tx := &Transaction{From: from, To: to, Amount: amount}
	tx.ID = tx.hashID()
	return tx
}

// NewCoinbaseTx builds a signature-free issuance of `amount` coins to `to`.
// This is the only way new coins enter circulation.
func NewCoinbaseTx(to []byte, amount uint64) *Transaction {
	tx := &Transaction{To: to, Amount: amount}
	tx.ID = tx.hashID()
	return tx
}

// IsCoinbase reports whether this is a coinbase (issuance) transaction.
func (tx *Transaction) IsCoinbase() bool {
	return len(tx.From) == 0
}

// Sign signs the transaction with the wallet's key and stores the signature.
// It refuses to sign a coinbase, and refuses to sign unless From is exactly the
// wallet's public key (you can only spend from your own account).
func (tx *Transaction) Sign(w *Wallet) error {
	if tx.IsCoinbase() {
		return errors.New("cannot sign a coinbase transaction")
	}
	if !bytes.Equal(tx.From, w.PublicKey()) {
		return errors.New("cannot sign: From does not match wallet public key")
	}
	// The io.Reader is ignored by ML-DSA; nil opts signs the message directly.
	sig, err := w.priv.Sign(nil, tx.signingPayload(), nil)
	if err != nil {
		return err
	}
	tx.Signature = sig
	tx.ID = tx.hashID()
	return nil
}

// Verify checks the transaction's integrity.
//   - Coinbase (empty From): valid only if it carries no signature.
//   - Otherwise: reconstruct the public key from From and verify the signature
//     over the canonical signing payload.
//
// Returns nil when the transaction is valid.
func (tx *Transaction) Verify() error {
	if tx.IsCoinbase() {
		if len(tx.Signature) != 0 {
			return errors.New("coinbase must not be signed")
		}
		return nil
	}
	if len(tx.Signature) == 0 {
		return errors.New("missing signature")
	}
	// Reconstruct the public key from the raw From bytes. This also rejects a
	// malformed or wrong-length From (returns an error rather than panicking).
	pk, err := mldsa.NewPublicKey(mldsa.MLDSA44(), tx.From)
	if err != nil {
		return err
	}
	// mldsa.Verify returns nil only when the signature is VALID.
	return mldsa.Verify(pk, tx.signingPayload(), tx.Signature, nil)
}
