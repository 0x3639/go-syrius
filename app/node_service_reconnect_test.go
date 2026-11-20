package app

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

type reconnectHandle struct {
	url   string
	stops atomic.Int32
}

func (h *reconnectHandle) WSURL() string   { return h.url }
func (h *reconnectHandle) DataDir() string { return "" }
func (h *reconnectHandle) Stop() error     { h.stops.Add(1); return nil }

func nextSubscription(t *testing.T, subscribed <-chan *momentumSocket) *momentumSocket {
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
	ctx, cancel := context.WithCancel(context.Background())
	n.ctx = ctx
	t.Cleanup(cancel)
	samples := make(chan SyncStatus, 10)
	ticks := make(chan struct{}, 1)
	n.emitEvent = func(_ context.Context, name string, data ...interface{}) {
		if name == EventMomentumTick {
			select {
			case ticks <- struct{}{}:
			default:
			}
		}
		if name == EventNodeSync {
			select {
			case samples <- data[0].(SyncStatus):
			default:
			}
		}
	}
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
	// Let polling advance past the injected momentum first. Delivery must not
	// depend on observing an exact height shared with the sync poller.
	select {
	case sample := <-samples:
		if sample.CurrentHeight != 400010 || sample.TargetHeight != 500000 || sample.Peers != 3 {
			t.Fatalf("unexpected recovered sync sample: %+v", sample)
		}
	case <-time.After(4 * time.Second):
		t.Fatal("recovery did not poll and publish a fresh sync sample")
	}
	if err := replacement.WriteJSON(map[string]any{
		"jsonrpc": "2.0", "method": "ledger.subscription",
		"params": map[string]any{"subscription": "test-subscription", "result": []map[string]any{{"height": 400001}}},
	}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-ticks:
	case <-time.After(3 * time.Second):
		t.Fatal("replacement subscription did not deliver momentum")
	}
	if h.stops.Load() != 0 {
		t.Fatal("recovery stopped the syncing node")
	}
	if _, err := n.wallet.CurrentAccounts(); err == nil {
		t.Fatal("recovery unlocked the wallet")
	}
	n.mu.RLock()
	sameNode := n.embedded == h
	n.mu.RUnlock()
	if !sameNode {
		t.Fatal("recovery replaced the running node")
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

func TestExplicitTransitionSupersedesBlockedRecovery(t *testing.T) {
	for _, action := range []string{"disconnect", "remote", "shutdown"} {
		t.Run(action, func(t *testing.T) {
			entered := make(chan struct{})
			release := make(chan struct{})
			url, _ := momentumTestServerWithFrontierGate(t, true, func() bool {
				close(entered)
				<-release
				return false
			})
			remoteURL, _ := momentumTestServer(t, true)
			n := newTestNode(t)
			if err := n.config.updateSettings(func(s *Settings) error { s.RemoteNodeURL = remoteURL; return nil }); err != nil {
				t.Fatal(err)
			}
			n.ctx = context.Background()
			var connectedEvents atomic.Int32
			n.emitEvent = func(_ context.Context, name string, data ...interface{}) {
				if name == EventNodeStatus && data[0].(NodeStatus).Connected {
					connectedEvents.Add(1)
				}
			}
			h := &reconnectHandle{url: url}
			n.mode = "embedded"
			n.embedded = h
			done := make(chan bool, 1)
			go func() { _, retry := n.retryEmbeddedConnection(0); done <- retry }()
			// Release the fake server before Disconnect even on a failed assertion.
			t.Cleanup(func() {
				select {
				case <-release:
				default:
					close(release)
				}
				n.Disconnect()
			})
			select {
			case <-entered:
			case <-time.After(3 * time.Second):
				t.Fatal("recovery never reached the frontier probe")
			}
			transitionDone := make(chan error, 1)
			go func() {
				switch action {
				case "disconnect":
					transitionDone <- n.Disconnect()
				case "remote":
					transitionDone <- n.SetNodeMode("remote")
				case "shutdown":
					a := &App{Node: n, Wallet: newWalletService(n.config)}
					a.OnShutdown(context.Background())
					transitionDone <- nil
				}
			}()
			select {
			case err := <-transitionDone:
				if err != nil {
					t.Fatal(err)
				}
			case <-time.After(2 * time.Second):
				t.Fatal("explicit transition waited for the background recovery probe")
			}
			winner := n.currentClient()
			events := connectedEvents.Load()
			close(release)
			select {
			case retry := <-done:
				if retry {
					t.Fatal("explicit transition left recovery active")
				}
			case <-time.After(3 * time.Second):
				t.Fatal("obsolete recovery did not finish")
			}
			if n.currentClient() != winner || connectedEvents.Load() != events {
				t.Fatal("late candidate replaced the explicit transition")
			}
			if n.NodeStatus().Connected != (action == "remote") {
				t.Fatal("incorrect final connection state")
			}
		})
	}
}

type stoppingReconnectHandle struct {
	reconnectHandle
	called, release chan struct{}
}

func (h *stoppingReconnectHandle) Stop() error {
	close(h.called)
	<-h.release
	return nil
}

func TestShutdownInvalidatesRecoveryBeforeNodeStops(t *testing.T) {
	probeEntered, probeRelease := make(chan struct{}), make(chan struct{})
	url, _ := momentumTestServerWithFrontierGate(t, true, func() bool {
		close(probeEntered)
		<-probeRelease
		return false
	})
	n := newTestNode(t)
	n.ctx = context.Background()
	var connected atomic.Int32
	n.emitEvent = func(_ context.Context, name string, data ...interface{}) {
		if name == EventNodeStatus && data[0].(NodeStatus).Connected {
			connected.Add(1)
		}
	}
	h := &stoppingReconnectHandle{reconnectHandle: reconnectHandle{url: url}, called: make(chan struct{}), release: make(chan struct{})}
	n.mode, n.embedded = "embedded", h
	t.Cleanup(func() {
		select {
		case <-probeRelease:
		default:
			close(probeRelease)
		}
		select {
		case <-h.release:
		default:
			close(h.release)
		}
		n.Disconnect()
	})
	recovery := make(chan bool, 1)
	go func() { _, retry := n.retryEmbeddedConnection(0); recovery <- retry }()
	select {
	case <-probeEntered:
	case <-time.After(2 * time.Second):
		t.Fatal("probe did not start")
	}
	shutdown := make(chan struct{})
	go func() {
		(&App{Node: n, Wallet: newWalletService(n.config)}).OnShutdown(context.Background())
		close(shutdown)
	}()
	select {
	case <-h.called:
	case <-time.After(2 * time.Second):
		t.Fatal("shutdown did not start stopping the node")
	}
	close(probeRelease)
	select {
	case retry := <-recovery:
		if retry {
			t.Fatal("shutdown left recovery active")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("candidate did not finish during node teardown")
	}
	if n.NodeStatus().Connected || connected.Load() != 0 {
		t.Fatal("candidate published before shutdown's final Disconnect")
	}
	close(h.release)
	select {
	case <-shutdown:
	case <-time.After(2 * time.Second):
		t.Fatal("shutdown did not finish")
	}
}
