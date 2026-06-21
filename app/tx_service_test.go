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
	if _, err := tx.ConfirmPublish(); err == nil {
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
	tx.pendingReq = SendRequest{ToAddress: "z1qzal6c5s9rjnnxd2z7dvdhjxpmmj4fmw56a0mz", Zts: types.ZnnTokenStandard.String(), Amount: "1"}
	if _, err := tx.ConfirmPublish(); err == nil {
		t.Fatal("expected mismatch error; tampered block must not publish")
	}
	if tx.pending != nil {
		t.Fatal("pending block must be cleared after a mismatch")
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
	tx.pendingReq = SendRequest{ToAddress: "z1qzal6c5s9rjnnxd2z7dvdhjxpmmj4fmw56a0mz", Zts: types.ZnnTokenStandard.String(), Amount: "1"}
	_, err := tx.ConfirmPublish()
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
	tx.pendingReq = SendRequest{ToAddress: addr, Zts: types.ZnnTokenStandard.String(), Amount: "1"}
	_, err := tx.ConfirmPublish()
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
	tx.pendingReq = SendRequest{ToAddress: addr, Zts: types.ZnnTokenStandard.String(), Amount: "1"}
	// Lock the wallet: activeAddress() becomes !ok and the session advances.
	if err := tx.wallet.Lock(); err != nil {
		t.Fatalf("Lock: %v", err)
	}
	_, err := tx.ConfirmPublish()
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

func TestReceiveRejectsBadHash(t *testing.T) {
	tx := newTestTxService(t)
	if _, err := tx.Receive("not-a-hash"); err == nil {
		t.Fatal("expected error for invalid hash")
	}
}

func TestSymbolFor(t *testing.T) {
	tx := newTestTxService(t)
	if tx.symbolFor(types.ZnnTokenStandard.String()) != "ZNN" || tx.symbolFor(types.QsrTokenStandard.String()) != "QSR" {
		t.Fatal("ZNN/QSR symbols wrong")
	}
}
