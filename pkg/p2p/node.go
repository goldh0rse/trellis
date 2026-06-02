package p2p

import (
	"encoding/gob"
	"log"
	"net"
	"sync"

	"github.com/goldh0rse/trellis/pkg/ledger"
)

// NodeConfig configures a Node.
type NodeConfig struct {
	Addr       string        // this node's listen address, e.g. "localhost:3001"
	Seed       string        // a peer to sync from on startup ("" = none)
	Chain      *ledger.Chain // the chain this node serves
	Mempool    *ledger.Mempool
	Mining     bool // if true, mine pending transactions into blocks
	Difficulty int  // Proof-of-Work target for mined/validated blocks
}

// Node is a TCP peer that keeps its chain in sync with others. All access to the
// chain, mempool, peer set, and download queue goes through mu; network I/O and
// mining are always performed with the lock released.
type Node struct {
	addr       string
	seed       string
	mining     bool
	difficulty int

	mu       sync.Mutex
	chain    *ledger.Chain
	mempool  *ledger.Mempool
	peers    map[string]bool
	wanted   [][]byte        // queued block hashes still to download (genesis→tip order)
	syncPeer string          // peer we are downloading queued blocks from
	seen     map[string]bool // tx IDs already processed, to stop propagation loops

	ln   net.Listener
	done chan struct{}
}

// NewNode builds a node from cfg. Call Run to start serving.
func NewNode(cfg NodeConfig) *Node {
	return &Node{
		addr:       cfg.Addr,
		seed:       cfg.Seed,
		mining:     cfg.Mining,
		difficulty: cfg.Difficulty,
		chain:      cfg.Chain,
		mempool:    cfg.Mempool,
		peers:      make(map[string]bool),
		seen:       make(map[string]bool),
		done:       make(chan struct{}),
	}
}

// Addr returns the node's listen address (valid after Run has bound it).
func (n *Node) Addr() string {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.ln != nil {
		return n.ln.Addr().String()
	}
	return n.addr
}

// Height returns the node's current chain height (race-safe).
func (n *Node) Height() int {
	n.mu.Lock()
	defer n.mu.Unlock()
	h, _ := n.chain.Height()
	return h
}

// Tip returns the node's current tip hash (race-safe).
func (n *Node) Tip() []byte {
	n.mu.Lock()
	defer n.mu.Unlock()
	t, _ := n.chain.Tip()
	return t
}

// Valid reports whether the node's chain validates (race-safe).
func (n *Node) Valid() bool {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.chain.IsValid() == nil
}

// Run binds the listener and serves connections until Close. If a seed was
// configured, it announces this node's height to the seed so syncing can begin.
func (n *Node) Run() error {
	ln, err := net.Listen("tcp", n.addr)
	if err != nil {
		return err
	}
	n.mu.Lock()
	n.ln = ln
	n.addr = ln.Addr().String()
	n.mu.Unlock()

	if n.seed != "" {
		go n.connectToSeed()
	}

	for {
		conn, err := ln.Accept()
		if err != nil {
			select {
			case <-n.done:
				return nil // closed intentionally
			default:
				return err
			}
		}
		go n.handleConn(conn)
	}
}

// Close stops the node's listener and accept loop.
func (n *Node) Close() error {
	close(n.done)
	n.mu.Lock()
	ln := n.ln
	n.mu.Unlock()
	if ln != nil {
		return ln.Close()
	}
	return nil
}

// connectToSeed announces our height to the seed node, prompting it to send us
// its own version (and, if it is ahead, to start feeding us blocks).
func (n *Node) connectToSeed() {
	n.mu.Lock()
	height, _ := n.chain.Height()
	n.mu.Unlock()
	if err := sendPayload(n.seed, TypeVersion, Version{Height: height, AddrFrom: n.addr}); err != nil {
		log.Printf("p2p: connect to seed %s: %v", n.seed, err)
	}
}

// handleConn reads exactly one Message from conn and dispatches it.
func (n *Node) handleConn(conn net.Conn) {
	defer conn.Close()
	var m Message
	if err := gob.NewDecoder(conn).Decode(&m); err != nil {
		log.Printf("p2p: decode: %v", err)
		return
	}
	if err := n.dispatch(m); err != nil {
		log.Printf("p2p: handle %s: %v", m.Type, err)
	}
}

func (n *Node) dispatch(m Message) error {
	switch m.Type {
	case TypeVersion:
		return n.handleVersion(m)
	case TypeGetBlocks:
		return n.handleGetBlocks(m)
	case TypeInv:
		return n.handleInv(m)
	case TypeGetData:
		return n.handleGetData(m)
	case TypeBlock:
		return n.handleBlock(m)
	case TypeTx:
		return n.handleTx(m)
	default:
		log.Printf("p2p: unknown message type %q", m.Type)
		return nil
	}
}

// addPeer records a peer address (other than ourselves). Caller must hold mu.
func (n *Node) addPeerLocked(addr string) {
	if addr != "" && addr != n.addr {
		n.peers[addr] = true
	}
}

// peerList returns a snapshot of known peers excluding `except`.
func (n *Node) peerList(except string) []string {
	n.mu.Lock()
	defer n.mu.Unlock()
	out := make([]string, 0, len(n.peers))
	for p := range n.peers {
		if p != except {
			out = append(out, p)
		}
	}
	return out
}
