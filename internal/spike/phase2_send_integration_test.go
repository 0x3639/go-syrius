//go:build integration

package spike

import (
	"math/big"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/0x3639/go-syrius/app"
	"github.com/0x3639/znn-sdk-go/rpc_client"
	"github.com/zenon-network/go-zenon/common/types"
	gzwallet "github.com/zenon-network/go-zenon/wallet"
)

// TestPhase2SendReceive exercises THIS repo's send/receive pipeline (the app.*
// services: ConfigService, WalletService, NodeService, TxService) end to end
// against a Zenon testnet node.
//
// It is build-tagged (`integration`) and skips cleanly when the required env or
// secrets are absent, so the default offline suite (`go test ./...`) never runs
// it. The module path used in the import above is the one declared in go.mod;
// confirm it if the import fails to resolve.
//
// Env:
//
//	ZNN_TESTNET_URL        testnet node URL (required; else the test skips)
//	ZNN_KEYSTORE           path to a syrius keystore (default secrets/pillar.json)
//	ZNN_KEYSTORE_PASSWORD  keystore password (else read from secrets/pillar-password.txt)
//	ZNN_SEND_TO            recipient z1… (default: the wallet's own address — self-send)
//	ZNN_EXPECT_CHAINID     expected chain id; refuse to send otherwise (default 73404)
//	ZNN_POW_ACCOUNT_INDEX  derivation index of an UNFUSED account (no plasma) for
//	                       the Gate-2 PoW send (default 1; index 0 is the funded/fused one)
//
// IMPORTANT — the node MUST expose the `embedded` RPC namespace. zenon.PrepareBlock
// (used by TxService.PrepareSend / Receive) calls
// embedded.plasma.getRequiredPoWForAccountBlock; a node without `embedded`
// enabled will fail autofill. A live pass of this test is therefore PENDING a
// testnet node with the embedded namespace enabled.

const (
	defaultExpectChainID = uint64(73404) // Zenon testnet
	defaultKeystorePath  = "../../secrets/pillar.json"
	defaultPasswordPath  = "../../secrets/pillar-password.txt"
	defaultPoWIndex      = 1 // index 0 is assumed funded/fused; index 1 is the unfused PoW account
	confirmTimeout       = 120 * time.Second
	pollInterval         = 3 * time.Second
)

// testEnv holds the resolved configuration for a run, or signals a skip.
type testEnv struct {
	url           string
	keystorePath  string
	password      string
	sendTo        string // "" ⇒ self-send
	expectChainID uint64
	powIndex      int
}

// resolveEnv reads the env/secrets. It calls t.Skip (never t.Fatal) when the
// prerequisites for a live run are absent, so the suite is a no-op offline.
func resolveEnv(t *testing.T) testEnv {
	t.Helper()

	url := os.Getenv("ZNN_TESTNET_URL")
	if url == "" {
		t.Skip("set ZNN_TESTNET_URL to run the Phase 2 testnet integration test")
	}

	keystorePath := os.Getenv("ZNN_KEYSTORE")
	if keystorePath == "" {
		keystorePath = defaultKeystorePath
	}
	if _, err := os.Stat(keystorePath); err != nil {
		t.Skipf("keystore %q not present (set ZNN_KEYSTORE or provide %s): %v", keystorePath, defaultKeystorePath, err)
	}

	password := os.Getenv("ZNN_KEYSTORE_PASSWORD")
	if password == "" {
		raw, err := os.ReadFile(defaultPasswordPath)
		if err != nil {
			t.Skip("no keystore password (set ZNN_KEYSTORE_PASSWORD or provide secrets/pillar-password.txt)")
		}
		password = strings.TrimSpace(string(raw))
	}

	expectChainID := defaultExpectChainID
	if v := os.Getenv("ZNN_EXPECT_CHAINID"); v != "" {
		n, err := strconv.ParseUint(v, 10, 64)
		if err != nil {
			t.Fatalf("bad ZNN_EXPECT_CHAINID %q: %v", v, err)
		}
		expectChainID = n
	}

	powIndex := defaultPoWIndex
	if v := os.Getenv("ZNN_POW_ACCOUNT_INDEX"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			t.Fatalf("bad ZNN_POW_ACCOUNT_INDEX %q: %v", v, err)
		}
		powIndex = n
	}

	return testEnv{
		url:           url,
		keystorePath:  keystorePath,
		password:      password,
		sendTo:        os.Getenv("ZNN_SEND_TO"),
		expectChainID: expectChainID,
		powIndex:      powIndex,
	}
}

// assertTestnet opens a throwaway RPC client and refuses to proceed unless the
// node reports the expected (testnet) chain id. This is the safety guard that
// prevents broadcasting a funded-wallet tx against mainnet, mirroring the
// Phase-0 test's guard (TxService applies the same gate internally, but this
// up-front check fails the test loudly before any block is built).
func assertTestnet(t *testing.T, env testEnv) {
	t.Helper()
	client, err := rpc_client.NewRpcClient(env.url)
	if err != nil {
		t.Fatalf("NewRpcClient(%q): %v", env.url, err)
	}
	defer client.Stop()

	m, err := client.LedgerApi.GetFrontierMomentum()
	if err != nil {
		t.Fatalf("GetFrontierMomentum: %v", err)
	}
	if m.ChainIdentifier != env.expectChainID {
		t.Fatalf("refusing to run: node chainId %d != expected %d — wrong ZNN_TESTNET_URL? "+
			"(this guard prevents broadcasting a funded-wallet tx against mainnet)",
			m.ChainIdentifier, env.expectChainID)
	}
	t.Logf("connected to chainId %d (height %d) — testnet confirmed", m.ChainIdentifier, m.Height)
}

// buildApp constructs the real app services exactly as the app does (app.New),
// pointed at a temp data dir, with the keystore imported and unlocked at the
// given account index. It returns the wired *app.App and a direct RPC client
// used only for read-side polling (confirmation by hash) that the services do
// not surface publicly.
func buildApp(t *testing.T, env testEnv, accountIndex int) (*app.App, *rpc_client.RpcClient) {
	t.Helper()

	// Isolate persisted state to a temp dir (ConfigService honors this env).
	dataDir := t.TempDir()
	t.Setenv("GO_SYRIUS_DATA_DIR", dataDir)

	a := app.New()

	// Import the keystore into the app's wallets dir under the temp data dir,
	// then unlock it through the real WalletService. Unlock is keyed by the
	// wallet ID (the uuid keystore filename the import assigned).
	meta, err := a.Wallet.ImportKeystore(env.keystorePath, "")
	if err != nil {
		t.Fatalf("Wallet.ImportKeystore(%q): %v", env.keystorePath, err)
	}
	if err := a.Wallet.Unlock(meta.ID, env.password); err != nil {
		t.Fatalf("Wallet.Unlock(%q): %v", meta.ID, err)
	}
	if accountIndex != 0 {
		if _, err := a.Wallet.SelectAccount(accountIndex); err != nil {
			t.Fatalf("Wallet.SelectAccount(%d): %v", accountIndex, err)
		}
	}

	// Connect the NodeService through the serialized transition path, exactly
	// as the running app does: persist the remote URL (the default mode), which
	// reconnects and starts the momentum subscription. The raw connector is
	// intentionally not exported.
	if err := a.Node.SetNodeURL("remote", env.url); err != nil {
		t.Fatalf("Node.SetNodeURL(remote, %q): %v", env.url, err)
	}

	client, err := rpc_client.NewRpcClient(env.url)
	if err != nil {
		t.Fatalf("read-side NewRpcClient: %v", err)
	}

	t.Cleanup(func() {
		_ = a.Node.Disconnect()
		_ = a.Wallet.Lock()
		client.Stop()
	})
	return a, client
}

// activeAddressFor decrypts the keystore directly (canonical go-zenon reader) to
// learn the address at accountIndex without depending on app internals. Used for
// self-send defaults and balance lookups.
func activeAddressFor(t *testing.T, env testEnv, accountIndex int) types.Address {
	t.Helper()
	kf, err := gzwallet.ReadKeyFile(env.keystorePath)
	if err != nil {
		t.Fatalf("go-zenon ReadKeyFile: %v", err)
	}
	ks, err := kf.Decrypt(env.password)
	if err != nil {
		t.Fatalf("go-zenon Decrypt: %v", err)
	}
	defer ks.Zero()
	_, kp, err := ks.DeriveForIndexPath(uint32(accountIndex))
	if err != nil {
		t.Fatalf("DeriveForIndexPath(%d): %v", accountIndex, err)
	}
	return kp.Address
}

// pollConfirmed waits until the block at hash has a ConfirmationDetail.
func pollConfirmed(t *testing.T, client *rpc_client.RpcClient, hash types.Hash) {
	t.Helper()
	deadline := time.Now().Add(confirmTimeout)
	for {
		got, err := client.LedgerApi.GetAccountBlockByHash(hash)
		if err == nil && got != nil && got.ConfirmationDetail != nil {
			t.Logf("block %s confirmed at momentum height %d", hash, got.ConfirmationDetail.MomentumHeight)
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("block %s not confirmed within %s", hash, confirmTimeout)
		}
		time.Sleep(pollInterval)
	}
}

// hashOf parses a hex hash string, failing the test on error.
func hashOf(t *testing.T, s string) types.Hash {
	t.Helper()
	h, err := types.HexToHash(s)
	if err != nil {
		t.Fatalf("HexToHash(%q): %v", s, err)
	}
	return h
}

// TestPhase2SendReceive drives the three Gate-2 subtests through THIS repo's
// services against testnet. Each subtest is independent and skips gracefully
// when its preconditions (funds, unreceived blocks) are absent.
func TestPhase2SendReceive(t *testing.T) {
	env := resolveEnv(t)
	assertTestnet(t, env)

	// (a) Send: PrepareSend → ConfirmPublish for a 0.1 ZNN self-send, confirmed
	// on-chain via GetAccountBlockByHash.ConfirmationDetail.
	t.Run("Send", func(t *testing.T) {
		a, client := buildApp(t, env, 0)

		to := env.sendTo
		if to == "" {
			to = activeAddressFor(t, env, 0).String()
		}
		req := app.SendRequest{
			ToAddress: to,
			Zts:       types.ZnnTokenStandard.String(),
			Amount:    big.NewInt(10_000_000).String(), // 0.1 ZNN (8 decimals)
		}

		preview, err := a.Tx.PrepareSend(req)
		if err != nil {
			t.Fatalf("Tx.PrepareSend: %v", err)
		}
		// PoW/hashing is deferred to ConfirmPublish, so the preview carries the
		// approved effect + hold identity, not a hash.
		t.Logf("prepared: from=%s to=%s amount=%s needsPoW=%v holdId=%d",
			preview.FromAddress, preview.ToAddress, preview.Amount, preview.NeedsPoW, preview.HoldID)

		hash, err := a.Tx.ConfirmPublish(preview.HoldID)
		if err != nil {
			t.Fatalf("Tx.ConfirmPublish: %v", err)
		}
		if hash == "" {
			t.Fatal("ConfirmPublish returned an empty hash")
		}
		pollConfirmed(t, client, hashOf(t, hash))
	})

	// (b) Receive: receive one unreceived block, skipping if none are available.
	t.Run("Receive", func(t *testing.T) {
		a, client := buildApp(t, env, 0)

		unreceived, err := a.Node.GetUnreceived()
		if err != nil {
			t.Fatalf("Node.GetUnreceived: %v", err)
		}
		if len(unreceived) == 0 {
			t.Skip("no unreceived blocks available to receive (send to this address first)")
		}
		fromHash := unreceived[0].FromHash
		t.Logf("receiving block from=%s amount=%s token=%s (fromHash=%s)",
			unreceived[0].FromAddress, unreceived[0].Amount, unreceived[0].Token, fromHash)

		recvHash, err := a.Tx.Receive(fromHash)
		if err != nil {
			t.Fatalf("Tx.Receive(%s): %v", fromHash, err)
		}
		pollConfirmed(t, client, hashOf(t, recvHash))
	})

	// (c) Gate-2 PoW send: from an UNFUSED account (no plasma) so RequiresPoW is
	// true and the published block carries Difficulty>0. If that account holds no
	// ZNN, skip with a clear message (do not fail).
	t.Run("PoWSend", func(t *testing.T) {
		a, client := buildApp(t, env, env.powIndex)

		powAddr := activeAddressFor(t, env, env.powIndex)
		t.Logf("PoW account index=%d address=%s", env.powIndex, powAddr)

		req := app.SendRequest{
			ToAddress: powAddr.String(), // self-send keeps funds on the same account
			Zts:       types.ZnnTokenStandard.String(),
			Amount:    big.NewInt(10_000_000).String(), // 0.1 ZNN
		}

		// Confirm this account genuinely requires PoW (no fused plasma). If it
		// doesn't, the test's premise is invalid — surface that, don't silently pass.
		needsPoW, err := a.Tx.RequiresPoW(req)
		if err != nil {
			t.Fatalf("Tx.RequiresPoW: %v", err)
		}
		if !needsPoW {
			t.Skipf("account index %d does not require PoW (it has fused plasma); "+
				"set ZNN_POW_ACCOUNT_INDEX to a genuinely unfused account", env.powIndex)
		}

		preview, err := a.Tx.PrepareSend(req)
		if err != nil {
			// A no-funds account fails at autofill (no frontier/insufficient
			// balance). Skip rather than fail, per the brief.
			if isNoFundsErr(err) {
				t.Skipf("PoW account index %d appears to have no ZNN to send: %v", env.powIndex, err)
			}
			t.Fatalf("Tx.PrepareSend (PoW path): %v", err)
		}
		// PoW is deferred to ConfirmPublish; at prepare time only NeedsPoW is
		// known. The on-chain Difficulty is asserted after confirmation below.
		if !preview.NeedsPoW {
			t.Fatalf("expected a PoW block (NeedsPoW=true), got needsPoW=%v", preview.NeedsPoW)
		}
		t.Logf("PoW block prepared: holdId=%d", preview.HoldID)

		hash, err := a.Tx.ConfirmPublish(preview.HoldID)
		if err != nil {
			// If the account simply lacks ZNN to spend, skip rather than fail. The
			// canonical on-chain PoW proof is TestGate2PoWReceive, which self-funds.
			if isNoFundsErr(err) {
				t.Skipf("PoW account index %d has no spendable ZNN: %v", env.powIndex, err)
			}
			t.Fatalf("Tx.ConfirmPublish (PoW path): %v", err)
		}
		pollConfirmed(t, client, hashOf(t, hash))
		// The published block must genuinely carry PoW.
		got, err := client.LedgerApi.GetAccountBlockByHash(hashOf(t, hash))
		if err != nil {
			t.Fatalf("GetAccountBlockByHash(%s): %v", hash, err)
		}
		if got == nil || got.Difficulty == 0 {
			t.Fatalf("published block must carry Difficulty>0 (PoW), got %+v", got)
		}
	})
}

// isNoFundsErr heuristically classifies an error as "this account has nothing to
// send" so the PoW subtest can skip rather than fail. It matches on substrings
// because the SDK/go-zenon surface these as plain text errors.
func isNoFundsErr(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	// Balance-specific substrings only — avoid broad terms like "not found"/"empty"
	// that could mask a genuine node/RPC error as a benign no-funds skip.
	for _, needle := range []string{"insufficient", "balance", "no frontier"} {
		if strings.Contains(msg, needle) {
			return true
		}
	}
	return false
}
