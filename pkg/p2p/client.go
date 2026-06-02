package p2p

import (
	"encoding/gob"
	"fmt"
	"net"
	"time"

	"github.com/goldh0rse/trellis/pkg/ledger"
)

// dialTimeout bounds how long an outbound dial waits — keeps a dead peer from
// blocking a handler.
const dialTimeout = 3 * time.Second

// sendMessage dials addr, sends exactly one Message, and closes the connection.
// One message per connection keeps framing trivial: gob over TCP self-delimits,
// so even a multi-kilobyte block is read by a single Decode regardless of how the
// bytes are split across TCP segments.
func sendMessage(addr string, m Message) error {
	conn, err := net.DialTimeout("tcp", addr, dialTimeout)
	if err != nil {
		return err
	}
	defer conn.Close()
	return gob.NewEncoder(conn).Encode(m)
}

// sendPayload encodes a typed payload and sends it to addr.
func sendPayload(addr, typ string, payload any) error {
	m, err := encodeMessage(typ, payload)
	if err != nil {
		return err
	}
	return sendMessage(addr, m)
}

// SendTx submits a signed transaction to a running node. The CLI uses this so it
// never has to import net or gob itself.
func SendTx(addr string, tx *ledger.Transaction) error {
	if err := sendPayload(addr, TypeTx, TxMsg{Tx: tx}); err != nil {
		return fmt.Errorf("send tx to %s: %w", addr, err)
	}
	return nil
}
