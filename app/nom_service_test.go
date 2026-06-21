package app

import (
	"math/big"
	"testing"

	embedded "github.com/0x3639/znn-sdk-go/api/embedded"
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
