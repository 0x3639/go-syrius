package app

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// startStallingWSTestServer completes the WebSocket handshake but never answers
// any JSON-RPC request — the "half-open" peer a remote-node verification must
// not wait on forever.
func startStallingWSTestServer(t *testing.T) (wsURL string) {
	t.Helper()
	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return // swallow requests; never reply
			}
		}
	}))
	t.Cleanup(srv.Close)
	return "ws" + strings.TrimPrefix(srv.URL, "http")
}

// TestSetNodeVerificationTimesOutOnStalledPeer: setNode's mandatory frontier
// check runs while the node-transition mutex is held. A peer that accepts the
// connection but never responds must fail the transition within a bounded
// time instead of wedging every later node lifecycle operation.
func TestSetNodeVerificationTimesOutOnStalledPeer(t *testing.T) {
	old := remoteVerifyTimeout
	remoteVerifyTimeout = 500 * time.Millisecond
	t.Cleanup(func() { remoteVerifyTimeout = old })

	n := newTestNode(t)
	url := startStallingWSTestServer(t)

	done := make(chan error, 1)
	go func() { done <- n.setNode(url) }()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("setNode against a stalled peer returned nil, want timeout error")
		}
		if !strings.Contains(err.Error(), "timed out") {
			t.Fatalf("error should name the timeout, got: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("setNode still blocked 5s after a 500ms verification timeout: opMu-holding transition is unbounded")
	}
	if st := n.NodeStatus(); st.Connected {
		t.Fatalf("status must be disconnected after a timed-out verification, got %+v", st)
	}
}
