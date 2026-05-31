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
	"github.com/goldh0rse/trellis/pkg/storage"
	"github.com/goldh0rse/trellis/pkg/wallet"
)

const (
	difficulty = 3   // Proof-of-Work target (leading hex zeros); low for a snappy CLI
	reward     = 100 // genesis coinbase reward
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
		return cmdPrintChain(cfg, out)
	default:
		return fmt.Errorf("unknown command %q", cmd)
	}
}

func usageError() error {
	return fmt.Errorf("usage: trellis <createwallet|listaddresses|createblockchain|getbalance|send|printchain> [flags]")
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
	if err := fs.Parse(args); err != nil {
		return err
	}
	w, ok := keyring.Wallet(*address)
	if !ok {
		return fmt.Errorf("unknown address %q (not in keyring)", *address)
	}

	store, err := storage.NewBolt(cfg.dbPath)
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
	if err := fs.Parse(args); err != nil {
		return err
	}
	w, ok := keyring.Wallet(*address)
	if !ok {
		return fmt.Errorf("unknown address %q (not in keyring)", *address)
	}

	chain, store, err := openChain(cfg, difficulty)
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
	toWallet, ok := keyring.Wallet(*to)
	if !ok {
		return fmt.Errorf("unknown recipient address %q: this single-node CLI can only send to wallets it holds", *to)
	}

	chain, store, err := openChain(cfg, difficulty)
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

func cmdPrintChain(cfg config, out io.Writer) error {
	chain, store, err := openChain(cfg, difficulty)
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

// openChain opens the store and loads the existing chain, erroring if the chain
// has not been initialized yet. The caller must Close the returned store.
func openChain(cfg config, difficulty int) (*ledger.Chain, *storage.Bolt, error) {
	store, err := storage.NewBolt(cfg.dbPath)
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
