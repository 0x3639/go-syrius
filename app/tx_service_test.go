package app

import (
	"math/big"
	"strings"
	"testing"

	"github.com/0x3639/znn-sdk-go/rpc_client"
	"github.com/zenon-network/go-zenon/chain/nom"
	"github.com/zenon-network/go-zenon/common/types"
)

func newTestTxService(t *testing.T) *TxService {
	t.Helper()
	t.Setenv("GO_SYRIUS_DATA_DIR", t.TempDir())
	cfg := newConfigService()
	w := newWalletService(cfg)
	n := newNodeService(cfg, w)
	return newTxService(cfg, w, n)
}

func TestPrepareSendRejectsBadAddress(t *testing.T) {
	tx := newTestTxService(t)
	if _, err := tx.PrepareSend(SendRequest{ToAddress: "not-an-address", Zts: types.ZnnTokenStandard.String(), Amount: "1"}); err == nil {
		t.Fatal("expected invalid-address error")
	}
}

func TestConfirmPublishNoPending(t *testing.T) {
	tx := newTestTxService(t)
	if _, err := tx.ConfirmPublish(0); err == nil {
		t.Fatal("expected error when no pending transaction")
	}
}

func TestConfirmPublishRejectsTamperedBlock(t *testing.T) {
	tx := newTestTxService(t)
	unlockTestWallet(t, tx.wallet)
	tx.pendingGen = tx.wallet.sessionGen()
	// Simulate a held block that disagrees with the recorded request.
	tx.pending = &nom.AccountBlock{
		ToAddress:     types.ParseAddressPanic("z1qqjnwjjpnue8xmmpanz6csze6tcmtzzdtfsww7"),
		Amount:        big.NewInt(999),
		TokenStandard: types.ZnnTokenStandard,
	}
	exTo, _ := types.ParseAddress("z1qzal6c5s9rjnnxd2z7dvdhjxpmmj4fmw56a0mz")
	tx.pendingExpect = callExpect{to: exTo, zts: types.ZnnTokenStandard, amount: big.NewInt(1)}
	tx.pendingHoldID = 1
	if _, err := tx.ConfirmPublish(1); err == nil {
		t.Fatal("expected mismatch error; tampered block must not publish")
	}
	if tx.pending != nil {
		t.Fatal("pending block must be cleared after a mismatch")
	}
}

func TestConfirmPublishFailsClosedOnZeroHoldID(t *testing.T) {
	tx := newTestTxService(t)
	unlockTestWallet(t, tx.wallet)
	tx.holdPending(&nom.AccountBlock{
		ToAddress:     types.ParseAddressPanic("z1qzal6c5s9rjnnxd2z7dvdhjxpmmj4fmw56a0mz"),
		Amount:        big.NewInt(1),
		TokenStandard: types.ZnnTokenStandard,
	}, callExpect{}, tx.wallet.sessionGen())
	// Every real preview carries a non-zero id; 0 means the frontend lost track
	// of what it is confirming — the gate must fail closed, not skip the check.
	_, err := tx.ConfirmPublish(0)
	if err == nil || !strings.Contains(err.Error(), "changed since it was displayed") {
		t.Fatalf("expected fail-closed identity refusal for holdId 0, got %v", err)
	}
	if tx.pending == nil {
		t.Fatal("a refused confirm must not clear the held block")
	}
}

func TestConfirmPublishMatchingHoldIDPassesTheGate(t *testing.T) {
	tx := newTestTxService(t)
	unlockTestWallet(t, tx.wallet)
	tx.node.chainID = 3
	const addr = "z1qzal6c5s9rjnnxd2z7dvdhjxpmmj4fmw56a0mz"
	exTo, _ := types.ParseAddress(addr)
	block := &nom.AccountBlock{
		ToAddress:       types.ParseAddressPanic(addr),
		Amount:          big.NewInt(1),
		TokenStandard:   types.ZnnTokenStandard,
		ChainIdentifier: 3,
	}
	id := tx.holdPending(block, callExpect{to: exTo, zts: types.ZnnTokenStandard, amount: big.NewInt(1)}, tx.wallet.sessionGen())
	// Offline test: the confirm must get PAST the identity gate and fail later
	// on the missing node connection — a gate regression that refuses valid
	// matching ids (bricking every real publish) fails here.
	_, err := tx.ConfirmPublish(id)
	if err == nil {
		t.Fatal("expected a downstream (not-connected) error in this offline test")
	}
	if strings.Contains(err.Error(), "changed since it was displayed") {
		t.Fatalf("matching holdId must pass the identity gate, got %v", err)
	}
	if !strings.Contains(err.Error(), "not connected") {
		t.Fatalf("expected the not-connected error downstream of the gate, got %v", err)
	}
}

func TestConfirmPublishRejectsMismatchedHoldID(t *testing.T) {
	tx := newTestTxService(t)
	unlockTestWallet(t, tx.wallet)
	// Hold a block as holdId 1; the caller confirms against a preview for a
	// DIFFERENT hold (e.g. a slower prepare replaced the slot after the dialog
	// was displayed). Confirm-what-you-sign: must refuse, must NOT clear the
	// (valid, newer) hold.
	tx.holdPending(&nom.AccountBlock{
		ToAddress:     types.ParseAddressPanic("z1qqjnwjjpnue8xmmpanz6csze6tcmtzzdtfsww7"),
		Amount:        big.NewInt(1),
		TokenStandard: types.ZnnTokenStandard,
	}, callExpect{}, tx.wallet.sessionGen())
	_, err := tx.ConfirmPublish(tx.pendingHoldID + 1)
	if err == nil || !strings.Contains(err.Error(), "changed since it was displayed") {
		t.Fatalf("expected hold-identity refusal, got %v", err)
	}
	if tx.pending == nil {
		t.Fatal("a mismatched confirm must not clear the held block")
	}
}

func TestCancelPendingIdentityChecked(t *testing.T) {
	tx := newTestTxService(t)
	unlockTestWallet(t, tx.wallet)
	id := tx.holdPending(&nom.AccountBlock{TokenStandard: types.ZnnTokenStandard, Amount: big.NewInt(1)}, callExpect{}, tx.wallet.sessionGen())
	// A stale cancel (different id) must not release the hold…
	if err := tx.CancelPending(id + 100); err != nil {
		t.Fatalf("CancelPending: %v", err)
	}
	if tx.pending == nil {
		t.Fatal("stale cancel must not release a newer hold")
	}
	// …the matching cancel must.
	if err := tx.CancelPending(id); err != nil {
		t.Fatalf("CancelPending: %v", err)
	}
	if tx.pending != nil {
		t.Fatal("matching cancel must release the hold")
	}
}

func TestConfirmPublishRejectsConcurrent(t *testing.T) {
	tx := newTestTxService(t)
	tx.publishMu.Lock() // simulate a confirm already in flight
	defer tx.publishMu.Unlock()
	if _, err := tx.ConfirmPublish(0); err == nil {
		t.Fatal("expected a concurrent ConfirmPublish to be rejected")
	}
}

func TestConfirmPublishBlockedOnMainnet(t *testing.T) {
	tx := newTestTxService(t)
	unlockTestWallet(t, tx.wallet)
	tx.pendingGen = tx.wallet.sessionGen()
	// Simulate being connected to mainnet with mainnet sending disabled (default).
	tx.node.chainID = mainnetChainID
	tx.pending = &nom.AccountBlock{
		ToAddress:     types.ParseAddressPanic("z1qzal6c5s9rjnnxd2z7dvdhjxpmmj4fmw56a0mz"),
		Amount:        big.NewInt(1),
		TokenStandard: types.ZnnTokenStandard,
	}
	exTo, _ := types.ParseAddress("z1qzal6c5s9rjnnxd2z7dvdhjxpmmj4fmw56a0mz")
	tx.pendingExpect = callExpect{to: exTo, zts: types.ZnnTokenStandard, amount: big.NewInt(1)}
	_, err := tx.ConfirmPublish(0)
	if err == nil {
		t.Fatal("expected mainnet guard error; block prepared on another chain must not publish")
	}
	if !strings.Contains(err.Error(), "mainnet") {
		t.Fatalf("expected error to mention mainnet, got: %v", err)
	}
	if tx.pending == nil {
		t.Fatal("pending block must NOT be cleared when blocked by the mainnet guard, so the user can reconnect and retry")
	}
}

func TestConfirmPublishRejectsChainMismatch(t *testing.T) {
	tx := newTestTxService(t)
	unlockTestWallet(t, tx.wallet)
	tx.pendingGen = tx.wallet.sessionGen()
	// A non-nil client makes the connected check pass; the chain check fires before
	// any client method is touched, so the empty client is never dereferenced.
	tx.node.client = &rpc_client.RpcClient{}
	// Two distinct non-mainnet chain ids: the block was prepared on one, but the
	// node is now connected to another. Guard passes (neither is mainnet).
	tx.node.chainID = 12
	const addr = "z1qzal6c5s9rjnnxd2z7dvdhjxpmmj4fmw56a0mz"
	tx.pending = &nom.AccountBlock{
		ToAddress:       types.ParseAddressPanic(addr),
		Amount:          big.NewInt(1),
		TokenStandard:   types.ZnnTokenStandard,
		ChainIdentifier: 3, // testnet id different from the connected node's
	}
	exTo, _ := types.ParseAddress(addr)
	tx.pendingExpect = callExpect{to: exTo, zts: types.ZnnTokenStandard, amount: big.NewInt(1)}
	tx.pendingHoldID = 1
	_, err := tx.ConfirmPublish(1)
	if err == nil {
		t.Fatal("expected chain-mismatch error; cross-chain block must not publish")
	}
	if !strings.Contains(err.Error(), "chain") {
		t.Fatalf("expected error to mention chain, got: %v", err)
	}
	if tx.pending == nil {
		t.Fatal("pending must be retained on chain mismatch so the user can reconnect and retry")
	}
}

func TestConfirmPublishRejectsWhenLocked(t *testing.T) {
	tx := newTestTxService(t)
	unlockTestWallet(t, tx.wallet)
	tx.pendingGen = tx.wallet.sessionGen()
	tx.node.chainID = 3
	const addr = "z1qzal6c5s9rjnnxd2z7dvdhjxpmmj4fmw56a0mz"
	tx.pending = &nom.AccountBlock{
		ToAddress:       types.ParseAddressPanic(addr),
		Amount:          big.NewInt(1),
		TokenStandard:   types.ZnnTokenStandard,
		ChainIdentifier: 3,
	}
	exTo, _ := types.ParseAddress(addr)
	tx.pendingExpect = callExpect{to: exTo, zts: types.ZnnTokenStandard, amount: big.NewInt(1)}
	tx.pendingHoldID = 1
	// Lock the wallet: activeAddress() becomes !ok and the session advances.
	if err := tx.wallet.Lock(); err != nil {
		t.Fatalf("Lock: %v", err)
	}
	_, err := tx.ConfirmPublish(1)
	if err == nil {
		t.Fatal("expected error; a locked wallet must not publish")
	}
	if tx.pending != nil {
		t.Fatal("pending must be cleared when the wallet is locked")
	}
}

func TestLockClearsPending(t *testing.T) {
	tx := newTestTxService(t)
	// Wire the App-style callback so locking the wallet clears the held block.
	tx.wallet.setOnLock(tx.clearPending)
	tx.pending = &nom.AccountBlock{
		ToAddress:     types.ParseAddressPanic("z1qzal6c5s9rjnnxd2z7dvdhjxpmmj4fmw56a0mz"),
		Amount:        big.NewInt(1),
		TokenStandard: types.ZnnTokenStandard,
	}
	if err := tx.wallet.Lock(); err != nil {
		t.Fatalf("Lock returned error: %v", err)
	}
	if tx.pending != nil {
		t.Fatal("pending block must be cleared when the wallet is locked")
	}
}

func TestAssertMatches(t *testing.T) {
	to, _ := types.ParseAddress("z1qzal6c5s9rjnnxd2z7dvdhjxpmmj4fmw56a0mz")
	other, _ := types.ParseAddress("z1qr4pexnnfaexqqz8nscjjcsajy5hdqfkgadvwx")
	e := callExpect{to: to, zts: types.QsrTokenStandard, amount: big.NewInt(100)}

	ok := &nom.AccountBlock{ToAddress: to, TokenStandard: types.QsrTokenStandard, Amount: big.NewInt(100)}
	if err := assertMatches(ok, e); err != nil {
		t.Fatalf("matching block should pass: %v", err)
	}
	for _, bad := range []*nom.AccountBlock{
		{ToAddress: other, TokenStandard: types.QsrTokenStandard, Amount: big.NewInt(100)},
		{ToAddress: to, TokenStandard: types.ZnnTokenStandard, Amount: big.NewInt(100)},
		{ToAddress: to, TokenStandard: types.QsrTokenStandard, Amount: big.NewInt(99)},
		{ToAddress: to, TokenStandard: types.QsrTokenStandard, Amount: nil},
	} {
		if err := assertMatches(bad, e); err == nil {
			t.Fatalf("divergent block must be rejected: %+v", bad)
		}
	}

	// Contract-call Data must also match: identical to/zts/amount but different
	// Data (e.g. a tampered Fuse beneficiary) must be rejected.
	ed := callExpect{to: to, zts: types.QsrTokenStandard, amount: big.NewInt(100), data: []byte{1, 2, 3}}
	okData := &nom.AccountBlock{ToAddress: to, TokenStandard: types.QsrTokenStandard, Amount: big.NewInt(100), Data: []byte{1, 2, 3}}
	if err := assertMatches(okData, ed); err != nil {
		t.Fatalf("matching Data block should pass: %v", err)
	}
	badData := &nom.AccountBlock{ToAddress: to, TokenStandard: types.QsrTokenStandard, Amount: big.NewInt(100), Data: []byte{1, 2, 4}}
	if err := assertMatches(badData, ed); err == nil {
		t.Fatal("block with divergent Data must be rejected")
	}
}

func TestConfiguredChainID(t *testing.T) {
	tx := newTestTxService(t)
	// Default settings (ChainID unset == 0) must normalize to mainnet.
	if got := tx.configuredChainID(); got != mainnetChainID {
		t.Fatalf("unset ChainID should normalize to mainnet (%d), got %d", mainnetChainID, got)
	}
	// A configured non-mainnet chain id must be returned verbatim.
	s, err := tx.config.GetSettings()
	if err != nil {
		t.Fatalf("GetSettings: %v", err)
	}
	s.ChainID = 73404
	if err := tx.config.SetSettings(s); err != nil {
		t.Fatalf("SetSettings: %v", err)
	}
	if got := tx.configuredChainID(); got != 73404 {
		t.Fatalf("configured ChainID 73404 should be returned, got %d", got)
	}
}

// TestConfiguredChainIDStampsTemplate proves the configured chain id reaches a
// built block's ChainIdentifier (the field committed in the signed block), as
// done at each of the three block-building sites before PrepareBlock/Send.
func TestConfiguredChainIDStampsTemplate(t *testing.T) {
	tx := newTestTxService(t)
	s, err := tx.config.GetSettings()
	if err != nil {
		t.Fatalf("GetSettings: %v", err)
	}
	s.ChainID = 73404
	if err := tx.config.SetSettings(s); err != nil {
		t.Fatalf("SetSettings: %v", err)
	}
	// Mirror the stamping step performed at every build site.
	template := &nom.AccountBlock{
		ToAddress:     types.ParseAddressPanic("z1qzal6c5s9rjnnxd2z7dvdhjxpmmj4fmw56a0mz"),
		Amount:        big.NewInt(1),
		TokenStandard: types.ZnnTokenStandard,
	}
	template.ChainIdentifier = tx.configuredChainID()
	if template.ChainIdentifier != 73404 {
		t.Fatalf("stamp should set ChainIdentifier to configured 73404, got %d", template.ChainIdentifier)
	}
}

func TestReceiveRejectsBadHash(t *testing.T) {
	tx := newTestTxService(t)
	if _, err := tx.Receive("not-a-hash"); err == nil {
		t.Fatal("expected error for invalid hash")
	}
}

const zeroHash = "0000000000000000000000000000000000000000000000000000000000000000"

func TestReceiveBlockedOnMainnet(t *testing.T) {
	tx := newTestTxService(t)
	// Non-nil client so the "not connected" check passes; the guard fires before
	// any client method, so the empty client is never dereferenced.
	tx.node.client = &rpc_client.RpcClient{}
	tx.node.chainID = mainnetChainID // mainnet, AllowMainnetSend false by default
	_, err := tx.Receive(zeroHash)
	if err == nil {
		t.Fatal("expected mainnet guard error; receive signs+publishes and must obey the guard")
	}
	if !strings.Contains(err.Error(), "mainnet") {
		t.Fatalf("expected error to mention mainnet, got: %v", err)
	}
}

func TestReceiveRejectsChainMismatch(t *testing.T) {
	tx := newTestTxService(t)
	tx.node.client = &rpc_client.RpcClient{}
	tx.node.chainID = 12 // connected chain
	// Configure a different non-mainnet chain id: guard passes, chain check fails.
	s, _ := tx.config.GetSettings()
	s.ChainID = 3
	if err := tx.config.SetSettings(s); err != nil {
		t.Fatalf("SetSettings: %v", err)
	}
	_, err := tx.Receive(zeroHash)
	if err == nil {
		t.Fatal("expected chain-mismatch error; receive must not publish onto the wrong chain")
	}
	if !strings.Contains(err.Error(), "chain") {
		t.Fatalf("expected error to mention chain, got: %v", err)
	}
}

func TestSymbolFor(t *testing.T) {
	tx := newTestTxService(t)
	if tx.symbolFor(types.ZnnTokenStandard.String()) != "ZNN" || tx.symbolFor(types.QsrTokenStandard.String()) != "QSR" {
		t.Fatal("ZNN/QSR symbols wrong")
	}
}
