package app

import (
	"encoding/base64"
	"errors"
	"math/big"
	"strings"
	"testing"

	"github.com/zenon-network/go-zenon/chain/nom"
	"github.com/zenon-network/go-zenon/common/types"
	definition "github.com/zenon-network/go-zenon/vm/embedded/definition"
)

func validWalletConnectRequest(t *testing.T) (WalletConnectSendRequest, types.Address) {
	t.Helper()
	active := types.ParseAddressPanic("z1qzal6c5s9rjnnxd2z7dvdhjxpmmj4fmw56a0mz")
	data := definition.ABIBridge.PackMethodPanic(
		definition.WrapTokenMethodName,
		uint32(2),
		uint32(1),
		"0x1111111111111111111111111111111111111111",
	)
	return WalletConnectSendRequest{
		FromAddress: active.String(),
		Topic:       "test-topic",
		RequestID:   1,
		AccountBlock: WalletConnectAccountBlockInput{
			Version:         1,
			ChainIdentifier: mainnetChainID,
			BlockType:       uint64(nom.BlockTypeUserSend),
			Address:         types.ZeroAddress.String(),
			ToAddress:       types.BridgeContract.String(),
			Amount:          "100000000",
			TokenStandard:   types.ZnnTokenStandard.String(),
			Data:            base64.StdEncoding.EncodeToString(data),
		},
	}, active
}

func TestWalletConnectBridgeTemplateAcceptsCanonicalWrap(t *testing.T) {
	req, active := validWalletConnectRequest(t)
	template, expect, effect, err := walletConnectBridgeTemplate(req, active)
	if err != nil {
		t.Fatal(err)
	}
	if effect.Contract != "Bridge" || effect.Method != definition.WrapTokenMethodName {
		t.Fatalf("unexpected effect: %+v", effect)
	}
	if template.Address != active || template.ToAddress != types.BridgeContract || template.Amount.Cmp(big.NewInt(100000000)) != 0 {
		t.Fatalf("unexpected clean template: %+v", template)
	}
	if err := assertMatches(template, expect); err != nil {
		t.Fatalf("clean template does not match held effect: %v", err)
	}
}

func canonicalRedeemRequest(t *testing.T) (WalletConnectSendRequest, types.Address) {
	t.Helper()
	req, active := validWalletConnectRequest(t)
	data := definition.ABIBridge.PackMethodPanic(
		definition.RedeemUnwrapMethodName,
		types.Hash{},
		uint32(7),
	)
	req.AccountBlock.Amount = "0"
	req.AccountBlock.TokenStandard = types.ZnnTokenStandard.String()
	req.AccountBlock.Data = base64.StdEncoding.EncodeToString(data)
	return req, active
}

func TestWalletConnectBridgeTemplateAcceptsCanonicalRedeem(t *testing.T) {
	req, active := canonicalRedeemRequest(t)
	template, _, effect, err := walletConnectBridgeTemplate(req, active)
	if err != nil {
		t.Fatal(err)
	}
	if effect.Method != definition.RedeemUnwrapMethodName || template.Amount.Sign() != 0 || template.TokenStandard != types.ZnnTokenStandard {
		t.Fatalf("unexpected canonical Redeem template/effect: template=%+v effect=%+v", template, effect)
	}
}

func TestWalletConnectBridgeTemplateFailsClosed(t *testing.T) {
	base, active := validWalletConnectRequest(t)
	other := types.ParseAddressPanic("z1qr4pexnnfaexqqz8nscjjcsajy5hdqfkgadvwx")
	tests := []struct {
		name string
		edit func(*WalletConnectSendRequest)
		want string
	}{
		{"wrong envelope sender", func(r *WalletConnectSendRequest) { r.FromAddress = other.String() }, "active wallet account"},
		{"wrong embedded sender", func(r *WalletConnectSendRequest) { r.AccountBlock.Address = other.String() }, "account-block sender"},
		{"wrong chain", func(r *WalletConnectSendRequest) { r.AccountBlock.ChainIdentifier = 73404 }, "zenon:1"},
		{"wrong block type", func(r *WalletConnectSendRequest) { r.AccountBlock.BlockType = uint64(nom.BlockTypeUserReceive) }, "user-send"},
		{"non bridge destination", func(r *WalletConnectSendRequest) { r.AccountBlock.ToAddress = types.StakeContract.String() }, "only the Zenon Bridge"},
		{"negative amount", func(r *WalletConnectSendRequest) { r.AccountBlock.Amount = "-1" }, "amount"},
		{"zero wrap amount", func(r *WalletConnectSendRequest) { r.AccountBlock.Amount = "0" }, "positive"},
		{"non canonical base64", func(r *WalletConnectSendRequest) { r.AccountBlock.Data = strings.TrimRight(r.AccountBlock.Data, "=") }, "canonical base64"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := base
			tc.edit(&req)
			_, _, _, err := walletConnectBridgeTemplate(req, active)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("got %v, want error containing %q", err, tc.want)
			}
		})
	}
}

func TestWalletConnectBridgeTemplateRejectsFundedOrNonCanonicalRedeem(t *testing.T) {
	base, active := canonicalRedeemRequest(t)
	tests := []struct {
		name string
		edit func(*WalletConnectSendRequest)
		want string
	}{
		{"attached funds", func(r *WalletConnectSendRequest) { r.AccountBlock.Amount = "1" }, "must not attach funds"},
		{"non ZNN standard", func(r *WalletConnectSendRequest) { r.AccountBlock.TokenStandard = types.QsrTokenStandard.String() }, "ZNN token standard"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := base
			tc.edit(&req)
			_, _, _, err := walletConnectBridgeTemplate(req, active)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("got %v, want error containing %q", err, tc.want)
			}
		})
	}
}

func TestWalletConnectBridgeTemplateRejectsPrivilegedBridgeMethod(t *testing.T) {
	req, active := validWalletConnectRequest(t)
	data := definition.ABIBridge.PackMethodPanic(definition.HaltMethodName, "bridge halted")
	req.AccountBlock.Amount = "0"
	req.AccountBlock.Data = base64.StdEncoding.EncodeToString(data)
	_, _, _, err := walletConnectBridgeTemplate(req, active)
	if err == nil || !strings.Contains(err.Error(), "not an approved user bridge operation") {
		t.Fatalf("got %v", err)
	}
}

func TestWalletConnectBlockJSONMatchesTypeScriptTemplateShape(t *testing.T) {
	b := &nom.AccountBlock{
		Version:         1,
		ChainIdentifier: 1,
		BlockType:       nom.BlockTypeUserSend,
		Address:         types.ParseAddressPanic("z1qzal6c5s9rjnnxd2z7dvdhjxpmmj4fmw56a0mz"),
		ToAddress:       types.BridgeContract,
		Amount:          big.NewInt(123456789),
		TokenStandard:   types.ZnnTokenStandard,
		Data:            []byte{0xde, 0xad, 0xbe, 0xef},
	}
	got, err := walletConnectBlockJSON(b)
	if err != nil {
		t.Fatal(err)
	}
	if got["amount"] != "123456789" || got["data"] != "3q2+7w==" || got["toAddress"] != types.BridgeContract.String() {
		t.Fatalf("published JSON does not match AccountBlockTemplate.fromJson input: %#v", got)
	}
	for _, field := range []string{"hash", "previousHash", "height", "momentumAcknowledged", "nonce", "publicKey", "signature"} {
		if _, ok := got[field]; !ok {
			t.Fatalf("published JSON missing %q", field)
		}
	}
}

func TestPrepareWalletConnectSendFailsClosedAtEveryMainnetGate(t *testing.T) {
	req, _ := validWalletConnectRequest(t)

	t.Run("locked wallet", func(t *testing.T) {
		tx := newTestTxService(t)
		if _, err := tx.PrepareWalletConnectSend(req); err == nil || !strings.Contains(err.Error(), "locked") {
			t.Fatalf("got %v, want locked-wallet refusal", err)
		}
	})

	t.Run("configured chain is not mainnet", func(t *testing.T) {
		tx := newTestTxService(t)
		unlockTestWallet(t, tx.wallet)
		tx.node.chainID = mainnetChainID
		if err := tx.config.SetChainID(73404); err != nil {
			t.Fatal(err)
		}
		if _, err := tx.PrepareWalletConnectSend(req); err == nil || !strings.Contains(err.Error(), "Chain ID 1") {
			t.Fatalf("got %v, want configured-chain refusal", err)
		}
	})

	t.Run("connected node is not mainnet", func(t *testing.T) {
		tx := newTestTxService(t)
		unlockTestWallet(t, tx.wallet)
		tx.node.chainID = 73404
		if _, err := tx.PrepareWalletConnectSend(req); err == nil || !strings.Contains(err.Error(), "mainnet node") {
			t.Fatalf("got %v, want node-chain refusal", err)
		}
	})

	t.Run("mainnet transactions are not enabled", func(t *testing.T) {
		tx := newTestTxService(t)
		unlockTestWallet(t, tx.wallet)
		tx.node.chainID = mainnetChainID
		active, _ := tx.wallet.activeAddress()
		activeReq := req
		activeReq.FromAddress = active.String()
		if _, err := tx.PrepareWalletConnectSend(activeReq); err == nil || !strings.Contains(err.Error(), "mainnet sending is disabled") {
			t.Fatalf("got %v, want explicit-opt-in refusal", err)
		}
	})

	t.Run("enabled but disconnected", func(t *testing.T) {
		tx := newTestTxService(t)
		unlockTestWallet(t, tx.wallet)
		tx.node.chainID = mainnetChainID
		if err := tx.config.SetAllowMainnetSend(true); err != nil {
			t.Fatal(err)
		}
		active, _ := tx.wallet.activeAddress()
		activeReq := req
		activeReq.FromAddress = active.String()
		if _, err := tx.PrepareWalletConnectSend(activeReq); err == nil || !strings.Contains(err.Error(), "not connected") {
			t.Fatalf("got %v, want disconnected-node refusal after all gates pass", err)
		}
	})
}

func TestPrepareWalletConnectSendRequiresResolvableCustomTokenDecimals(t *testing.T) {
	tx := newTestTxService(t)
	unlockTestWallet(t, tx.wallet)
	tx.node.chainID = mainnetChainID
	if err := tx.config.SetAllowMainnetSend(true); err != nil {
		t.Fatal(err)
	}
	active, _ := tx.wallet.activeAddress()
	req, _ := validWalletConnectRequest(t)
	req.FromAddress = active.String()
	req.AccountBlock.TokenStandard = "zts1x27drtpzgj99rjxcm7xmmg"
	tx.decimalsLookup = func(types.ZenonTokenStandard) (int, error) {
		return 0, errors.New("token metadata unavailable")
	}
	if _, err := tx.PrepareWalletConnectSend(req); err == nil || !strings.Contains(err.Error(), "decimals") {
		t.Fatalf("got %v, want a decimals-resolution refusal", err)
	}
}

func TestPrepareWalletConnectSendSkipsDecimalsLookupForZnn(t *testing.T) {
	tx := newTestTxService(t)
	unlockTestWallet(t, tx.wallet)
	tx.node.chainID = mainnetChainID
	if err := tx.config.SetAllowMainnetSend(true); err != nil {
		t.Fatal(err)
	}
	active, _ := tx.wallet.activeAddress()
	req, _ := validWalletConnectRequest(t)
	req.FromAddress = active.String()
	tx.decimalsLookup = func(types.ZenonTokenStandard) (int, error) {
		t.Fatal("ZNN must resolve its protocol-fixed decimals without a lookup")
		return 0, nil
	}
	// ZNN passes the decimals gate without a node call and fails later at the
	// connectivity check, proving gate ordering.
	if _, err := tx.PrepareWalletConnectSend(req); err == nil || !strings.Contains(err.Error(), "not connected") {
		t.Fatalf("got %v, want not-connected after passing the decimals gate", err)
	}
}
