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
// the real services land, and it fails to build if the facade signatures
// change. It performs NO network I/O.
//
// go-syrius uses the SDK for RPC (rpc_client), the send orchestration (zenon),
// and the wallet.KeyPair type that zenon.Send consumes for signing. Keystore
// reading/derivation is NOT done via the SDK — that goes through go-zenon's
// wallet directly (see internal/compat), because the SDK cannot read real
// syrius keystores.
func TestSDKFacadePresent(t *testing.T) {
	// zenon.NewZenon builds a stateless wrapper; a nil client is safe because
	// we never invoke a method that touches the network.
	if z := zenon.NewZenon(nil); z == nil {
		t.Fatal("zenon.NewZenon returned nil")
	}

	// Reference (do not call) the constructors go-syrius relies on. Calling
	// NewRpcClient would open a websocket; referencing pins the signatures and
	// anchors the import. NewKeyStoreFromMnemonic is how a signing KeyPair is
	// built for zenon.Send from a mnemonic recovered via go-zenon.
	_ = rpc_client.NewRpcClient
	_ = wallet.NewKeyStoreFromMnemonic
}
