package ledger

import "encoding/hex"

// Short returns a truncated hex string (up to 8 chars) for compact display of
// hashes and transaction IDs, which are far too long to print in full.
func Short(b []byte) string {
	s := hex.EncodeToString(b)
	if len(s) > 8 {
		return s[:8]
	}
	return s
}
