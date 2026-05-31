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
cmd/trellis/      # demo binary
pkg/pqsig/        # post-quantum signatures — the ONLY importer of filippo.io/mldsa
  pqsig.go          GenerateKey; PrivateKey.Sign; Verify
pkg/wallet/       # identity, built on pqsig
  wallet.go         keypair; PublicKey; Sign; Address
pkg/ledger/       # data model & rules (one concept per file)
  transaction.go    account-style tx; Signer; Sign / Verify; coinbase
  block.go          SHA-256 hash-linked block; Proof of Work (Mine)
  chain.go          genesis; AddBlock; IsValid (Store-backed)
  store.go          Store interface; ErrBlockNotFound
  iterator.go       walks the chain tip → genesis
  util.go           display helpers
pkg/storage/      # persistence — the ONLY importer of go.etcd.io/bbolt + encoding/gob
  storage.go        Bolt: implements ledger.Store (bbolt buckets, gob blocks)
```

Dependencies flow one way: `wallet → pqsig`, `ledger → pqsig`, and
`storage → ledger`. The ledger defines interfaces it needs — `Signer` (satisfied
by a wallet) and `Store` (satisfied by `storage.Bolt`) — so it never imports the
identity or storage layers. Each external dependency is confined to one package:
ML-DSA to `pqsig` (making the planned Go 1.27 `crypto/mldsa` migration a one-file
change) and bbolt/gob to `storage`.

The model is **account-style**: each transaction is `From → To : Amount`, where
`From`/`To` are raw ML-DSA public keys. A coinbase transaction (empty `From`) is
signature-free and is how coins enter circulation.

## Build, test, run

```bash
go build ./...        # compile
go vet ./...          # static checks
go test ./...         # unit tests
go run ./cmd/trellis  # demo: build/persist a chain; run again to see it reload
make clean            # remove the demo database (trellis.db) and binary
```

On a fresh database the demo mines a genesis block and one signed transfer, then
prints the chain height and validity. Run it again and it loads the existing
chain from `trellis.db` — demonstrating that the chain survives restarts.

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
- [ ] **Phase 4** — Wallets, mempool & CLI
- [ ] **Phase 5** — Networking & consensus (P2P)
- [ ] **Phase 6** — Tests & polish

## Disclaimer

A learning project — not audited, not production-ready. Keys are held in memory
only for now; do not use it to secure anything real.
