package ledger

import (
	"bytes"
	"fmt"
)

// Balance returns the spendable balance of pubKey under the account model, by
// replaying every confirmed transaction: +Amount when the key is the recipient
// (To), -Amount when it is the sender (From). A coinbase transaction has an empty
// From, which never equals a real 1312-byte public key, so coinbase only ever
// credits.
//
// A signed accumulator is used internally so partial sums can dip; a final
// negative total means the store is corrupt (a valid chain only grows through
// AddBlock after a balance check), and is reported as an error.
func (c *Chain) Balance(pubKey []byte) (uint64, error) {
	it, err := c.Iterator()
	if err != nil {
		return 0, err
	}
	var bal int64
	for {
		b, err := it.Next()
		if err != nil {
			return 0, err
		}
		if b == nil {
			break
		}
		for _, tx := range b.Transactions {
			if bytes.Equal(tx.To, pubKey) {
				bal += int64(tx.Amount)
			}
			if bytes.Equal(tx.From, pubKey) {
				bal -= int64(tx.Amount)
			}
		}
	}
	if bal < 0 {
		return 0, fmt.Errorf("negative balance for %s (corrupt chain)", Short(pubKey))
	}
	return uint64(bal), nil
}
