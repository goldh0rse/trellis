package p2p

import (
	"bytes"
	"reflect"
	"testing"

	"github.com/goldh0rse/trellis/pkg/ledger"
	"github.com/goldh0rse/trellis/pkg/wallet"
)

// minedBlockWithTx builds a real mined block carrying one signed transaction —
// exercising the large (~2.4 KB signature, 1312-byte pubkey) payloads on the wire.
func minedBlockWithTx(t *testing.T) (*ledger.Block, *ledger.Transaction) {
	t.Helper()
	alice, err := wallet.NewWallet()
	if err != nil {
		t.Fatalf("NewWallet: %v", err)
	}
	bobby, err := wallet.NewWallet()
	if err != nil {
		t.Fatalf("NewWallet: %v", err)
	}
	tx := ledger.NewTransaction(alice.PublicKey(), bobby.PublicKey(), 7)
	if err := tx.Sign(alice); err != nil {
		t.Fatalf("Sign: %v", err)
	}
	b := ledger.NewBlock([]*ledger.Transaction{tx}, []byte{0x01})
	b.Mine(1)
	return b, tx
}

func TestMessageRoundTripSimplePayloads(t *testing.T) {
	cases := []struct {
		typ     string
		payload any
		out     any
	}{
		{TypeVersion, Version{Height: 5, AddrFrom: "a"}, &Version{}},
		{TypeGetBlocks, GetBlocks{AddrFrom: "a"}, &GetBlocks{}},
		{TypeInv, Inv{AddrFrom: "a", Hashes: [][]byte{{1, 2}, {3, 4}}}, &Inv{}},
		{TypeGetData, GetData{AddrFrom: "a", Hash: []byte{9}}, &GetData{}},
	}
	for _, tc := range cases {
		t.Run(tc.typ, func(t *testing.T) {
			m, err := encodeMessage(tc.typ, tc.payload)
			if err != nil {
				t.Fatalf("encode: %v", err)
			}
			if m.Type != tc.typ {
				t.Fatalf("Type = %q, want %q", m.Type, tc.typ)
			}
			if err := decodeBody(m, tc.out); err != nil {
				t.Fatalf("decode: %v", err)
			}
			got := reflect.ValueOf(tc.out).Elem().Interface()
			if !reflect.DeepEqual(got, tc.payload) {
				t.Fatalf("round-trip mismatch: got %+v, want %+v", got, tc.payload)
			}
		})
	}
}

func TestMessageRoundTripBlockAndTx(t *testing.T) {
	block, tx := minedBlockWithTx(t)

	// BlockMsg: the full block (including the ~2.4 KB signature) survives gob.
	m, err := encodeMessage(TypeBlock, BlockMsg{Block: block})
	if err != nil {
		t.Fatalf("encode block: %v", err)
	}
	var bm BlockMsg
	if err := decodeBody(m, &bm); err != nil {
		t.Fatalf("decode block: %v", err)
	}
	if !bytes.Equal(bm.Block.Hash, block.Hash) || bm.Block.Nonce != block.Nonce {
		t.Fatal("block hash/nonce did not survive round trip")
	}
	if !bytes.Equal(bm.Block.Transactions[0].Signature, tx.Signature) {
		t.Fatal("block's tx signature did not survive round trip")
	}
	// The decoded block must still validate against itself.
	if !bytes.Equal(bm.Block.Hash, bm.Block.ComputeHash()) {
		t.Fatal("decoded block fails its own ComputeHash")
	}

	// TxMsg.
	m2, err := encodeMessage(TypeTx, TxMsg{Tx: tx})
	if err != nil {
		t.Fatalf("encode tx: %v", err)
	}
	var txm TxMsg
	if err := decodeBody(m2, &txm); err != nil {
		t.Fatalf("decode tx: %v", err)
	}
	if !bytes.Equal(txm.Tx.ID, tx.ID) || !bytes.Equal(txm.Tx.Signature, tx.Signature) {
		t.Fatal("tx did not survive round trip")
	}
	if err := txm.Tx.Verify(); err != nil {
		t.Fatalf("round-tripped tx should still verify, got: %v", err)
	}
}
