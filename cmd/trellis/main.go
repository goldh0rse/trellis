// Command trellis is the command-line interface to the post-quantum blockchain.
//
// Usage:
//
//	trellis createwallet
//	trellis listaddresses
//	trellis createblockchain -address ADDR
//	trellis getbalance -address ADDR
//	trellis send -from ADDR -to ADDR -amount N
//	trellis printchain
//
// Wallets are persisted (unencrypted — learning only) to wallets.dat; the chain
// to trellis.db. This is a single-node CLI: send only works between wallets held
// in the local keyring.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/goldh0rse/trellis/pkg/ledger"
	"github.com/goldh0rse/trellis/pkg/p2p"
	"github.com/goldh0rse/trellis/pkg/storage"
	"github.com/goldh0rse/trellis/pkg/wallet"
)

const (
	difficulty = 3                // Proof-of-Work target (leading hex zeros); low for a snappy CLI
	reward     = 100              // genesis coinbase reward
	seedNode   = "localhost:3000" // the central node every other node syncs from
)

// config holds the file locations, so tests can sandbox them in a temp dir.
type config struct {
	dbPath     string
	walletPath string
}

func main() {
	cfg := config{dbPath: "trellis.db", walletPath: "wallets.dat"}
	if err := run(os.Args[1:], cfg, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(args []string, cfg config, out io.Writer) error {
	if len(args) == 0 {
		return usageError()
	}
	cmd, rest := args[0], args[1:]

	keyring, err := wallet.OpenKeyring(storage.NewKeyFile(cfg.walletPath))
	if err != nil {
		return fmt.Errorf("open keyring: %w", err)
	}

	switch cmd {
	case "createwallet":
		return cmdCreateWallet(keyring, out)
	case "listaddresses":
		return cmdListAddresses(keyring, out)
	case "createblockchain":
		return cmdCreateBlockchain(rest, cfg, keyring, out)
	case "getbalance":
		return cmdGetBalance(rest, cfg, keyring, out)
	case "send":
		return cmdSend(rest, cfg, keyring, out)
	case "printchain":
		return cmdPrintChain(rest, cfg, out)
	case "startnode":
		return cmdStartNode(rest, cfg, keyring, out)
	default:
		return fmt.Errorf("unknown command %q", cmd)
	}
}

func usageError() error {
	return fmt.Errorf("usage: trellis <createwallet|listaddresses|createblockchain|getbalance|send|printchain|startnode> [flags]")
}

// --- keyring-only commands ---

func cmdCreateWallet(keyring *wallet.Keyring, out io.Writer) error {
	_, addr, err := keyring.CreateWallet()
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "new wallet address: %s\n", addr)
	return nil
}

func cmdListAddresses(keyring *wallet.Keyring, out io.Writer) error {
	addrs := keyring.Addresses()
	if len(addrs) == 0 {
		fmt.Fprintln(out, "no wallets yet — run: trellis createwallet")
		return nil
	}
	for _, a := range addrs {
		fmt.Fprintln(out, a)
	}
	return nil
}

// --- chain commands ---

func cmdCreateBlockchain(args []string, cfg config, keyring *wallet.Keyring, out io.Writer) error {
	fs := flag.NewFlagSet("createblockchain", flag.ContinueOnError)
	address := fs.String("address", "", "address to receive the genesis reward")
	db := fs.String("db", "", "chain database path (default: trellis.db)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	w, ok := keyring.Wallet(*address)
	if !ok {
		return fmt.Errorf("unknown address %q (not in keyring)", *address)
	}

	store, err := storage.NewBolt(dbPathOr(*db, cfg))
	if err != nil {
		return err
	}
	defer store.Close()

	tip, err := store.Tip()
	if err != nil {
		return err
	}
	if len(tip) != 0 {
		return fmt.Errorf("blockchain already exists")
	}

	if _, err := ledger.NewChain(store, w.PublicKey(), reward, difficulty); err != nil {
		return err
	}
	fmt.Fprintf(out, "blockchain created — genesis reward of %d to %s\n", reward, *address)
	return nil
}

func cmdGetBalance(args []string, cfg config, keyring *wallet.Keyring, out io.Writer) error {
	fs := flag.NewFlagSet("getbalance", flag.ContinueOnError)
	address := fs.String("address", "", "address to query")
	db := fs.String("db", "", "chain database path (default: trellis.db)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	w, ok := keyring.Wallet(*address)
	if !ok {
		return fmt.Errorf("unknown address %q (not in keyring)", *address)
	}

	chain, store, err := openChain(dbPathOr(*db, cfg), difficulty)
	if err != nil {
		return err
	}
	defer store.Close()

	bal, err := chain.Balance(w.PublicKey())
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "balance of %s: %d\n", *address, bal)
	return nil
}

func cmdSend(args []string, cfg config, keyring *wallet.Keyring, out io.Writer) error {
	fs := flag.NewFlagSet("send", flag.ContinueOnError)
	from := fs.String("from", "", "sender address")
	to := fs.String("to", "", "recipient address")
	amount := fs.Uint64("amount", 0, "amount to send")
	node := fs.String("node", "", "submit the tx to a running node at this address (default: mine locally)")
	db := fs.String("db", "", "chain database path for local mode (default: trellis.db)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *amount == 0 {
		return fmt.Errorf("amount must be > 0")
	}
	fromWallet, ok := keyring.Wallet(*from)
	if !ok {
		return fmt.Errorf("unknown sender address %q (not in keyring)", *from)
	}
	// The recipient's public key must be known: an address is a one-way hash, so
	// even when sending over the network the recipient must be in the shared keyring.
	toWallet, ok := keyring.Wallet(*to)
	if !ok {
		return fmt.Errorf("unknown recipient address %q: send can only target wallets in the keyring", *to)
	}

	// Network mode: sign locally and submit to a node, which validates the balance
	// and (if it mines) confirms the tx in a block.
	if *node != "" {
		tx := ledger.NewTransaction(fromWallet.PublicKey(), toWallet.PublicKey(), *amount)
		if err := tx.Sign(fromWallet); err != nil {
			return err
		}
		if err := p2p.SendTx(*node, tx); err != nil {
			return err
		}
		fmt.Fprintf(out, "submitted tx %s (%d from %s to %s) to node %s\n",
			ledger.Short(tx.ID), *amount, *from, *to, *node)
		return nil
	}

	// Local mode: check balance, sign, and mine the block ourselves.
	chain, store, err := openChain(dbPathOr(*db, cfg), difficulty)
	if err != nil {
		return err
	}
	defer store.Close()

	bal, err := chain.Balance(fromWallet.PublicKey())
	if err != nil {
		return err
	}
	if *amount > bal {
		return fmt.Errorf("insufficient funds: balance %d < amount %d", bal, *amount)
	}

	tx := ledger.NewTransaction(fromWallet.PublicKey(), toWallet.PublicKey(), *amount)
	if err := tx.Sign(fromWallet); err != nil {
		return err
	}
	mp := ledger.NewMempool()
	if err := mp.Add(tx); err != nil {
		return err
	}
	block, attempts, err := chain.AddBlock(mp.Transactions())
	if err != nil {
		return err
	}
	mp.Clear()
	fmt.Fprintf(out, "sent %d from %s to %s\n", *amount, *from, *to)
	fmt.Fprintf(out, "mined block %s (%d attempts)\n", ledger.Short(block.Hash), attempts)
	return nil
}

func cmdPrintChain(args []string, cfg config, out io.Writer) error {
	fs := flag.NewFlagSet("printchain", flag.ContinueOnError)
	db := fs.String("db", "", "chain database path (default: trellis.db)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	chain, store, err := openChain(dbPathOr(*db, cfg), difficulty)
	if err != nil {
		return err
	}
	defer store.Close()

	it, err := chain.Iterator()
	if err != nil {
		return err
	}
	for {
		b, err := it.Next()
		if err != nil {
			return err
		}
		if b == nil {
			break
		}
		fmt.Fprintf(out, "block %s  prev %s  nonce %d\n", ledger.Short(b.Hash), ledger.Short(b.PrevHash), b.Nonce)
		for _, tx := range b.Transactions {
			if tx.IsCoinbase() {
				fmt.Fprintf(out, "  coinbase -> %s : %d\n", ledger.Short(tx.To), tx.Amount)
			} else {
				fmt.Fprintf(out, "  %s -> %s : %d\n", ledger.Short(tx.From), ledger.Short(tx.To), tx.Amount)
			}
		}
	}
	fmt.Fprintf(out, "valid: %v\n", chain.IsValid() == nil)
	return nil
}

func cmdStartNode(args []string, cfg config, keyring *wallet.Keyring, out io.Writer) error {
	fs := flag.NewFlagSet("startnode", flag.ContinueOnError)
	port := fs.String("port", "3000", "TCP port to listen on")
	mine := fs.Bool("mine", false, "mine pending transactions into blocks")
	address := fs.String("address", "", "miner address (optional; validated against the keyring if given)")
	db := fs.String("db", "", "chain database path (default: trellis.db); give each local node its own")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *mine && *address != "" {
		if _, ok := keyring.Wallet(*address); !ok {
			return fmt.Errorf("unknown miner address %q (not in keyring)", *address)
		}
	}

	dbPath := dbPathOr(*db, cfg)
	store, err := storage.NewBolt(dbPath)
	if err != nil {
		return err
	}
	defer store.Close()

	// LoadChain (not NewChain) so a fresh node stays empty and learns genesis from
	// a peer rather than minting a divergent one.
	chain, err := ledger.LoadChain(store, difficulty)
	if err != nil {
		return err
	}

	addr := "localhost:" + *port
	seed := seedNode
	if addr == seedNode {
		seed = "" // this is the seed/central node — it has no upstream peer
	}

	node := p2p.NewNode(p2p.NodeConfig{
		Addr:       addr,
		Seed:       seed,
		Chain:      chain,
		Mempool:    ledger.NewMempool(),
		Mining:     *mine,
		Difficulty: difficulty,
	})
	fmt.Fprintf(out, "node listening on %s (mining=%v, db=%s)\n", addr, *mine, dbPath)
	return node.Run()
}

// dbPathOr returns the -db flag value if set, otherwise the config default. It
// lets every chain command target a specific database file (e.g. a second node's).
func dbPathOr(flagVal string, cfg config) string {
	if flagVal != "" {
		return flagVal
	}
	return cfg.dbPath
}

// openChain opens the store at path and loads the existing chain, erroring if the
// chain has not been initialized yet. The caller must Close the returned store.
func openChain(path string, difficulty int) (*ledger.Chain, *storage.Bolt, error) {
	store, err := storage.NewBolt(path)
	if err != nil {
		return nil, nil, err
	}
	tip, err := store.Tip()
	if err != nil {
		store.Close()
		return nil, nil, err
	}
	if len(tip) == 0 {
		store.Close()
		return nil, nil, fmt.Errorf("no blockchain; run: trellis createblockchain -address ADDR")
	}
	// NewChain loads the existing chain (tip is non-empty, so it does not re-seed).
	// The `to`/reward args are unused on load; pass nil/0.
	chain, err := ledger.NewChain(store, nil, 0, difficulty)
	if err != nil {
		store.Close()
		return nil, nil, err
	}
	return chain, store, nil
}
