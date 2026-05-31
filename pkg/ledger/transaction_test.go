package ledger_test

import (
	"testing"

	"github.com/goldh0rse/trellis/pkg/ledger"
)

func TestTransactionSignVerifyRoundTrip(t *testing.T) {
	alice := newTestWallet(t)
	bobby := newTestWallet(t)

	tx := ledger.NewTransaction(alice.PublicKey(), bobby.PublicKey(), 42)
	if err := tx.Sign(alice); err != nil {
		t.Fatalf("Sign: %v", err)
	}
	if err := tx.Verify(); err != nil {
		t.Fatalf("Verify after signing should pass, got: %v", err)
	}
}

func TestTransactionTamperedAmountFailsVerify(t *testing.T) {
	alice := newTestWallet(t)
	bobby := newTestWallet(t)

	tx := ledger.NewTransaction(alice.PublicKey(), bobby.PublicKey(), 42)
	if err := tx.Sign(alice); err != nil {
		t.Fatalf("Sign: %v", err)
	}

	tx.Amount = 43 // tamper after signing
	if err := tx.Verify(); err == nil {
		t.Fatal("Verify should fail for a tampered amount, got nil")
	}
}

func TestTransactionSignWrongWalletFails(t *testing.T) {
	alice := newTestWallet(t)
	bobby := newTestWallet(t)
	mallory := newTestWallet(t)

	// From is Alice, but Mallory tries to sign it.
	tx := ledger.NewTransaction(alice.PublicKey(), bobby.PublicKey(), 42)
	if err := tx.Sign(mallory); err == nil {
		t.Fatal("signing with a wallet that does not own From should fail, got nil")
	}
}

func TestCoinbaseVerify(t *testing.T) {
	bobby := newTestWallet(t)

	cb := ledger.NewCoinbaseTx(bobby.PublicKey(), 100)
	if !cb.IsCoinbase() {
		t.Fatal("NewCoinbaseTx should produce a coinbase transaction")
	}
	if err := cb.Verify(); err != nil {
		t.Fatalf("unsigned coinbase should verify, got: %v", err)
	}

	// A coinbase that carries a signature must be rejected.
	cb.Signature = []byte("not allowed")
	if err := cb.Verify(); err == nil {
		t.Fatal("a signed coinbase should fail Verify, got nil")
	}
}

func TestSignCoinbaseRejected(t *testing.T) {
	bobby := newTestWallet(t)
	cb := ledger.NewCoinbaseTx(bobby.PublicKey(), 100)
	if err := cb.Sign(bobby); err == nil {
		t.Fatal("signing a coinbase should fail, got nil")
	}
}

func TestVerifyMissingSignature(t *testing.T) {
	alice := newTestWallet(t)
	bobby := newTestWallet(t)

	// A non-coinbase transaction that was never signed.
	tx := ledger.NewTransaction(alice.PublicKey(), bobby.PublicKey(), 5)
	if err := tx.Verify(); err == nil {
		t.Fatal("Verify should fail for an unsigned non-coinbase tx, got nil")
	}
}

func TestVerifyMalformedFromFails(t *testing.T) {
	bobby := newTestWallet(t)

	// From is not a valid public key, but a signature is present so we get past
	// the missing-signature check and into public-key reconstruction.
	tx := &ledger.Transaction{
		From:      []byte("not a real public key"),
		To:        bobby.PublicKey(),
		Amount:    5,
		Signature: []byte("some bytes"),
	}
	if err := tx.Verify(); err == nil {
		t.Fatal("Verify should fail when From is not a valid public key, got nil")
	}
}
