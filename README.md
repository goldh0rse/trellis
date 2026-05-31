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
pkg/blockchain/   # library (one concept per file)
  wallet.go         ML-DSA-44 keypair; PublicKey; Address
  transaction.go    account-style tx; Sign / Verify; coinbase
  block.go          SHA-256 hash-linked block
  blockchain.go     genesis; AddBlock; IsValid
  util.go           display helpers
docs/plans/       # build plan
```

The model is **account-style**: each transaction is `From → To : Amount`, where
`From`/`To` are raw ML-DSA public keys. A coinbase transaction (empty `From`) is
signature-free and is how coins enter circulation.

## Build, test, run

```bash
go build ./...        # compile
go vet ./...          # static checks
go test ./...         # unit tests
go run ./cmd/trellis  # demo: prints "Chain valid? true" then "false" after tampering
```

The demo builds a chain, makes a signed transfer, validates it, then tampers
with the transfer's amount — which is rejected by ML-DSA signature verification.

## Benchmarks

```bash
go test -run=^$ -bench=. -benchmem ./pkg/blockchain/
```

Benchmarks cover key generation, signing, verification, block hashing, and
full-chain validation. They make the project's thesis measurable: SHA-256
hashing is hundreds of times cheaper than a single signature verification, so
validation cost is dominated by the post-quantum signatures.

## Status & roadmap

- [x] **Phase 1** — Foundation + post-quantum signature layer
- [x] **Phase 2** — Proof of Work (nonce, difficulty, mining)
- [ ] **Phase 3** — Persistence (bbolt + gob)
- [ ] **Phase 4** — Wallets, mempool & CLI
- [ ] **Phase 5** — Networking & consensus (P2P)
- [ ] **Phase 6** — Tests & polish

See [`docs/plans/BUILD_PLAN.md`](docs/plans/BUILD_PLAN.md) for the full plan.

## Disclaimer

A learning project — not audited, not production-ready. Keys are held in memory
only for now; do not use it to secure anything real.
