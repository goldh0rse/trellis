# 0002. Split blockchain into pqsig, wallet, and ledger packages

- Status: Accepted
- Date: 2026-05-31

## Context

The project began with every concept — keys, transactions, blocks, the chain —
in a single `pkg/blockchain` package. Two issues emerged as it grew:

1. The post-quantum dependency (`filippo.io/mldsa`) was imported by multiple
   files (`wallet.go`, `transaction.go`). The build plan calls for migrating to
   the stdlib `crypto/mldsa` when it lands in Go 1.27, and a scattered dependency
   makes that migration touch many places.
2. `Transaction.Sign(w *Wallet)` reached into the wallet's unexported `priv`
   field — a transaction concept depending on the internals of an identity
   concept, which is a layering smell.

We considered three granularities: a minimal 2-package split (extract crypto
only), this layered 3-package split, and a granular 5-package split (one package
per concept). The granular option was rejected: transactions, blocks, and the
chain are tightly coupled, and separating them would force exporting internal
helpers (`signingPayload`, `txDigest`, `meetsDifficulty`), working against the
"one concept per file / clarity over completeness" principles.

## Decision

We will split the code into three packages along the boundaries that actually
reduce coupling:

- **`pkg/pqsig`** — post-quantum signature primitives (ML-DSA-44). It is the
  *only* package that imports `filippo.io/mldsa`. Public keys cross its boundary
  as raw `[]byte`.
- **`pkg/wallet`** — identity (keypair + address), built on `pqsig`. A wallet can
  sign a message, which lets it satisfy the ledger's `Signer` interface.
- **`pkg/ledger`** — the data model and rules (transactions, blocks, the chain,
  Proof of Work, validation). It depends on `pqsig` for verification but not on
  `wallet`: it defines a small `Signer` interface (`PublicKey() []byte;
  Sign(message []byte) ([]byte, error)`) that the wallet satisfies, inverting the
  dependency.

The chain type is renamed `Blockchain` → `ledger.Chain` (and `NewBlockchain` →
`NewChain`) to avoid stutter. `pqsig.Seed()` and size constants are deferred until
Phase 4 has a consumer. Ledger tests use an external `ledger_test` package, since
no test needs unexported internals.

## Consequences

- The ML-DSA dependency is confined to one file, so the Go 1.27 stdlib migration
  becomes a one-file change, and the post-quantum boundary is explicit.
- The `Signer` interface removes the cross-concept private-field access; the
  ledger no longer knows what a wallet is.
- More packages and a little more exported surface (e.g. `wallet.Sign`), and a
  one-time rename rippling through `cmd` and tests.
- Behavior is unchanged: build/vet/test stay green (coverage ~98% ledger, ~90%
  pqsig, ~88% wallet) and the demo still prints `Chain valid? true` then `false`.
- Supersedes the implicit single-package structure; future concepts slot into the
  package whose boundary they belong to.
