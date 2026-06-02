package p2p

import (
	"bytes"
	"encoding/hex"
	"errors"
	"log"

	"github.com/goldh0rse/trellis/pkg/ledger"
)

// handleVersion records the peer and compares heights. If the peer is ahead we
// ask for its blocks; if we are ahead we announce back so it syncs from us.
func (n *Node) handleVersion(m Message) error {
	var v Version
	if err := decodeBody(m, &v); err != nil {
		return err
	}
	n.mu.Lock()
	n.addPeerLocked(v.AddrFrom)
	myHeight, err := n.chain.Height()
	n.mu.Unlock()
	if err != nil {
		return err
	}

	switch {
	case v.Height > myHeight:
		return sendPayload(v.AddrFrom, TypeGetBlocks, GetBlocks{AddrFrom: n.addr})
	case myHeight > v.Height:
		return sendPayload(v.AddrFrom, TypeVersion, Version{Height: myHeight, AddrFrom: n.addr})
	default:
		return nil
	}
}

// handleGetBlocks replies with our block-hash inventory (genesis→tip).
func (n *Node) handleGetBlocks(m Message) error {
	var gb GetBlocks
	if err := decodeBody(m, &gb); err != nil {
		return err
	}
	n.mu.Lock()
	n.addPeerLocked(gb.AddrFrom)
	hashes, err := n.chain.BlockHashes()
	n.mu.Unlock()
	if err != nil {
		return err
	}
	return sendPayload(gb.AddrFrom, TypeInv, Inv{AddrFrom: n.addr, Hashes: hashes})
}

// handleInv queues the hashes we are missing (in order) and requests the first.
// Subsequent blocks are pulled one at a time as each arrives (handleBlock), which
// guarantees blocks are applied genesis-first.
func (n *Node) handleInv(m Message) error {
	var inv Inv
	if err := decodeBody(m, &inv); err != nil {
		return err
	}

	n.mu.Lock()
	n.addPeerLocked(inv.AddrFrom)
	var missing [][]byte
	for _, h := range inv.Hashes {
		if _, err := n.chain.Block(h); errors.Is(err, ledger.ErrBlockNotFound) {
			missing = append(missing, h)
		}
	}
	n.wanted = missing
	n.syncPeer = inv.AddrFrom
	var next []byte
	if len(missing) > 0 {
		next = missing[0]
	}
	n.mu.Unlock()

	if next == nil {
		return nil
	}
	return sendPayload(inv.AddrFrom, TypeGetData, GetData{AddrFrom: n.addr, Hash: next})
}

// handleGetData answers a block request.
func (n *Node) handleGetData(m Message) error {
	var gd GetData
	if err := decodeBody(m, &gd); err != nil {
		return err
	}
	n.mu.Lock()
	n.addPeerLocked(gd.AddrFrom)
	b, err := n.chain.Block(gd.Hash)
	n.mu.Unlock()
	if err != nil {
		return err // ErrBlockNotFound: we don't have it
	}
	return sendPayload(gd.AddrFrom, TypeBlock, BlockMsg{Block: b})
}

// handleBlock validates and accepts a block. If it was part of an in-progress
// download we pull the next queued block; otherwise (a freshly mined block) we
// propagate its inventory to our other peers.
func (n *Node) handleBlock(m Message) error {
	var bm BlockMsg
	if err := decodeBody(m, &bm); err != nil {
		return err
	}
	b := bm.Block

	n.mu.Lock()
	err := n.chain.AcceptBlock(b)
	if err != nil {
		n.mu.Unlock()
		if errors.Is(err, ledger.ErrNotExtending) {
			log.Printf("p2p: dropping non-extending block %s", ledger.Short(b.Hash))
			return nil
		}
		return err
	}
	// Was this the head of our download queue?
	queued := len(n.wanted) > 0 && bytes.Equal(n.wanted[0], b.Hash)
	var next []byte
	syncPeer := n.syncPeer
	if queued {
		n.wanted = n.wanted[1:]
		if len(n.wanted) > 0 {
			next = n.wanted[0]
		}
	}
	n.mu.Unlock()

	if queued {
		if next != nil {
			return sendPayload(syncPeer, TypeGetData, GetData{AddrFrom: n.addr, Hash: next})
		}
		return nil
	}

	// A live block: tell our peers about it.
	n.broadcastInv(b.Hash, "")
	return nil
}

// handleTx validates an inbound transaction, adds it to the mempool, propagates
// it, and — if this node mines — mines a block carrying the pending transactions.
func (n *Node) handleTx(m Message) error {
	var tm TxMsg
	if err := decodeBody(m, &tm); err != nil {
		return err
	}
	tx := tm.Tx

	n.mu.Lock()
	key := hex.EncodeToString(tx.ID)
	if n.seen[key] {
		n.mu.Unlock()
		return nil // already processed; stop the propagation loop
	}
	n.seen[key] = true

	// Validate affordability against confirmed chain state before accepting.
	if !tx.IsCoinbase() {
		bal, err := n.chain.Balance(tx.From)
		if err != nil {
			n.mu.Unlock()
			return err
		}
		if tx.Amount > bal {
			n.mu.Unlock()
			log.Printf("p2p: dropping unaffordable tx %s (balance %d < %d)", ledger.Short(tx.ID), bal, tx.Amount)
			return nil
		}
	}
	if err := n.mempool.Add(tx); err != nil { // re-verifies the signature
		n.mu.Unlock()
		return err
	}
	n.mu.Unlock()

	// Propagate to peers (no AddrFrom on a tx, so send to all; dedup via `seen`).
	for _, p := range n.peerList("") {
		if err := sendPayload(p, TypeTx, TxMsg{Tx: tx}); err != nil {
			log.Printf("p2p: propagate tx to %s: %v", p, err)
		}
	}

	if n.mining {
		n.minePending()
	}
	return nil
}

// minePending mines a block from the current mempool (if non-empty) and, on
// success, clears the mempool and broadcasts the new block's inventory. The
// expensive Mine call runs with the lock released.
func (n *Node) minePending() {
	n.mu.Lock()
	txs := n.mempool.Transactions()
	if len(txs) == 0 {
		n.mu.Unlock()
		return
	}
	tip, err := n.chain.Tip()
	n.mu.Unlock()
	if err != nil {
		log.Printf("p2p: mine: read tip: %v", err)
		return
	}

	block := ledger.NewBlock(txs, tip)
	block.Mine(n.difficulty) // CPU work, lock released

	n.mu.Lock()
	err = n.chain.AcceptBlock(block)
	if err == nil {
		n.mempool.Clear()
	}
	n.mu.Unlock()
	if err != nil {
		log.Printf("p2p: mine: accept own block: %v", err) // e.g. tip advanced; txs stay pending
		return
	}
	n.broadcastInv(block.Hash, "")
}

// broadcastInv advertises a single block hash to all peers except `except`.
func (n *Node) broadcastInv(hash []byte, except string) {
	for _, p := range n.peerList(except) {
		if err := sendPayload(p, TypeInv, Inv{AddrFrom: n.addr, Hashes: [][]byte{hash}}); err != nil {
			log.Printf("p2p: broadcast inv to %s: %v", p, err)
		}
	}
}
