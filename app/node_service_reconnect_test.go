package app

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

type reconnectHandle struct {
	url   string
	stops atomic.Int32
}

func (h *reconnectHandle) WSURL() string   { return h.url }
func (h *reconnectHandle) DataDir() string { return "" }
func (h *reconnectHandle) Stop() error     { h.stops.Add(1); return nil }

func nextSubscription(t *testing.T, subscribed <-chan *websocket.Conn) *websocket.Conn {
	t.Helper()
	select {
	case conn := <-subscribed:
		return conn
	case <-time.After(3 * time.Second):
		t.Fatal("wallet did not restore its momentum subscription")
		return nil
	}
}

func TestEmbeddedConnectionRecoversWhileWalletLocked(t *testing.T) {
	var recovering atomic.Bool
	var attempts atomic.Int32
	url, subscribed := momentumTestServerWithFrontierGate(t, true, func() bool {
		return recovering.Load() && attempts.Add(1) <= 2
	})
	n := newTestNode(t)
	n.wallet = newWalletService(n.config)
	unlockTestWallet(t, n.wallet)
	h := &reconnectHandle{url: url}
	n.mode = "embedded"
	n.embedded = h
	n.embeddedReconnectDelay = 5 * time.Millisecond
	n.embeddedStart = func(string) (embeddedHandle, error) {
		t.Error("recovery must never start another embedded node")
		return nil, nil
	}
	t.Cleanup(func() { n.Disconnect() })
	if err := n.setNode(url); err != nil {
		t.Fatal(err)
	}
	old := nextSubscription(t, subscribed)
	if err := n.wallet.Lock(); err != nil {
		t.Fatal(err)
	}
	if !n.NodeStatus().Connected {
		t.Fatal("locking disconnected the node")
	}
	// Simulate the lost wallet socket seen during sync. The first two recovery
	// probes fail while the node is busy; none may stop or restart that node.
	recovering.Store(true)
	old.Close()
	replacement := nextSubscription(t, subscribed)
	if err := replacement.WriteJSON(map[string]any{
		"jsonrpc": "2.0", "method": "ledger.subscription",
		"params": map[string]any{"subscription": "test-subscription", "result": []map[string]any{{"height": 400001}}},
	}); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for n.NodeStatus().Height != 400001 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if n.NodeStatus().Height != 400001 {
		t.Fatal("replacement subscription did not deliver momentum")
	}
	if h.stops.Load() != 0 {
		t.Fatal("recovery stopped the syncing node")
	}
	if _, err := n.wallet.CurrentAccounts(); err == nil {
		t.Fatal("recovery unlocked the wallet")
	}
	n.mu.RLock()
	polling := n.syncStop != nil
	sameNode := n.embedded == h
	n.mu.RUnlock()
	if !polling || !sameNode {
		t.Fatal("recovery did not restore sync polling on the existing node")
	}
	if _, err := n.currentClient().LedgerApi.GetFrontierMomentum(); err != nil {
		t.Fatalf("restored read: %v", err)
	}
}

func TestEmbeddedRecoveryRespectsNewerIntent(t *testing.T) {
	for _, action := range []string{"disconnect", "remote", "shutdown"} {
		t.Run(action, func(t *testing.T) {
			n := newTestNode(t)
			h := &reconnectHandle{url: "ws://127.0.0.1:1"}
			n.mode = "embedded"
			n.embedded = h
			gen := n.connGen
			switch action {
			case "disconnect":
				n.Disconnect()
			case "remote":
				seedNodeMode(t, n, "remote")
			case "shutdown":
				n.stopEmbedded()
			}
			if _, retry := n.retryEmbeddedConnection(gen); retry {
				t.Fatal("superseded recovery remained active")
			}
			if n.NodeStatus().Connected {
				t.Fatal("superseded recovery installed a connection")
			}
		})
	}
}

func TestShutdownInvalidatesEmbeddedRecoveryInFlight(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	url, _ := momentumTestServerWithFrontierGate(t, true, func() bool {
		close(entered)
		<-release
		return false
	})
	n := newTestNode(t)
	h := &reconnectHandle{url: url}
	n.mode = "embedded"
	n.embedded = h
	done := make(chan bool, 1)
	go func() { _, retry := n.retryEmbeddedConnection(0); done <- retry }()
	t.Cleanup(func() { n.Disconnect() })
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		close(release)
		t.Fatal("recovery never reached the frontier probe")
	}
	n.stopEmbedded()
	close(release)
	select {
	case retry := <-done:
		if retry {
			t.Fatal("shutdown left recovery active")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("recovery did not finish after shutdown")
	}
	if n.NodeStatus().Connected || n.currentClient() != nil {
		t.Fatal("recovery published a connection after shutdown")
	}
}
