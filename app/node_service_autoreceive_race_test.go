package app

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/0x3639/znn-sdk-go/rpc_client"
	"github.com/gorilla/websocket"
)

// startWSTestServer runs a minimal JSON-RPC-over-WebSocket server that answers
// every request with a generic success, just enough for rpc_client.NewRpcClient
// (which dials at construction) to connect. Closed via t.Cleanup.
func startWSTestServer(t *testing.T) (wsURL string) {
	t.Helper()
	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		for {
			var req map[string]interface{}
			if err := conn.ReadJSON(&req); err != nil {
				return
			}
			if err := conn.WriteJSON(map[string]interface{}{
				"jsonrpc": "2.0", "id": req["id"], "result": "ok",
			}); err != nil {
				return
			}
		}
	}))
	t.Cleanup(srv.Close)
	return "ws" + strings.TrimPrefix(srv.URL, "http")
}

// TestStartAutoReceiveErrorsWhenSubscribeRacesDisconnect is the regression test
// for the P1 found in the znn-sdk-go v0.3.0 upgrade review. StartAutoReceive
// snapshots n.client and then subscribes WITHOUT holding the node-transition
// lock, so a concurrent Disconnect/SetNodeMode can Stop() the snapshotted
// client between those two steps. SDK v0.3.0's Stop() detaches SubscriberApi's
// underlying client (SetClient(nil)), and before v0.3.1 the To* methods
// dereferenced it — panicking the whole app. v0.2.1 (closed, non-nil client)
// and v0.3.1+ (api.ErrNotConnected nil-guard) both return an error instead.
//
// The test constructs the exact interleaving state deterministically: the
// client StartAutoReceive will snapshot is stopped before the subscribe runs.
func TestStartAutoReceiveErrorsWhenSubscribeRacesDisconnect(t *testing.T) {
	client, err := rpc_client.NewRpcClient(startWSTestServer(t))
	if err != nil {
		t.Fatalf("NewRpcClient: %v", err)
	}

	w := newTestWalletService(t)
	unlockTestWallet(t, w) // activeAddress must succeed so we reach the subscribe
	n := newNodeService(newTestConfig(t), w)
	n.mu.Lock()
	n.client = client
	n.mu.Unlock()

	// The concurrent disconnect wins the race: the installed client is stopped,
	// but n.client still points at it — the state StartAutoReceive observes when
	// Stop() lands between its snapshot and its subscribe.
	client.Stop()

	if err := n.StartAutoReceive(); err == nil {
		t.Fatal("StartAutoReceive on a stopped client returned nil error, want not-connected error")
	}

	// The failed start must release the running slot so a later StartAutoReceive
	// can retry instead of seeing a phantom "already running".
	n.mu.Lock()
	slotHeld := n.autoStop != nil
	n.mu.Unlock()
	if slotHeld {
		t.Fatal("failed StartAutoReceive left the auto-receive slot claimed")
	}
}
