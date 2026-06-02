# 0005. Networking and sync-by-extension consensus over gob/TCP

- Status: Accepted
- Date: 2026-06-02

## Context

Trellis ran as a single node. Phase 5 of the build plan requires multiple nodes
to stay in sync over the network. This forces TCP and a wire format into the
project and raises the consensus question — how a node decides which chain to
follow — while the project's principles still demand a clean domain, stdlib-first
code, and "validate everything from outside". ADR-0003 had stated "gob only in
storage", which networking must revisit (the wire needs serialization too).

## Decision

We will add a `pkg/p2p` networking adapter and the supporting ledger primitives.

**Adapter invariant (restated, supersedes ADR-0003 on this point):** the domain
packages (`pqsig`, `wallet`, `ledger`) stay free of serialization and transport
libraries; *adapter* packages own their formats — `storage` for disk (bbolt +
gob), `p2p` for the wire (gob over TCP). `net` is imported only by `p2p`.

**Protocol.** A `Message{Type, Body}` envelope is gob-encoded over a TCP
connection; `Body` is the gob of a typed payload (`Version`, `GetBlocks`, `Inv`,
`GetData`, `BlockMsg`, `TxMsg`). One message per connection: gob over a stream is
self-delimiting, so a multi-kilobyte ML-DSA block is reconstructed by a single
`Decode` regardless of TCP segmentation — no manual buffer sizing. Requests carry
the sender's listen address so responders can dial back. No `gob.Register` is
needed (all payloads are concrete types).

**Consensus — sync-by-extension.** A fresh node downloads a peer's chain
genesis→tip (`version` → `getblocks` → `inv` → `getdata`/`block`, applied in
order via a pull-next queue), then accepts broadcast blocks that *validly extend*
its tip. A new `Chain.AcceptBlock` validates a pre-mined block (hash integrity,
Proof of Work via the now-exported `MeetsDifficulty`, transactions, coinbase only
in genesis) and appends it only if it builds on the current tip (`ErrNotExtending`
otherwise). The chain only ever grows by one and is never replaced, so a shorter
or competing chain can never win. Deep forks/reorgs are out of scope. A fresh
node uses `LoadChain` (no genesis seeding) so it adopts the peer's genesis rather
than minting a divergent one.

**Mining.** Event-driven: a node that mines builds a block from its mempool when
a transaction arrives (gated on a non-empty mempool), mines with the lock
released, then `AcceptBlock`s its own block and broadcasts it. There is no
per-block coinbase (supply stays fixed at the genesis reward, per ADR-0004).
Inbound transactions are checked for affordability against confirmed chain state
(`Chain.Balance`) before entering the mempool and being propagated; a `seen` set
stops propagation loops.

**Concurrency.** One mutex guards the chain, mempool, peer set, and download
queue; network I/O and the mining loop always run with the lock released.

**CLI.** `startnode -port P [-mine] [-db FILE]` serves a node (seed defaults to
`localhost:3000`); `send -node ADDR` signs locally and submits the tx to a node.
Every chain command takes `-db` so each node on one machine uses its own file.
`storage.NewBolt` now opens with a lock timeout so a second opener fails fast.

**Testing.** `storage.MemStore` (an in-memory `ledger.Store`) lets the two-node
integration test run disk-free at difficulty 1, binding ephemeral ports and
polling to convergence (no fixed sleeps); it passes under `-race`.

## Consequences

- Multiple nodes converge: the acceptance (submit at B → propagate → A mines →
  B receives → identical tips, both valid) passes in-process and via the CLI.
- Each external concern is confined to one adapter; the domain stays pure.
- Sync-by-extension is simple and safe but cannot resolve competing forks — a
  block that does not extend the local tip is dropped. Mempool affordability is
  checked against confirmed state only, so double-spends within one mempool
  window are possible; the miner's `AcceptBlock` is the final authority.
- bbolt is single-writer, so a node's database can only be inspected while that
  node is stopped. Peer discovery is a single hardcoded seed node.
