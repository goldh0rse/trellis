package p2p_test

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/goldh0rse/trellis/pkg/ledger"
	"github.com/goldh0rse/trellis/pkg/p2p"
	"github.com/goldh0rse/trellis/pkg/storage"
	"github.com/goldh0rse/trellis/pkg/wallet"
)

const testDifficulty = 1 // exercises the PoW path but stays near-instant

func newWallet(t *testing.T) *wallet.Wallet {
	t.Helper()
	w, err := wallet.NewWallet()
	if err != nil {
		t.Fatalf("NewWallet: %v", err)
	}
	return w
}

// startNode runs a node on an ephemeral port and waits until its listener is bound.
func startNode(t *testing.T, cfg p2p.NodeConfig) *p2p.Node {
	t.Helper()
	cfg.Addr = "127.0.0.1:0"
	cfg.Difficulty = testDifficulty
	n := p2p.NewNode(cfg)
	go func() { _ = n.Run() }()
	t.Cleanup(func() { n.Close() })

	// Wait for the listener to bind (Addr stops ending in ":0").
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if a := n.Addr(); !strings.HasSuffix(a, ":0") {
			return n
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("node did not bind in time")
	return nil
}

// waitFor polls cond until it is true or the deadline passes.
func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

func TestTwoNodesSyncAndPropagate(t *testing.T) {
	w := newWallet(t) // funded by genesis
	x := newWallet(t) // recipient

	// Node A: miner with a funded chain (genesis -> W : 100).
	storeA := storage.NewMemStore()
	chainA, err := ledger.NewChain(storeA, w.PublicKey(), 100, testDifficulty)
	if err != nil {
		t.Fatalf("NewChain: %v", err)
	}
	nodeA := startNode(t, p2p.NodeConfig{Chain: chainA, Mempool: ledger.NewMempool(), Mining: true})

	// Node B: empty, syncs from A.
	storeB := storage.NewMemStore()
	chainB, err := ledger.LoadChain(storeB, testDifficulty)
	if err != nil {
		t.Fatalf("LoadChain: %v", err)
	}
	nodeB := startNode(t, p2p.NodeConfig{Chain: chainB, Mempool: ledger.NewMempool(), Seed: nodeA.Addr()})

	// B downloads A's chain (genesis).
	waitFor(t, "B to sync genesis", func() bool { return nodeB.Height() == 1 })
	if !bytes.Equal(nodeA.Tip(), nodeB.Tip()) {
		t.Fatal("tips differ after initial sync")
	}

	// Submit a tx at B; it must propagate to the miner A, get mined, and come back.
	tx := ledger.NewTransaction(w.PublicKey(), x.PublicKey(), 30)
	if err := tx.Sign(w); err != nil {
		t.Fatalf("Sign: %v", err)
	}
	if err := p2p.SendTx(nodeB.Addr(), tx); err != nil {
		t.Fatalf("SendTx: %v", err)
	}

	// Both nodes reach height 2 with identical tips.
	waitFor(t, "both nodes to reach height 2", func() bool {
		return nodeA.Height() == 2 && nodeB.Height() == 2 && bytes.Equal(nodeA.Tip(), nodeB.Tip())
	})

	if !nodeA.Valid() || !nodeB.Valid() {
		t.Fatal("a node reports an invalid chain after sync")
	}
}
