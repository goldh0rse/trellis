// Command trellis is the demo for the post-quantum blockchain. It opens a
// bbolt-backed store, creates the chain on first run (mining a genesis and a
// signed transfer), and on every run prints the chain's height and validity —
// demonstrating that the chain survives restarts.
package main

import (
	"fmt"
	"log"
	"time"

	"github.com/goldh0rse/trellis/pkg/ledger"
	"github.com/goldh0rse/trellis/pkg/storage"
	"github.com/goldh0rse/trellis/pkg/wallet"
)

// difficulty is the Proof-of-Work target: mined block hashes must start with this
// many zero hex digits. Kept low so the demo stays effectively instant.
const difficulty = 4

// dbPath is where the chain is persisted (git-ignored).
const dbPath = "trellis.db"

func main() {
	store, err := storage.NewBolt(dbPath)
	if err != nil {
		log.Fatalf("open store: %v", err)
	}
	defer store.Close()

	// Detect a fresh database before NewChain (which seeds genesis transparently).
	tip, err := store.Tip()
	if err != nil {
		log.Fatalf("read tip: %v", err)
	}
	fresh := len(tip) == 0

	// Wallets are ephemeral until Phase 4, so they are regenerated each run. On a
	// fresh DB we use alice as the genesis beneficiary and the sender.
	alice, err := wallet.NewWallet()
	if err != nil {
		log.Fatalf("create wallet: %v", err)
	}

	chain, err := ledger.NewChain(store, alice.PublicKey(), 100, difficulty)
	if err != nil {
		log.Fatalf("open chain: %v", err)
	}

	if fresh {
		fmt.Printf("fresh database — mined genesis (difficulty %d)\n", difficulty)
		bobby, err := wallet.NewWallet()
		if err != nil {
			log.Fatalf("create wallet: %v", err)
		}
		tx := ledger.NewTransaction(alice.PublicKey(), bobby.PublicKey(), 30)
		if err := tx.Sign(alice); err != nil {
			log.Fatalf("sign transaction: %v", err)
		}
		start := time.Now()
		block, attempts, err := chain.AddBlock([]*ledger.Transaction{tx})
		if err != nil {
			log.Fatalf("add block: %v", err)
		}
		fmt.Printf("mined block %s nonce %d (%d attempts in %s) — alice -> bobby : 30\n",
			ledger.Short(block.Hash), block.Nonce, attempts, time.Since(start).Round(time.Microsecond))
	} else {
		fmt.Printf("loaded existing chain from %s\n", dbPath)
	}

	height, err := chain.Height()
	if err != nil {
		log.Fatalf("height: %v", err)
	}
	tip, err = chain.Tip()
	if err != nil {
		log.Fatalf("tip: %v", err)
	}
	fmt.Printf("height: %d  tip: %s\n", height, ledger.Short(tip))
	fmt.Printf("Chain valid? %v\n", chain.IsValid() == nil)
	fmt.Printf("(run again to confirm the chain persists across restarts)\n")
}
