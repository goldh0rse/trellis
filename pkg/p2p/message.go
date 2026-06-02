// Package p2p is the networking adapter: it lets multiple nodes stay in sync over
// TCP using gob-encoded messages. It is the only package that imports net, and —
// like pkg/storage for disk — it owns a serialization format (gob on the wire).
// The domain packages (pqsig/wallet/ledger) stay free of net and gob.
//
// Consensus is sync-by-extension: a fresh node downloads a peer's chain
// genesis→tip, then accepts broadcast blocks that validly extend its tip. It
// never replaces its chain with a shorter or competing one.
package p2p

import (
	"bytes"
	"encoding/gob"

	"github.com/goldh0rse/trellis/pkg/ledger"
)

// Message type tags.
const (
	TypeVersion   = "version"
	TypeGetBlocks = "getblocks"
	TypeInv       = "inv"
	TypeGetData   = "getdata"
	TypeBlock     = "block"
	TypeTx        = "tx"
)

// Message is the wire envelope. Body is the gob encoding of one of the payload
// structs below; Type selects which.
type Message struct {
	Type string
	Body []byte
}

// Version announces a node's chain height so peers can decide who is ahead.
type Version struct {
	Height   int
	AddrFrom string
}

// GetBlocks asks a peer for its block-hash inventory.
type GetBlocks struct {
	AddrFrom string
}

// Inv advertises block hashes in genesis→tip order.
type Inv struct {
	AddrFrom string
	Hashes   [][]byte
}

// GetData requests a single block by hash.
type GetData struct {
	AddrFrom string
	Hash     []byte
}

// BlockMsg carries one block.
type BlockMsg struct {
	Block *ledger.Block
}

// TxMsg carries one transaction.
type TxMsg struct {
	Tx *ledger.Transaction
}

// encodeMessage wraps a typed payload in a Message envelope. No gob.Register is
// needed: every payload is a concrete type with no interface-typed fields.
func encodeMessage(typ string, payload any) (Message, error) {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(payload); err != nil {
		return Message{}, err
	}
	return Message{Type: typ, Body: buf.Bytes()}, nil
}

// decodeBody decodes a Message's Body into payload (a pointer to the matching type).
func decodeBody(m Message, payload any) error {
	return gob.NewDecoder(bytes.NewReader(m.Body)).Decode(payload)
}
