package ledger

// Mempool holds transactions that have been accepted but not yet mined into a
// block. In Phase 4 the CLI adds a single transaction and mines it immediately,
// so the pool is effectively single-use; it becomes central in Phase 5, where
// transactions propagate between nodes and a miner drains the pool.
type Mempool struct {
	txs []*Transaction
}

// NewMempool returns an empty mempool.
func NewMempool() *Mempool {
	return &Mempool{}
}

// Add validates a transaction's signature/integrity (via Verify) and queues it.
// It does NOT check balances — that is chain state the mempool does not own; the
// caller checks affordability before adding.
func (m *Mempool) Add(tx *Transaction) error {
	if err := tx.Verify(); err != nil {
		return err
	}
	m.txs = append(m.txs, tx)
	return nil
}

// Transactions returns a copy of the queued transactions, so a later Clear (or
// further Adds) cannot mutate a slice the caller is mining.
func (m *Mempool) Transactions() []*Transaction {
	out := make([]*Transaction, len(m.txs))
	copy(out, m.txs)
	return out
}

// Clear removes all queued transactions.
func (m *Mempool) Clear() {
	m.txs = nil
}
