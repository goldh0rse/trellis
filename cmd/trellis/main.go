// Command trellis is the Phase 1 demo for the post-quantum blockchain: it
// builds a chain, validates it, tampers with a transaction, and shows that the
// tamper is rejected by ML-DSA signature verification.
package main

import (
	"fmt"
	"log"

	bc "github.com/goldh0rse/trellis/pkg/blockchain"
)

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

	// Genesis issues 100 coins to Alice via a signature-free coinbase.
	chain := bc.NewBlockchain(alice.PublicKey(), 100)

	// Alice sends 30 to Bob and signs the transfer.
	tx := bc.NewTransaction(alice.PublicKey(), bobby.PublicKey(), 30)
	if err := tx.Sign(alice); err != nil {
		log.Fatalf("sign transaction: %v", err)
	}
	if _, err := chain.AddBlock([]*bc.Transaction{tx}); err != nil {
		log.Fatalf("add block: %v", err)
	}
	fmt.Printf("added block with tx %s (alice -> bobby : 30)\n", bc.Short(tx.ID))

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
