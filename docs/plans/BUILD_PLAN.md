# BUILD_PLAN.md — Post-Quantum Blockchain in Go

A working plan for an agent (Claude Code) to extend this learning project. Tackle
**one phase per session**, keep the build green, and tick the checkboxes as you go.

---

## 0. Context & non-negotiables

This is a **learning project**: prioritise clarity and correctness over
performance or completeness. It is a small, single-binary blockchain in Go whose
distinguishing feature is **post-quantum signatures**.

**Guiding principles — do not violate these:**

1. **Hashing stays SHA-256.** Hash functions are already quantum-resistant (the
   best quantum attack only halves their strength). Do **not** try to make the
   hashing "post-quantum." The only quantum-vulnerable part of a blockchain is
   the **signatures**, and those already use ML-DSA. Leave SHA-256 alone.
2. **One concept per file.** Keep the existing file layout. New concepts get new
   files, not bloated existing ones.
3. **Each phase must compile and run before the next is started.** After every
   phase, this must pass clean:
   ```bash
   go build ./... && go vet ./... && go test ./...
   ```
4. **Stdlib-first.** Add a dependency only when it clearly earns its place
   (the plan names the few that do).
5. **Validate everything that comes from outside.** Never trust a transaction,
   block, or peer without verifying signatures, hashes, and links.

Consider creating a `CLAUDE.md` that restates principles 1–5 so they persist
across sessions.

---

## 1. Current state (already implemented — do not rebuild)

Phase 1 plus the post-quantum signature layer already exists:

| File             | Responsibility                                            |
|------------------|-----------------------------------------------------------|
| `block.go`       | `Block` struct; SHA-256 `ComputeHash`; `NewBlock`         |
| `transaction.go` | Account-style `Transaction`; `Sign` / `Verify` via ML-DSA |
| `wallet.go`      | `Wallet` (ML-DSA-44 keypair); `PublicKey`; `Address`      |
| `blockchain.go`  | `Blockchain`; genesis; `AddBlock`; `IsValid`              |
| `util.go`        | `short()` hex helper                                      |
| `main.go`        | Demo: build → validate → tamper → invalid                 |

The model is **account-style** (each tx is `From → To : Amount`, where `From`/`To`
are raw ML-DSA public keys). Keep this model unless a phase says otherwise.

**First action of the first session:** run `go get filippo.io/mldsa@latest`,
then `go mod tidy`, then `go run .`. Confirm it prints `Chain valid? true` then
`Chain valid? false` after tampering. Fix any compile issues before proceeding —
the scaffold was written against the documented API but had not been compiled.

---

## 2. Dependencies

- **`filippo.io/mldsa`** — ML-DSA-44 signatures (FIPS 204 / standardised
  CRYSTALS-Dilithium). This wraps Go's internal FIPS 204 implementation.
- Alternative: **`github.com/cloudflare/circl/sign/mldsa`** (CIRCL) if the above
  ever fails to resolve.
- **Future:** a public `crypto/mldsa` is slated for **Go 1.27**. When that lands,
  migrate off `filippo.io/mldsa` to the stdlib package and drop the dependency.
- Phase 3 adds **`go.etcd.io/bbolt`** for storage (only when you reach it).

### ML-DSA API reference — use these signatures exactly

The agent's training data predates this package. Do **not** guess the API; use
the following (package `filippo.io/mldsa`):

```go
// Parameter sets (use MLDSA44 for this project)
func MLDSA44() *Parameters   // also MLDSA65(), MLDSA87()

// Key generation
func GenerateKey(params *Parameters) (*PrivateKey, error)
func NewPrivateKey(params *Parameters, seed []byte) (*PrivateKey, error) // seed is PrivateKeySize (32) bytes
func NewPublicKey(params *Parameters, encoding []byte) (*PublicKey, error)

// PrivateKey methods
func (sk *PrivateKey) PublicKey() *PublicKey
func (sk *PrivateKey) Bytes() []byte // returns the 32-byte seed (use to persist a key)
func (sk *PrivateKey) Sign(_ io.Reader, message []byte, opts crypto.SignerOpts) ([]byte, error)
// ^ the io.Reader is ignored; pass nil. nil opts => sign the message directly.

// PublicKey methods
func (pk *PublicKey) Bytes() []byte // raw encoding; round-trips with NewPublicKey

// Verification — returns nil error when the signature is VALID
func Verify(pk *PublicKey, message []byte, signature []byte, opts *Options) error

// Sizes (informational): public key 1312 B, signature 2420 B, seed 32 B for ML-DSA-44.
```

These large sizes matter for serialization and networking (Phases 3 & 5).

---

## 3. Build phases

### Phase 2 — Proof of Work

**Goal:** a block is only valid once it has been mined to a target difficulty.

- [ ] Add `Nonce uint64` and a difficulty notion to the design (store
      `Difficulty int` on `Blockchain`; pass it into block creation).
- [ ] Include `Nonce` in the bytes hashed by `Block.ComputeHash`.
- [ ] Add `Block.Mine(difficulty int)` that increments `Nonce` until the hex
      hash starts with `difficulty` zeros. Return the attempt count.
- [ ] Mine new blocks in `AddBlock` (genesis may use a fixed/mined nonce).
- [ ] Extend `IsValid` to also reject any block whose hash fails the difficulty.
- [ ] Update `main.go` to print attempts and elapsed time per mined block.

**Acceptance:** with difficulty 4, mined blocks have hashes starting `0000`;
`IsValid` returns false if a block's nonce or data is altered.
**Watch-outs:** `Nonce` must be inside the hashed bytes; pick hex-nibble zeros
(simple) and keep difficulty low (3–4) so it stays instant.

---

### Phase 3 — Persistence

**Goal:** the chain survives process restarts.

- [ ] Add `go.etcd.io/bbolt`.
- [ ] Serialise blocks with `encoding/gob` (all relevant fields are exported and
      `[]byte`, so gob works directly).
- [ ] Store blocks in a bucket keyed by block hash, plus a `tip` key holding the
      latest hash (mirrors the standard pattern).
- [ ] On startup, open/create the DB; if empty, write genesis.
- [ ] Add a `BlockchainIterator` that walks from tip back to genesis via
      `PrevHash`.
- [ ] Refactor `Blockchain` to read/write the DB instead of an in-memory slice
      (keep `IsValid` working by iterating).

**Acceptance:** run the program once to add blocks, run it again, and the chain
is still present and `IsValid`.
**Watch-outs:** never store private keys in the chain DB (Phase 4 handles keys
separately). Close the DB cleanly.

---

### Phase 4 — Wallets, mempool & CLI

**Goal:** drive the chain from the command line with persistent wallets.

- [ ] Persist wallets to a separate file: store each key's 32-byte seed
      (`sk.Bytes()`), restore with `NewPrivateKey(MLDSA44(), seed)`. Key the file
      by address. **Add a comment: seeds are stored unencrypted — learning only.**
- [ ] Add a **mempool** of pending transactions; mining pulls from it.
- [ ] Compute balances by replaying confirmed transactions (account model).
- [ ] Reject sends that exceed the sender's balance.
- [ ] CLI (use `flag` or a small subcommand dispatcher):
  - [ ] `createwallet` → prints a new address
  - [ ] `listaddresses`
  - [ ] `getbalance -address ADDR`
  - [ ] `send -from ADDR -to ADDR -amount N` → signs, adds to mempool, mines a block
  - [ ] `printchain`

**Acceptance:** create two wallets, `send` between them, `printchain` shows the
transaction in a mined block, and `getbalance` reflects it.
**Watch-outs:** the genesis/coinbase reward (no `From`) is how coins enter
circulation — keep that path signature-free but everything else signed.

---

### Phase 5 — Networking & consensus (P2P)

**Goal:** multiple nodes stay in sync. This is the largest phase.

- [ ] A node server over TCP using gob-encoded messages.
- [ ] Message types: `version`, `getblocks`, `inv`, `getdata`, `block`, `tx`.
- [ ] A known-nodes list with one seed/central node for discovery (a deliberate
      learning simplification — real discovery is out of scope).
- [ ] On `version`, compare best heights and request missing blocks.
- [ ] Propagate new transactions; a miner node mines them and broadcasts the block.
- [ ] **Consensus rule:** accept a competing chain only if it is **longer AND
      fully valid** (`IsValid` plus every block meets difficulty).
- [ ] Use goroutines for connections; guard shared state (chain, mempool) with
      `sync.Mutex`.

**Acceptance:** start node A (miner) and node B; `send` a tx to B; it propagates
to A; A mines and broadcasts; B receives the block; both nodes report identical
tips and `IsValid`.
**Watch-outs:** validate every inbound block and tx before accepting. Never
replace your chain with a shorter or invalid one. Remember ML-DSA blocks are
large (~2.4 KB per signature) — size your reads/buffers accordingly.

---

### Phase 6 — Tests & polish

**Goal:** lock in correctness.

- [ ] Table-driven unit tests:
  - [ ] hash determinism (same block → same hash)
  - [ ] sign/verify round-trip; tampered amount fails `Verify`
  - [ ] `IsValid` catches tampered data, broken links, forged signatures
  - [ ] proof-of-work: mined hash meets difficulty
  - [ ] persistence round-trip (write, reopen, still valid)
- [ ] Optional: a read-only HTTP/JSON endpoint to inspect the chain.
- [ ] Optional: structured logging.

**Acceptance:** `go test ./...` passes with meaningful coverage of the above.

---

## 4. Definition of done (per phase)

A phase is done when: its checkboxes are ticked, `go build ./... && go vet ./...
&& go test ./...` is clean, its acceptance scenario has been run by hand, and the
`main.go` demo (or CLI) still works end-to-end.

## 5. Out of scope

Real consensus economics / incentives, NAT-traversing peer discovery, smart
contracts or scripting, encrypted/HSM-backed key storage, and any performance
tuning. If tempted by these, stop and confirm with the human first.
