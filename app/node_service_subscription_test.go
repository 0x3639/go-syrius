package app

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// Answer frontier reads, with an optional subscription acknowledgement. The
// returned socket lets the test simulate remote transport loss after setup.
func momentumTestServer(t *testing.T, acknowledge bool) (string, <-chan *websocket.Conn) {
	return momentumTestServerWithFrontierGate(t, acknowledge, nil)
}

func momentumTestServerWithFrontierGate(t *testing.T, acknowledge bool, rejectFrontier func() bool) (string, <-chan *websocket.Conn) {
	t.Helper()
	subscribed := make(chan *websocket.Conn, 1)
	var socketsMu sync.Mutex
	var sockets []*websocket.Conn
	closed := false
	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		socketsMu.Lock()
		if closed {
			socketsMu.Unlock()
			conn.Close()
			return
		}
		sockets = append(sockets, conn)
		socketsMu.Unlock()
		defer conn.Close()
		for {
			var req struct {
				ID     any    `json:"id"`
				Method string `json:"method"`
			}
			if err := conn.ReadJSON(&req); err != nil {
				return
			}
			var result any
			switch req.Method {
			case "ledger.getFrontierMomentum":
				if rejectFrontier != nil && rejectFrontier() {
					if err := conn.WriteJSON(map[string]any{"jsonrpc": "2.0", "id": req.ID, "error": map[string]any{"code": -32000, "message": "node busy"}}); err != nil {
						return
					}
					continue
				}
				result = map[string]any{"height": 42, "chainIdentifier": 1}
			case "ledger.subscribe":
				if !acknowledge {
					continue
				}
				result = "test-subscription"
			default:
				result = true
			}
			if err := conn.WriteJSON(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": result}); err != nil {
				return
			}
			if req.Method == "ledger.subscribe" {
				subscribed <- conn
			}
		}
	}))
	t.Cleanup(func() {
		socketsMu.Lock()
		closed = true
		for _, conn := range sockets {
			conn.Close()
		}
		socketsMu.Unlock()
		srv.Close()
	})
	return "ws" + strings.TrimPrefix(srv.URL, "http"), subscribed
}

func TestMomentumSubscriptionSetupTimesOut(t *testing.T) {
	old := momentumSubscribeTimeout
	momentumSubscribeTimeout = 100 * time.Millisecond
	t.Cleanup(func() { momentumSubscribeTimeout = old })
	n := newTestNode(t)
	url, _ := momentumTestServer(t, false)
	done := make(chan error, 1)
	go func() { done <- n.setNode(url) }()
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	deadline := time.After(5 * time.Second)
wait:
	for {
		select {
		case <-ticker.C:
			if n.NodeStatus().Connected {
				t.Fatal("candidate was exposed before subscription setup completed")
			}
		case err := <-done:
			if err == nil || !strings.Contains(err.Error(), "deadline exceeded") {
				t.Fatalf("want subscription timeout, got %v", err)
			}
			break wait
		case <-deadline:
			t.Fatal("subscription setup did not release the node transition")
		}
	}
	if n.NodeStatus().Connected || n.currentClient() != nil {
		t.Fatal("timed-out subscription left a connected client")
	}
}

func TestMomentumTransportLossClearsConnection(t *testing.T) {
	n := newTestNode(t)
	url, subscribed := momentumTestServer(t, true)
	if err := n.setNode(url); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { n.Disconnect() })
	conn := <-subscribed
	// Setup's cancelled context must not end the established subscription.
	if err := conn.WriteJSON(map[string]any{
		"jsonrpc": "2.0", "method": "ledger.subscription",
		"params": map[string]any{"subscription": "test-subscription", "result": []map[string]any{{"height": 43}}},
	}); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for n.NodeStatus().Height != 43 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if n.NodeStatus().Height != 43 {
		t.Fatal("established subscription did not deliver momentum")
	}
	conn.Close()
	deadline = time.Now().Add(3 * time.Second)
	for n.NodeStatus().Connected && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if st := n.NodeStatus(); st.Connected || st.Height != 0 || st.ChainID != 0 {
		t.Fatalf("lost subscription retained status: %+v", st)
	}
}
