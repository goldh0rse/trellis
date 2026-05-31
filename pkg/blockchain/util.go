package blockchain

import "encoding/hex"

// Short returns a truncated hex string (up to 8 chars) for compact display of
// hashes, public keys and signatures — all of which are far too long to print
// in full.
func Short(b []byte) string {
	s := hex.EncodeToString(b)
	if len(s) > 8 {
		return s[:8]
	}
	return s
}
