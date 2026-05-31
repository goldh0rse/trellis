package main

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

// runCLI drives run() with a sandboxed config and returns its stdout.
func runCLI(t *testing.T, cfg config, args ...string) string {
	t.Helper()
	var out bytes.Buffer
	if err := run(args, cfg, &out); err != nil {
		t.Fatalf("run %v: %v", args, err)
	}
	return out.String()
}

func TestCLIEndToEnd(t *testing.T) {
	dir := t.TempDir()
	cfg := config{
		dbPath:     filepath.Join(dir, "trellis.db"),
		walletPath: filepath.Join(dir, "wallets.dat"),
	}

	// Create two wallets and capture their addresses.
	addrA := lastField(runCLI(t, cfg, "createwallet"))
	addrB := lastField(runCLI(t, cfg, "createwallet"))
	if addrA == "" || addrB == "" || addrA == addrB {
		t.Fatalf("unexpected wallet addresses: %q, %q", addrA, addrB)
	}

	// Both appear in listaddresses.
	list := runCLI(t, cfg, "listaddresses")
	if !strings.Contains(list, addrA) || !strings.Contains(list, addrB) {
		t.Fatalf("listaddresses missing an address:\n%s", list)
	}

	// Fund the chain via genesis to A, then send A -> B.
	runCLI(t, cfg, "createblockchain", "-address", addrA)
	assertBalanceLine(t, runCLI(t, cfg, "getbalance", "-address", addrA), 100)

	runCLI(t, cfg, "send", "-from", addrA, "-to", addrB, "-amount", "30")
	assertBalanceLine(t, runCLI(t, cfg, "getbalance", "-address", addrA), 70)
	assertBalanceLine(t, runCLI(t, cfg, "getbalance", "-address", addrB), 30)

	// printchain shows the transfer in a mined block and reports valid.
	chain := runCLI(t, cfg, "printchain")
	if !strings.Contains(chain, ": 30") {
		t.Fatalf("printchain does not show the 30-coin transfer:\n%s", chain)
	}
	if !strings.Contains(chain, "valid: true") {
		t.Fatalf("printchain does not report a valid chain:\n%s", chain)
	}
}

func TestCLIErrors(t *testing.T) {
	dir := t.TempDir()
	cfg := config{
		dbPath:     filepath.Join(dir, "trellis.db"),
		walletPath: filepath.Join(dir, "wallets.dat"),
	}
	addr := lastField(runCLI(t, cfg, "createwallet"))

	cases := [][]string{
		{},                               // no command
		{"frobnicate"},                   // unknown command
		{"getbalance", "-address", addr}, // chain not initialized
		{"createblockchain", "-address", "deadbeefdeadbeef"}, // unknown address
	}
	for _, args := range cases {
		var out bytes.Buffer
		if err := run(args, cfg, &out); err == nil {
			t.Fatalf("run %v should have errored, got nil", args)
		}
	}

	// Insufficient funds after the chain exists.
	other := lastField(runCLI(t, cfg, "createwallet"))
	runCLI(t, cfg, "createblockchain", "-address", addr)
	var out bytes.Buffer
	if err := run([]string{"send", "-from", other, "-to", addr, "-amount", "5"}, cfg, &out); err == nil {
		t.Fatal("send with zero balance should fail, got nil")
	}
}

// lastField returns the last whitespace-separated token of the (single-line) output.
func lastField(s string) string {
	fields := strings.Fields(s)
	if len(fields) == 0 {
		return ""
	}
	return fields[len(fields)-1]
}

func assertBalanceLine(t *testing.T, out string, want uint64) {
	t.Helper()
	suffix := fmt.Sprintf(": %d", want)
	if !strings.Contains(out, suffix) {
		t.Fatalf("balance output %q does not contain %q", strings.TrimSpace(out), suffix)
	}
}
