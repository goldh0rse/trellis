# 0004. Command-line interface with persistent wallets, mempool, and balance replay

- Status: Accepted
- Date: 2026-05-31

## Context

After persistence (ADR-0003) the chain was still driven by a hard-coded demo with
wallets regenerated on every run, so balances and identities never survived.
Phase 4 of the build plan calls for a usable CLI: persistent wallets, a mempool,
balance tracking, and subcommands to create wallets, fund the chain, send coins,
and inspect state. Several smaller decisions fall out of this — how coins enter
circulation, where private keys live, how balances are computed, and how to keep
the logic testable behind a command-line front end.

## Decision

We will add a `cmd/trellis` CLI built on the standard library `flag` package,
with a subcommand dispatcher implemented as `run(args []string, cfg config, out
io.Writer) error`. Taking the file paths via `config` and writing to an
`io.Writer` keeps the whole CLI testable; `main` builds the production config and
an end-to-end test drives `run` against files in a temp directory.

Subcommands: `createwallet`, `listaddresses`, `createblockchain -address`,
`getbalance -address`, `send -from -to -amount`, `printchain`. The store is
opened lazily — keyring-only commands do not create the chain database.

Specific choices:
- **Genesis funding** is explicit: `createblockchain -address ADDR` mines a
  genesis coinbase reward to ADDR (guarding against an already-initialized chain).
  There is no per-block mining reward (incentives are out of scope), so total
  supply is fixed at the genesis reward.
- **Wallet keys** are persisted behind a consumer-side `wallet.KeyStore`
  interface; a gob file implementation (`storage.KeyFile`) lives in `pkg/storage`,
  preserving ADR-0003's "gob only in storage" invariant. `storage.KeyFile`
  satisfies `wallet.KeyStore` structurally — storage does not import wallet; the
  satisfaction is asserted in the storage test. Seeds are stored **unencrypted**
  in a separate file (`wallets.dat`), keyed by address, never in the chain DB.
  Writes are atomic (temp file + rename, `0600`).
- **Balances** are computed by replaying confirmed transactions over the chain
  iterator, keyed by full public key (`+Amount` to the recipient, `-Amount` from
  the sender; coinbase only credits). `send` rejects amounts exceeding the
  sender's balance before signing.
- A minimal `ledger.Mempool` holds pending transactions (validated by signature
  on add). In Phase 4 each `send` adds one transaction and mines immediately, so
  the mempool is effectively single-use; it becomes central in Phase 5.

## Consequences

- The chain is usable end to end from the shell, and wallets/balances persist
  across runs. The acceptance scenario (two wallets, fund, send, inspect) passes
  by hand and as an automated test.
- The same consumer-side-interface pattern now covers signing, block storage, and
  key storage, keeping each external dependency confined to one package.
- Heavy logic (keyring, balance, mempool) lives in tested packages; the CLI layer
  is thin glue but still covered by an end-to-end test.
- Limitation: this is a single node — `send` only works between wallets in the
  local keyring, because resolving a recipient requires its public key. Sending
  to external parties arrives with networking in Phase 5.
- Security caveat: `wallets.dat` holds unencrypted private-key seeds and is
  git-ignored. Encrypted/HSM-backed storage remains out of scope.
