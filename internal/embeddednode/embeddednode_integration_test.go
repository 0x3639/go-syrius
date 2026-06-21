//go:build integration

package embeddednode

import (
	"testing"
	"time"

	"github.com/0x3639/znn-sdk-go/rpc_client"
)

// TestStartStop spins up a real mainnet embedded node, confirms its WS RPC
// answers, then stops it. Heavy (downloads peers/genesis); opt-in.
func TestStartStop(t *testing.T) {
	h, err := Start(t.TempDir())
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer h.Stop()

	client, err := rpc_client.NewRpcClient(h.WSURL())
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer client.Stop()

	deadline := time.Now().Add(30 * time.Second)
	for {
		if _, err := client.StatsApi.SyncInfo(); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("SyncInfo never answered")
		}
		time.Sleep(time.Second)
	}
	if err := h.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}
}
