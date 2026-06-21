package app

import (
	"math/big"
	"testing"

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

func TestSymbolFor(t *testing.T) {
	tx := newTestTxService(t)
	if tx.symbolFor(types.ZnnTokenStandard.String()) != "ZNN" || tx.symbolFor(types.QsrTokenStandard.String()) != "QSR" {
		t.Fatal("ZNN/QSR symbols wrong")
	}
}
