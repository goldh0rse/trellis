package blockchain

import "testing"

// BenchmarkNewWallet measures ML-DSA-44 key generation — the most expensive
// wallet operation, and notably heavier than classical (e.g. ECDSA) keygen.
func BenchmarkNewWallet(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		if _, err := NewWallet(); err != nil {
			b.Fatalf("NewWallet: %v", err)
		}
	}
}
