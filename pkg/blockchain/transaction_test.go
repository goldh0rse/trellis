package blockchain

import "testing"

// newTestWallet fails the test/benchmark rather than returning an error,
// keeping the bodies focused on behaviour. It takes testing.TB so both tests
// and benchmarks can use it.
func newTestWallet(tb testing.TB) *Wallet {
	tb.Helper()
	w, err := NewWallet()
	if err != nil {
		tb.Fatalf("NewWallet: %v", err)
	}
	return w
}

func TestTransactionSignVerifyRoundTrip(t *testing.T) {
	alice := newTestWallet(t)
	bobby := newTestWallet(t)

	tx := NewTransaction(alice.PublicKey(), bobby.PublicKey(), 42)
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

	tx := NewTransaction(alice.PublicKey(), bobby.PublicKey(), 42)
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
	tx := NewTransaction(alice.PublicKey(), bobby.PublicKey(), 42)
	if err := tx.Sign(mallory); err == nil {
		t.Fatal("signing with a wallet that does not own From should fail, got nil")
	}
}

func TestCoinbaseVerify(t *testing.T) {
	bobby := newTestWallet(t)

	cb := NewCoinbaseTx(bobby.PublicKey(), 100)
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
	cb := NewCoinbaseTx(bobby.PublicKey(), 100)
	if err := cb.Sign(bobby); err == nil {
		t.Fatal("signing a coinbase should fail, got nil")
	}
}

// BenchmarkSignTransaction measures producing one ML-DSA-44 signature over a
// transaction's signing payload.
func BenchmarkSignTransaction(b *testing.B) {
	alice := newTestWallet(b)
	bobby := newTestWallet(b)
	tx := NewTransaction(alice.PublicKey(), bobby.PublicKey(), 42)

	b.ReportAllocs()
	for b.Loop() {
		if err := tx.Sign(alice); err != nil {
			b.Fatalf("Sign: %v", err)
		}
	}
}

// BenchmarkVerifyTransaction measures verifying one signed transaction. This is
// the per-transaction hot path inside Blockchain.IsValid: it reconstructs the
// public key from raw bytes and runs ML-DSA verification.
func BenchmarkVerifyTransaction(b *testing.B) {
	alice := newTestWallet(b)
	bobby := newTestWallet(b)
	tx := NewTransaction(alice.PublicKey(), bobby.PublicKey(), 42)
	if err := tx.Sign(alice); err != nil {
		b.Fatalf("Sign: %v", err)
	}

	b.ReportAllocs()
	for b.Loop() {
		if err := tx.Verify(); err != nil {
			b.Fatalf("Verify: %v", err)
		}
	}
}
