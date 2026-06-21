// Package embeddednode runs a full go-zenon node in-process (mainnet, loopback
// RPC) so the wallet can use a locally-synced node. It is not Wails-bound.
package embeddednode

import (
	"fmt"
	"net"
	"path/filepath"
	"sync"
	"time"

	"github.com/zenon-network/go-zenon/node"
)

// EmbeddedWSPort is the loopback port the embedded node's WS RPC binds to.
const EmbeddedWSPort = 35998

var (
	mu      sync.Mutex
	current *node.Node // single-instance guard
)

// buildConfig derives the embedded node config from go-zenon defaults, keeping
// the default seeders/peer settings but forcing loopback WS, HTTP off, no
// producer, and an empty GenesisFile (→ embedded mainnet genesis).
func buildConfig(dataDir string) node.Config {
	cfg := node.DefaultNodeConfig // value copy keeps Net defaults (seeders)
	cfg.DataPath = filepath.Join(dataDir, "embedded")
	cfg.WalletPath = filepath.Join(cfg.DataPath, "wallet")
	cfg.GenesisFile = ""
	cfg.Name = "go-syrius-embedded"
	cfg.LogLevel = "warn"
	cfg.Producer = nil
	cfg.RPC = node.RPCConfig{
		EnableWS:   true,
		WSHost:     "127.0.0.1",
		WSPort:     EmbeddedWSPort,
		EnableHTTP: false,
		WSOrigins:  []string{},
	}
	return cfg
}

// Handle owns a running embedded node.
type Handle struct {
	node    *node.Node
	wsURL   string
	dataDir string
}

func (h *Handle) WSURL() string   { return h.wsURL }
func (h *Handle) DataDir() string { return h.dataDir }

// Start brings up the embedded node and returns once its WS RPC accepts a TCP
// connection (or a bounded timeout elapses). Only one embedded node may run.
func Start(dataDir string) (*Handle, error) {
	mu.Lock()
	defer mu.Unlock()
	if current != nil {
		return nil, fmt.Errorf("embedded node already running")
	}
	cfg := buildConfig(dataDir)
	n, err := node.NewNode(&cfg)
	if err != nil {
		return nil, fmt.Errorf("create embedded node: %w", err)
	}
	if err := n.Start(); err != nil {
		_ = n.Stop()
		return nil, fmt.Errorf("start embedded node: %w", err)
	}
	addr := fmt.Sprintf("127.0.0.1:%d", EmbeddedWSPort)
	if err := waitForPort(addr, 30*time.Second); err != nil {
		_ = n.Stop()
		return nil, fmt.Errorf("embedded node rpc not ready: %w", err)
	}
	current = n
	return &Handle{node: n, wsURL: fmt.Sprintf("ws://%s", addr), dataDir: cfg.DataPath}, nil
}

// Stop halts the embedded node. Idempotent.
func (h *Handle) Stop() error {
	mu.Lock()
	defer mu.Unlock()
	if h == nil || h.node == nil {
		return nil
	}
	err := h.node.Stop()
	if current == h.node {
		current = nil
	}
	h.node = nil
	return err
}

func waitForPort(addr string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		c, err := net.DialTimeout("tcp", addr, time.Second)
		if err == nil {
			_ = c.Close()
			return nil
		}
		time.Sleep(250 * time.Millisecond)
	}
	return fmt.Errorf("port %s not open within %s", addr, timeout)
}
