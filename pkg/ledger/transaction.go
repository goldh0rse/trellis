// Package ledger holds the chain's data model and rules: transactions, blocks,
// the chain itself, Proof of Work, and validation. It depends on pkg/pqsig for
// signature verification but knows nothing about wallets — signing is expressed
// through the Signer interface, which a wallet satisfies.
package ledger

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"

	"github.com/goldh0rse/trellis/pkg/pqsig"
)

// Signer is anything that can sign a message with a key and expose its public
// key. A *wallet.Wallet satisfies it. Declaring the interface here (rather than
// importing wallet) keeps the ledger decoupled from the identity layer.
type Signer interface {
	PublicKey() []byte
	Sign(message []byte) ([]byte, error)
}

// Transaction is an account-style transfer: From sends Amount to To. From and To
// are raw ML-DSA-44 public keys. A transaction with an empty From is a coinbase:
// a signature-free issuance of new coins (genesis or, later, a block reward) —
// this is how coins enter circulation.
type Transaction struct {
	From      []byte // sender public key; empty == coinbase
	To        []byte // recipient public key
	Amount    uint64 // account model: no negative sends, so unsigned
	Signature []byte // signature over signingPayload(); empty for coinbase
	ID        []byte // SHA-256 of signingPayload(); identity/display, derivable
}

// signingPayload returns the canonical bytes the signature covers. It commits to
// From, To and Amount — but deliberately NOT to Signature or ID. Every
// variable-length field is length-prefixed and all integers are big-endian, so
// the byte stream is unambiguous:
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

// NewTransaction builds an unsigned transfer. The caller must Sign it before
// adding it to a block.
func NewTransaction(from, to []byte, amount uint64) *Transaction {
	tx := &Transaction{From: from, To: to, Amount: amount}
	tx.ID = tx.hashID()
	return tx
}

// NewCoinbaseTx builds a signature-free issuance of `amount` coins to `to`.
func NewCoinbaseTx(to []byte, amount uint64) *Transaction {
	tx := &Transaction{To: to, Amount: amount}
	tx.ID = tx.hashID()
	return tx
}

// IsCoinbase reports whether this is a coinbase (issuance) transaction.
func (tx *Transaction) IsCoinbase() bool {
	return len(tx.From) == 0
}

// Sign signs the transaction with the given signer and stores the signature. It
// refuses to sign a coinbase, and refuses unless From is exactly the signer's
// public key (you can only spend from your own account).
func (tx *Transaction) Sign(s Signer) error {
	if tx.IsCoinbase() {
		return errors.New("cannot sign a coinbase transaction")
	}
	if !bytes.Equal(tx.From, s.PublicKey()) {
		return errors.New("cannot sign: From does not match signer public key")
	}
	sig, err := s.Sign(tx.signingPayload())
	if err != nil {
		return err
	}
	tx.Signature = sig
	tx.ID = tx.hashID()
	return nil
}

// Verify checks the transaction's integrity.
//   - Coinbase (empty From): valid only if it carries no signature.
//   - Otherwise: the signature must verify against From over the signing payload.
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
	// pqsig.Verify reconstructs the key from From (rejecting malformed bytes) and
	// returns nil only when the signature is valid.
	return pqsig.Verify(tx.From, tx.signingPayload(), tx.Signature)
}
