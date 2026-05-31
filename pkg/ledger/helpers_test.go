package ledger_test

import (
	"testing"

	"github.com/goldh0rse/trellis/pkg/wallet"
)

// newTestWallet builds a wallet for use as a transaction Signer in tests. It
// lives in the external test package because the ledger itself does not depend
// on wallet — only its tests do.
func newTestWallet(tb testing.TB) *wallet.Wallet {
	tb.Helper()
	w, err := wallet.NewWallet()
	if err != nil {
		tb.Fatalf("NewWallet: %v", err)
	}
	return w
}
