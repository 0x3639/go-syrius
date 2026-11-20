package app

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/0x3639/znn-sdk-go/rpc_client"
	"github.com/gorilla/websocket"
	"github.com/zenon-network/go-zenon/chain/nom"
	api "github.com/zenon-network/go-zenon/rpc/api"
)

type autoReceiveSubscription struct {
	conn *momentumSocket
	id   string
}

type autoReceiveFixture struct {
	node       *NodeService
	requested  chan int
	subscribed chan autoReceiveSubscription
	reads      chan struct{}
}

func newAutoReceiveFixture(t *testing.T, blocks bool, beforeSubscribe func(int)) *autoReceiveFixture {
	t.Helper()
	f := &autoReceiveFixture{requested: make(chan int, 10), subscribed: make(chan autoReceiveSubscription, 10), reads: make(chan struct{}, 10)}
	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		conn := &momentumSocket{Conn: raw}
		defer conn.Close()
		count := 0
		for {
			var req struct {
				ID     any    `json:"id"`
				Method string `json:"method"`
			}
			if conn.ReadJSON(&req) != nil {
				return
			}
			var result any = true
			id := ""
			switch req.Method {
			case "ledger.subscribe":
				count++
				f.requested <- count
				if beforeSubscribe != nil {
					beforeSubscribe(count)
				}
				id = fmt.Sprintf("auto-%d", count)
				result = id
			case "ledger.getUnreceivedBlocksByAddress":
				f.reads <- struct{}{}
				list := []*api.AccountBlock{}
				if blocks {
					list = append(list, &api.AccountBlock{AccountBlock: nom.AccountBlock{Amount: big.NewInt(0)}})
				}
				result = map[string]any{"list": list, "count": len(list)}
			}
			if conn.WriteJSON(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": result}) != nil {
				return
			}
			if id != "" {
				f.subscribed <- autoReceiveSubscription{conn, id}
			}
		}
	}))
	t.Cleanup(srv.Close)
	client, err := rpc_client.NewRpcClient("ws" + strings.TrimPrefix(srv.URL, "http"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(client.Stop)
	n := newTestNode(t)
	n.wallet = newWalletService(n.config)
	unlockTestWallet(t, n.wallet)
	n.ctx = context.Background()
	n.emitEvent = func(context.Context, string, ...interface{}) {}
	n.client = client
	n.receiveFn = func(string) (string, error) { return "", nil }
	f.node = n
	t.Cleanup(func() { n.Disconnect() })
	return f
}

func awaitAutoReceiveSignal[T any](t *testing.T, ch <-chan T) T {
	t.Helper()
	select {
	case value := <-ch:
		return value
	case <-time.After(2 * time.Second):
		t.Fatal("auto-receive did not reach the expected step")
		var zero T
		return zero
	}
}

func TestAutoReceiveSetupCancellation(t *testing.T) {
	for _, action := range []string{"stop", "disconnect", "timeout"} {
		t.Run(action, func(t *testing.T) {
			release := make(chan struct{})
			f := newAutoReceiveFixture(t, false, func(int) { <-release })
			t.Cleanup(func() { close(release) })
			if action == "timeout" {
				old := autoReceiveSubscribeTimeout
				autoReceiveSubscribeTimeout = 100 * time.Millisecond
				t.Cleanup(func() { autoReceiveSubscribeTimeout = old })
			}
			done := make(chan error, 1)
			go func() { done <- f.node.StartAutoReceive() }()
			awaitAutoReceiveSignal(t, f.requested)
			switch action {
			case "stop":
				f.node.StopAutoReceive()
			case "disconnect":
				f.node.Disconnect()
			}
			err := awaitAutoReceiveSignal(t, done)
			if err == nil {
				t.Fatal("cancelled setup succeeded")
			}
			if action == "timeout" && !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("expected setup deadline, got %v", err)
			}
			f.node.mu.RLock()
			claimed := f.node.autoStop != nil || f.node.autoSetupCancel != nil
			f.node.mu.RUnlock()
			if claimed {
				t.Fatal("cancelled setup retained the worker slot")
			}
			select {
			case <-f.reads:
				t.Fatal("cancelled setup started a sweep")
			default:
			}
		})
	}
}

func TestAutoReceiveLateSetupCannotSweepOrRetireReplacement(t *testing.T) {
	release := make(chan struct{})
	f := newAutoReceiveFixture(t, false, func(count int) {
		if count == 1 {
			<-release
		}
	})
	t.Cleanup(func() {
		select {
		case <-release:
		default:
			close(release)
		}
	})
	old := make(chan error, 1)
	go func() { old <- f.node.StartAutoReceive() }()
	awaitAutoReceiveSignal(t, f.requested)
	f.node.StopAutoReceive()
	if err := awaitAutoReceiveSignal(t, old); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancelled setup, got %v", err)
	}
	close(release)
	awaitAutoReceiveSignal(t, f.subscribed) // the server's late acknowledgement
	if err := f.node.StartAutoReceive(); err != nil {
		t.Fatal(err)
	}
	current := awaitAutoReceiveSignal(t, f.subscribed)
	awaitAutoReceiveSignal(t, f.reads) // only the replacement's initial sweep
	// Start's setup context has been cancelled on return. The established
	// subscription must still deliver new-block notifications and trigger a read.
	if err := current.conn.WriteJSON(map[string]any{
		"jsonrpc": "2.0", "method": "ledger.subscription",
		"params": map[string]any{"subscription": current.id, "result": []map[string]any{}},
	}); err != nil {
		t.Fatal(err)
	}
	awaitAutoReceiveSignal(t, f.reads)
	f.node.mu.RLock()
	running := f.node.autoStop != nil
	f.node.mu.RUnlock()
	if !running {
		t.Fatal("replacement worker lost ownership")
	}
	select {
	case <-f.reads:
		t.Fatal("retired setup started an extra sweep")
	default:
	}
}

func TestRetiredAutoReceiveSweepCannotOverwriteActivity(t *testing.T) {
	f := newAutoReceiveFixture(t, true, nil)
	n := f.node
	oldEntered, newEntered := make(chan struct{}), make(chan struct{})
	oldRelease, newRelease := make(chan struct{}), make(chan struct{})
	oldDone, newDone := make(chan struct{}), make(chan struct{})
	var calls atomic.Int32
	n.receiveFn = func(string) (string, error) {
		if calls.Add(1) == 1 {
			close(oldEntered)
			<-oldRelease
			return "", errors.New("retired receive failed")
		}
		close(newEntered)
		<-newRelease
		return "", nil
	}
	t.Cleanup(func() {
		n.StopAutoReceive()
		select {
		case <-oldRelease:
		default:
			close(oldRelease)
		}
		select {
		case <-newRelease:
		default:
			close(newRelease)
		}
	})
	type event struct {
		name  string
		value any
	}
	events := make(chan event, 10)
	n.emitEvent = func(_ context.Context, name string, data ...interface{}) { events <- event{name, data[0]} }
	addr, _ := n.wallet.activeAddress()
	oldStop := make(chan struct{})
	n.autoStop = oldStop
	go func() { n.sweepUnreceived(n.currentClient(), addr, oldStop); close(oldDone) }()
	awaitAutoReceiveSignal(t, oldEntered)
	n.StopAutoReceive()
	newStop := make(chan struct{})
	n.mu.Lock()
	n.autoStop = newStop
	n.mu.Unlock()
	go func() { n.sweepUnreceived(n.currentClient(), addr, newStop); close(newDone) }()
	awaitAutoReceiveSignal(t, newEntered)
	for _, active := range []bool{true, false, true} {
		got := awaitAutoReceiveSignal(t, events)
		if got.name != EventAutoReceiving || got.value != active {
			t.Fatalf("unexpected activity event: %+v", got)
		}
	}
	close(oldRelease)
	awaitAutoReceiveSignal(t, oldDone)
	select {
	case got := <-events:
		t.Fatalf("retired sweep published %+v", got)
	default:
	}
	n.StopAutoReceive()
	got := awaitAutoReceiveSignal(t, events)
	if got.name != EventAutoReceiving || got.value != false {
		t.Fatalf("teardown did not clear activity: %+v", got)
	}
	close(newRelease)
	awaitAutoReceiveSignal(t, newDone)
}
