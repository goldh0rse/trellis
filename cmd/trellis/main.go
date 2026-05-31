// Command trellis is the Phase 1 demo for the post-quantum blockchain: it
// builds a chain, validates it, tampers with a transaction, and shows that the
// tamper is rejected by ML-DSA signature verification.
package main

import (
	"fmt"
	"log"
	"time"

	bc "github.com/goldh0rse/trellis/pkg/blockchain"
)

// difficulty is the Proof-of-Work target: mined block hashes must start with
// this many zero hex digits. Kept low so the demo stays effectively instant.
const difficulty = 4

func main() {
	// Two actors, each with a post-quantum (ML-DSA-44) keypair.
	alice, err := bc.NewWallet()
	if err != nil {
		log.Fatalf("create alice wallet: %v", err)
	}
	bobby, err := bc.NewWallet()
	if err != nil {
		log.Fatalf("create bobby wallet: %v", err)
	}
	fmt.Printf("alice address: %s\n", alice.Address())
	fmt.Printf("bobby address: %s\n", bobby.Address())

	// Genesis issues 100 coins to Alice via a signature-free coinbase. It is
	// mined to the target difficulty like every other block.
	fmt.Printf("mining genesis (difficulty %d)...\n", difficulty)
	chain := bc.NewBlockchain(alice.PublicKey(), 100, difficulty)
	genesis := chain.Blocks[0]
	fmt.Printf("genesis mined: hash %s nonce %d\n", bc.Short(genesis.Hash), genesis.Nonce)

	// Alice sends 30 to Bob and signs the transfer.
	tx := bc.NewTransaction(alice.PublicKey(), bobby.PublicKey(), 30)
	if err := tx.Sign(alice); err != nil {
		log.Fatalf("sign transaction: %v", err)
	}

	start := time.Now()
	block, attempts, err := chain.AddBlock([]*bc.Transaction{tx})
	if err != nil {
		log.Fatalf("add block: %v", err)
	}
	fmt.Printf("mined block: hash %s nonce %d (%d attempts in %s) tx %s (alice -> bobby : 30)\n",
		bc.Short(block.Hash), block.Nonce, attempts, time.Since(start).Round(time.Microsecond), bc.Short(tx.ID))

	fmt.Printf("Chain valid? %v\n", chain.IsValid() == nil)

	// --- Tamper ---
	// Mutate the transfer's Amount in place (30 -> 31) without re-signing. The
	// cached tx.ID is unchanged, so the block hash still matches; the forgery is
	// caught by signature verification, because the signature covers 30 while the
	// live transaction now claims 31. This is the whole point of the project: a
	// forged transfer is rejected by the post-quantum signature.
	chain.Blocks[1].Transactions[0].Amount = 31

	fmt.Printf("Chain valid? %v\n", chain.IsValid() == nil)
	if err := chain.IsValid(); err != nil {
		fmt.Printf("rejected because: %v\n", err)
	}
}
