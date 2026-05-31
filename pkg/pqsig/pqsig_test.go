package pqsig

import (
	"bytes"
	"testing"
)

func newTestKey(tb testing.TB) *PrivateKey {
	tb.Helper()
	k, err := GenerateKey()
	if err != nil {
		tb.Fatalf("GenerateKey: %v", err)
	}
	return k
}

func TestSignVerifyRoundTrip(t *testing.T) {
	k := newTestKey(t)
	msg := []byte("post-quantum hello")

	sig, err := k.Sign(msg)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	if err := Verify(k.PublicKey(), msg, sig); err != nil {
		t.Fatalf("Verify of a valid signature should pass, got: %v", err)
	}
}

func TestVerifyTamperedMessageFails(t *testing.T) {
	k := newTestKey(t)
	msg := []byte("pay alice 10")

	sig, err := k.Sign(msg)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	if err := Verify(k.PublicKey(), []byte("pay alice 11"), sig); err == nil {
		t.Fatal("Verify should fail for a tampered message, got nil")
	}
}

func TestVerifyMalformedPublicKeyFails(t *testing.T) {
	k := newTestKey(t)
	msg := []byte("hello")
	sig, err := k.Sign(msg)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	if err := Verify([]byte("not a real public key"), msg, sig); err == nil {
		t.Fatal("Verify should fail for a malformed public key, got nil")
	}
}

func TestVerifyBadSignatureFails(t *testing.T) {
	k := newTestKey(t)
	msg := []byte("hello")
	if err := Verify(k.PublicKey(), msg, []byte("garbage signature")); err == nil {
		t.Fatal("Verify should fail for a bad signature, got nil")
	}
}

func TestSeedRoundTrip(t *testing.T) {
	k := newTestKey(t)
	seed := k.Seed()
	if len(seed) != 32 {
		t.Fatalf("Seed length = %d, want 32", len(seed))
	}

	restored, err := NewPrivateKeyFromSeed(seed)
	if err != nil {
		t.Fatalf("NewPrivateKeyFromSeed: %v", err)
	}
	// The restored key has the same public key...
	if !bytes.Equal(restored.PublicKey(), k.PublicKey()) {
		t.Fatal("restored key has a different public key")
	}
	// ...and can produce signatures the original's public key verifies.
	msg := []byte("restored signer")
	sig, err := restored.Sign(msg)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	if err := Verify(k.PublicKey(), msg, sig); err != nil {
		t.Fatalf("signature from restored key should verify, got: %v", err)
	}
}

func TestNewPrivateKeyFromSeedWrongLength(t *testing.T) {
	if _, err := NewPrivateKeyFromSeed([]byte("too short")); err == nil {
		t.Fatal("NewPrivateKeyFromSeed should reject a wrong-length seed, got nil")
	}
}

// BenchmarkGenerateKey measures ML-DSA-44 key generation — notably heavier than
// classical (e.g. ECDSA) keygen.
func BenchmarkGenerateKey(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		if _, err := GenerateKey(); err != nil {
			b.Fatalf("GenerateKey: %v", err)
		}
	}
}

// BenchmarkSign measures producing one ML-DSA-44 signature.
func BenchmarkSign(b *testing.B) {
	k := newTestKey(b)
	msg := []byte("benchmark message")
	b.ReportAllocs()
	for b.Loop() {
		if _, err := k.Sign(msg); err != nil {
			b.Fatalf("Sign: %v", err)
		}
	}
}

// BenchmarkVerify measures verifying one ML-DSA-44 signature — the per-transaction
// hot path during chain validation, and far costlier than SHA-256 hashing.
func BenchmarkVerify(b *testing.B) {
	k := newTestKey(b)
	msg := []byte("benchmark message")
	sig, err := k.Sign(msg)
	if err != nil {
		b.Fatalf("Sign: %v", err)
	}
	pub := k.PublicKey()
	if !bytes.Equal(pub, k.PublicKey()) { // sanity: PublicKey is stable
		b.Fatal("PublicKey not stable")
	}
	b.ReportAllocs()
	for b.Loop() {
		if err := Verify(pub, msg, sig); err != nil {
			b.Fatalf("Verify: %v", err)
		}
	}
}
