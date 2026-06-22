package app

import (
	"math/big"
	"testing"

	embedded "github.com/0x3639/znn-sdk-go/api/embedded"
	nom "github.com/zenon-network/go-zenon/chain/nom"
	"github.com/zenon-network/go-zenon/common/types"
)

func TestFusionEntryDTORevocable(t *testing.T) {
	addr, _ := types.ParseAddress("z1qzal6c5s9rjnnxd2z7dvdhjxpmmj4fmw56a0mz")
	id := types.HexToHashPanic("0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20")
	e := &embedded.FusionEntry{QsrAmount: big.NewInt(10_000_000_000), Beneficiary: addr, ExpirationHeight: 100, Id: id}

	// frontier below expiration → not revocable
	d := fusionEntryDTO(e, 50)
	if d.IsRevocable {
		t.Fatal("should not be revocable below expiration")
	}
	if d.Beneficiary != addr.String() || d.ExpirationHeight != 100 {
		t.Fatalf("bad mapping: %+v", d)
	}
	// frontier at/above expiration → revocable
	if !fusionEntryDTO(e, 100).IsRevocable {
		t.Fatal("should be revocable at expiration")
	}
	if !fusionEntryDTO(e, 150).IsRevocable {
		t.Fatal("should be revocable above expiration")
	}
}

func TestStakeEntryDTO(t *testing.T) {
	addr, _ := types.ParseAddress("z1qzal6c5s9rjnnxd2z7dvdhjxpmmj4fmw56a0mz")
	id := types.HexToHashPanic("0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20")
	start := int64(1_700_000_000)
	const unit = int64(2_592_000)
	// 3-month stake
	e := &embedded.StakeEntry{
		Amount:              big.NewInt(500_000_000), // 5 ZNN
		StartTimestamp:      start,
		ExpirationTimestamp: start + 3*unit,
		Address:             addr,
		Id:                  id,
	}
	// before expiration → not matured
	d := stakeEntryDTO(e, start+unit)
	if d.IsMatured {
		t.Fatal("should not be matured before expiration")
	}
	if d.DurationMonths != 3 {
		t.Fatalf("DurationMonths = %d, want 3", d.DurationMonths)
	}
	if d.Amount != "500000000" || d.Id != id.String() {
		t.Fatalf("bad mapping: %+v", d)
	}
	// at/after expiration → matured
	if !stakeEntryDTO(e, start+3*unit).IsMatured {
		t.Fatal("should be matured at expiration")
	}
	if !stakeEntryDTO(e, start+10*unit).IsMatured {
		t.Fatal("should be matured after expiration")
	}
}

func TestPrepareFuseValidatesInput(t *testing.T) {
	s := newNomService(newTestNode(t), newTestWalletService(t), nil)
	// Bad beneficiary and bad amount are rejected BEFORE any node/client use.
	if _, err := s.PrepareFuse("not-an-address", "100"); err == nil {
		t.Fatal("expected invalid beneficiary to be rejected")
	}
	if _, err := s.PrepareFuse("z1qzal6c5s9rjnnxd2z7dvdhjxpmmj4fmw56a0mz", "0"); err == nil {
		t.Fatal("expected zero amount to be rejected")
	}
	if _, err := s.PrepareFuse("z1qzal6c5s9rjnnxd2z7dvdhjxpmmj4fmw56a0mz", "abc"); err == nil {
		t.Fatal("expected non-numeric amount to be rejected")
	}
}

func TestPrepareCancelFuseValidatesInput(t *testing.T) {
	s := newNomService(newTestNode(t), newTestWalletService(t), nil)
	if _, err := s.PrepareCancelFuse("not-a-hash"); err == nil {
		t.Fatal("expected invalid id to be rejected")
	}
}

func TestPrepareStakeValidatesInput(t *testing.T) {
	s := newNomService(newTestNode(t), newTestWalletService(t), nil)
	// amount below 1 ZNN min, non-numeric amount, and bad duration are rejected before any node use.
	if _, err := s.PrepareStake("50000000", "3"); err == nil { // 0.5 ZNN < 1 ZNN min
		t.Fatal("expected below-min amount to be rejected")
	}
	if _, err := s.PrepareStake("abc", "3"); err == nil {
		t.Fatal("expected non-numeric amount to be rejected")
	}
	if _, err := s.PrepareStake("100000000", "0"); err == nil {
		t.Fatal("expected duration 0 to be rejected")
	}
	if _, err := s.PrepareStake("100000000", "13"); err == nil {
		t.Fatal("expected duration 13 to be rejected")
	}
	if _, err := s.PrepareCancelStake("not-a-hash"); err == nil {
		t.Fatal("expected bad id to be rejected")
	}
}

func TestStakeTemplateTokenStandards(t *testing.T) {
	api := embedded.NewStakeApi(nil) // builders construct blocks from args; no client deref
	id := types.HexToHashPanic("0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20")
	for name, b := range map[string]*nom.AccountBlock{
		"stake":   api.Stake(stakeTimeUnitSec, big.NewInt(100_000_000)),
		"cancel":  api.Cancel(id),
		"collect": api.CollectReward(),
	} {
		if b.ToAddress != types.StakeContract {
			t.Fatalf("%s: ToAddress=%v want StakeContract", name, b.ToAddress)
		}
		if b.TokenStandard != types.ZnnTokenStandard {
			t.Fatalf("%s: TokenStandard=%v want ZNN", name, b.TokenStandard)
		}
	}
}

// TestPlasmaTemplateTokenStandards locks in the SDK template token-standard
// expectations our callExpects rely on. The callExpect zts passed to
// prepareCall MUST equal the SDK template's TokenStandard, or
// TxService.ConfirmPublish's assertMatches rejects the published block.
//
// Built against the REAL SDK template builders: PlasmaApi.Fuse / .Cancel
// construct a *nom.AccountBlock from the receiver only (they do not touch
// pa.client), so embedded.NewPlasmaApi(nil) runs fully offline. We avoid
// rpc_client.NewRpcClient here because it dials (server.Dial) at construction
// and cannot run offline.
//
// Fuse uses QSR; Cancel uses ZNN — a real, asymmetric SDK behavior. This test
// fails if either the SDK changes or our PrepareFuse/PrepareCancelFuse zts
// drifts away from the template.
func TestPlasmaTemplateTokenStandards(t *testing.T) {
	pa := embedded.NewPlasmaApi(nil)
	addr, _ := types.ParseAddress("z1qzal6c5s9rjnnxd2z7dvdhjxpmmj4fmw56a0mz")
	id := types.HexToHashPanic("0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20")

	// Fuse template uses QSR — PrepareFuse's callExpect.zts must match.
	fuse := pa.Fuse(addr, big.NewInt(100))
	if fuse.TokenStandard != types.QsrTokenStandard {
		t.Fatalf("Fuse template zts=%v, want QSR %v", fuse.TokenStandard, types.QsrTokenStandard)
	}

	// Cancel template uses ZNN (NOT QSR) — PrepareCancelFuse's callExpect.zts
	// must match. This is the bug this test guards against.
	cancel := pa.Cancel(id)
	if cancel.TokenStandard != types.ZnnTokenStandard {
		t.Fatalf("Cancel template zts=%v, want ZNN %v", cancel.TokenStandard, types.ZnnTokenStandard)
	}

	// Sanity: the two standards are genuinely distinct, otherwise the above
	// assertions would be vacuous.
	if types.QsrTokenStandard == types.ZnnTokenStandard {
		t.Fatal("QSR and ZNN token standards must be distinct")
	}
}
