package app

import (
	"math/big"
	"testing"

	"github.com/zenon-network/go-zenon/chain/nom"
	"github.com/zenon-network/go-zenon/common/types"
	api "github.com/zenon-network/go-zenon/rpc/api"
)

func TestToTokenBalance(t *testing.T) {
	bi := &api.BalanceInfo{
		TokenInfo: &api.Token{ZenonTokenStandard: types.ZnnTokenStandard, TokenSymbol: "ZNN", Decimals: 8},
		Balance:   big.NewInt(5000000000000),
	}
	got := toTokenBalance(types.ZnnTokenStandard, bi)
	if got.Symbol != "ZNN" || got.Decimals != 8 || got.Amount != "5000000000000" {
		t.Fatalf("toTokenBalance = %+v", got)
	}
	if got.Zts != types.ZnnTokenStandard.String() {
		t.Fatalf("zts = %s", got.Zts)
	}
}

func TestToTxRecordDirection(t *testing.T) {
	send := &api.AccountBlock{}
	send.AccountBlock = nom.AccountBlock{
		Hash:          types.HexToHashPanic("0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20"),
		BlockType:     nom.BlockTypeUserSend,
		ToAddress:     types.ParseAddressPanic("z1qqjnwjjpnue8xmmpanz6csze6tcmtzzdtfsww7"),
		Amount:        big.NewInt(100000000),
		TokenStandard: types.ZnnTokenStandard,
	}
	rec := toTxRecord(send)
	if rec.Direction != "send" {
		t.Fatalf("direction = %s, want send", rec.Direction)
	}
	if rec.Amount != "100000000" || rec.Confirmed {
		t.Fatalf("rec = %+v", rec)
	}
}

func TestStatusDefaults(t *testing.T) {
	n := newNodeService(newConfigService(), newWalletService(newConfigService()))
	s := n.NodeStatus()
	if s.Connected || s.Mode != "remote" {
		t.Fatalf("status = %+v", s)
	}
}
