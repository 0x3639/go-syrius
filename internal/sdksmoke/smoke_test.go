package sdksmoke

import (
	"testing"

	"github.com/0x3639/znn-sdk-go/rpc_client"
	"github.com/0x3639/znn-sdk-go/wallet"
	"github.com/0x3639/znn-sdk-go/zenon"
)

// TestSDKFacadePresent is a compile-time + light-runtime smoke test that the
// znn-sdk-go API surface go-syrius depends on exists at the pinned version
// (v0.1.16). It anchors the dependency so `go mod tidy` cannot drop it before
// the real services in Tasks 5/6 land, and it fails to build if the facade
// signatures change. It performs NO network I/O.
func TestSDKFacadePresent(t *testing.T) {
	// zenon.NewZenon builds a stateless wrapper; a nil client is safe because
	// we never invoke a method that touches the network.
	if z := zenon.NewZenon(nil); z == nil {
		t.Fatal("zenon.NewZenon returned nil")
	}

	// Keystore manager creation is local (filesystem only), no network.
	if _, err := wallet.NewKeyStoreManager(t.TempDir()); err != nil {
		t.Fatalf("wallet.NewKeyStoreManager: %v", err)
	}

	// Reference (do not call) the RPC client constructor — calling it would
	// open a websocket. Referencing pins the signature and anchors the import.
	_ = rpc_client.NewRpcClient
}
