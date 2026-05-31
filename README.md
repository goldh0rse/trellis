# Trellis

A small, single-binary **post-quantum blockchain** written in Go, built as a
learning project. Its one distinguishing feature is **post-quantum signatures**:
transactions are signed with **ML-DSA-44** (FIPS 204 / CRYSTALS-Dilithium) via
[`filippo.io/mldsa`](https://pkg.go.dev/filippo.io/mldsa). Hashing stays
SHA-256 — hash functions are already quantum-resistant, so only the signatures
need to change.

## Guiding principles

1. **Hashing stays SHA-256.** Only signatures are post-quantum.
2. **One concept per file.**
3. **Each phase must build, vet, and test clean** before the next begins.
4. **Stdlib-first** — add a dependency only when it clearly earns its place.
5. **Validate everything from outside** (signatures, hashes, links).

## Layout

```
cmd/trellis/      # CLI binary
pkg/pqsig/        # post-quantum signatures — the ONLY importer of filippo.io/mldsa
  pqsig.go          GenerateKey; NewPrivateKeyFromSeed; Sign; Verify; Seed
pkg/wallet/       # identity, built on pqsig
  wallet.go         keypair; PublicKey; Sign; Address; FromSeed; Seed
  keystore.go       KeyStore interface (seed persistence boundary)
  keyring.go        Keyring: create/lookup/list wallets via a KeyStore
pkg/ledger/       # data model & rules (one concept per file)
  transaction.go    account-style tx; Signer; Sign / Verify; coinbase
  block.go          SHA-256 hash-linked block; Proof of Work (Mine)
  chain.go          genesis; AddBlock; IsValid (Store-backed)
  store.go          Store interface; ErrBlockNotFound
  iterator.go       walks the chain tip → genesis
  mempool.go        pending transactions awaiting mining
  balance.go        account balance by transaction replay
  util.go           display helpers
pkg/storage/      # persistence — the ONLY importer of go.etcd.io/bbolt + encoding/gob
  storage.go        Bolt: implements ledger.Store (bbolt buckets, gob blocks)
  keyfile.go        KeyFile: implements wallet.KeyStore (gob seed file)
```

Dependencies flow one way: `wallet → pqsig`, `ledger → pqsig`, and
`storage → ledger`. Each package defines the interfaces it needs and depends on
those, never on the implementing package: the ledger's `Signer` (satisfied by a
wallet) and `Store` (satisfied by `storage.Bolt`), and the wallet's `KeyStore`
(satisfied structurally by `storage.KeyFile`). Each external dependency is
confined to one package: ML-DSA to `pqsig` (making the planned Go 1.27
`crypto/mldsa` migration a one-file change) and bbolt/gob to `storage`.

The model is **account-style**: each transaction is `From → To : Amount`, where
`From`/`To` are raw ML-DSA public keys. A coinbase transaction (empty `From`) is
signature-free and is how coins enter circulation.

## Build & test

```bash
go build ./...   # compile
go vet ./...     # static checks
go test ./...    # unit tests
make clean       # remove trellis.db, wallets.dat, and the binary
```

## CLI

The `trellis` binary drives the chain from the command line. Wallets persist to
`wallets.dat` and the chain to `trellis.db`.

```bash
go run ./cmd/trellis createwallet               # -> a new address
go run ./cmd/trellis listaddresses
go run ./cmd/trellis createblockchain -address ADDR   # genesis reward to ADDR
go run ./cmd/trellis getbalance -address ADDR
go run ./cmd/trellis send -from ADDR -to ADDR -amount N
go run ./cmd/trellis printchain
```

Example session:

```bash
A=$(go run ./cmd/trellis createwallet | awk '{print $NF}')
B=$(go run ./cmd/trellis createwallet | awk '{print $NF}')
go run ./cmd/trellis createblockchain -address "$A"   # A gets 100
go run ./cmd/trellis send -from "$A" -to "$B" -amount 30
go run ./cmd/trellis getbalance -address "$A"         # 70
go run ./cmd/trellis getbalance -address "$B"         # 30
```

This is a single-node CLI: `send` only works between wallets held in the local
keyring (the recipient's public key must be known).

## Benchmarks

```bash
make bench   # or: go test -run=^$ -bench=. -benchmem ./pkg/...
```

Benchmarks cover key generation, signing, verification, block hashing, and
full-chain validation. They make the project's thesis measurable: SHA-256
hashing is hundreds of times cheaper than a single signature verification, so
validation cost is dominated by the post-quantum signatures.

## Status & roadmap

- [x] **Phase 1** — Foundation + post-quantum signature layer
- [x] **Phase 2** — Proof of Work (nonce, difficulty, mining)
- [x] **Phase 3** — Persistence (bbolt + gob)
- [x] **Phase 4** — Wallets, mempool & CLI
- [ ] **Phase 5** — Networking & consensus (P2P)
- [ ] **Phase 6** — Tests & polish

## Disclaimer

A learning project — not audited, not production-ready. Wallet seeds are
persisted **unencrypted** to `wallets.dat`; do not use it to secure anything real.
