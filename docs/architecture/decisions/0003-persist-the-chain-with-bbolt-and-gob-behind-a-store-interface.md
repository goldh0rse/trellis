# 0003. Persist the chain with bbolt and gob behind a Store interface

- Status: Accepted
- Date: 2026-05-31

## Context

Until now the chain lived in an in-memory slice and was lost when the process
exited. Phase 3 of the build plan requires the chain to survive restarts. This
forces a storage engine and a serialization format into the project, which sits
in tension with two principles: "stdlib-first" (bbolt is a third-party
dependency) and keeping the domain (`pkg/ledger`) clean. We had already
established a pattern for exactly this tension — `pkg/pqsig` is the sole importer
of the ML-DSA library, and the ledger depends on a consumer-defined `Signer`
interface rather than on the wallet package.

We considered putting bbolt directly inside the ledger (fewer packages, the build
plan's literal suggestion) versus hiding it behind an interface. The direct
approach couples the domain to bbolt and gob and forces every ledger test to use
real temporary database files.

## Decision

We will persist the chain with [bbolt](https://github.com/etcd-io/bbolt)
(embedded key-value store) and serialise blocks with `encoding/gob`, both
confined to a new `pkg/storage` package — the only importer of either.

The ledger defines a consumer-side `Store` interface
(`Tip`, `GetBlock`, `AppendBlock`) and a sentinel `ErrBlockNotFound`. `Chain`
holds a `Store` instead of a `[]*Block` slice; `NewChain` seeds and mines genesis
only when the store is empty, otherwise it loads the existing chain. A new
`Iterator` walks the chain tip → genesis via `PrevHash`, and `IsValid` validates
by iterating. `storage.Bolt` implements `Store` using two buckets (`blocks`
keyed by hash, `meta` holding the `tip`), writing a block and advancing the tip
in a single atomic transaction. The concrete `*Bolt` owns `Close`; the caller
manages its lifecycle.

Because `PrevHash` and `Nonce` are part of `ComputeHash`, the backward walk needs
no separate link check: a tampered `PrevHash` breaks hash integrity, and a
dangling parent surfaces as `ErrBlockNotFound`. `IsValid` also carries a cycle
guard so a corrupted store cannot loop forever.

## Consequences

- The chain persists across restarts (the Phase 3 acceptance), and each external
  dependency is now confined to one package: ML-DSA to `pqsig`, bbolt/gob to
  `storage`. Swapping the storage engine later is a one-package change.
- The ledger stays storage-agnostic; its tests run against an in-memory `Store`
  fake (no disk), while `pkg/storage` is tested against a real bbolt file on
  `t.TempDir()`, including a close-and-reopen round trip.
- `NewChain` now returns an error and takes a `Store`, rippling through `cmd` and
  tests. The demo became load-or-init (no live tamper demo; tampering is covered
  by tests).
- New discipline: bbolt values are only valid inside their transaction, so blocks
  are gob-decoded inside the read transaction and the tip is copied out.
- Private keys are never written to the chain database (wallet persistence is a
  separate Phase 4 concern).
