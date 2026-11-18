package app

import (
	"math/big"
	"strings"
	"testing"

	embedded "github.com/0x3639/znn-sdk-go/api/embedded"
	"github.com/zenon-network/go-zenon/chain/nom"
	"github.com/zenon-network/go-zenon/common/types"
)

const (
	effAddr1 = "z1qzal6c5s9rjnnxd2z7dvdhjxpmmj4fmw56a0mz"
	effAddr2 = "z1qr4pexnnfaexqqz8nscjjcsajy5hdqfkgadvwx"
	effHash  = "00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff"
	// A value far beyond uint64 — exercises big.Int rendering.
	effBigAmount = "123456789012345678901234567890"
)

// Keep contract decoding coverage independent of any proposal UI: these SDK
// templates also reach confirmation through WalletConnect.
func TestDecodeContractTemplates(t *testing.T) {
	bridge := embedded.NewBridgeApi(nil)
	liquidity := embedded.NewLiquidityApi(nil)
	spork := embedded.NewSporkApi(nil)
	a := types.ParseAddressPanic(effAddr1)
	b := types.ParseAddressPanic(effAddr2)
	h := types.HexToHashPanic(effHash)
	znn, qsr := types.ZnnTokenStandard, types.QsrTokenStandard
	tests := []struct {
		name  string
		block *nom.AccountBlock
		want  []string
	}{
		{"spork.create", spork.CreateSpork("test-spork", "gates the new hash algo"), []string{"test-spork", "gates the new hash algo"}},
		{"spork.activate", spork.ActivateSpork(h), []string{effHash}},
		{"bridge.addNetwork", bridge.AddNetwork(2, 31337, "Ethereum", "0xabcdef0123", `{"rpc":"wss://x"}`), []string{"2", "31337", "Ethereum", "0xabcdef0123", `{"rpc":"wss://x"}`}},
		{"bridge.removeNetwork", bridge.RemoveNetwork(2, 31337), []string{"2", "31337"}},
		{"bridge.setTokenPair", bridge.SetTokenPair(1, 2, znn, "0xdeadbeef", true, false, true, mustBig(t, effBigAmount), 15, 40, "{}"), []string{znn.String(), "0xdeadbeef", "true", "false", effBigAmount, "15", "40", "{}"}},
		{"bridge.removeTokenPair", bridge.RemoveTokenPair(1, 2, qsr, "0xdeadbeef"), []string{qsr.String(), "0xdeadbeef"}},
		{"bridge.halt", bridge.Halt("halt-signature-data"), []string{"halt-signature-data"}},
		{"bridge.unhalt", bridge.Unhalt(), nil},
		{"bridge.emergency", bridge.Emergency(), nil},
		{"bridge.changeAdministrator", bridge.ChangeAdministrator(a), []string{effAddr1}},
		{"bridge.changeTssECDSAPubKey", bridge.ChangeTssECDSAPubKey("pubkey-b64", "old-sig", "new-sig"), []string{"pubkey-b64", "old-sig", "new-sig"}},
		{"bridge.setAllowKeygen", bridge.SetAllowKeygen(true), []string{"true"}},
		{"bridge.setOrchestratorInfo", bridge.SetOrchestratorInfo(6, 66, 20, 10), []string{"6", "66", "20", "10"}},
		{"bridge.setMetadata", bridge.SetBridgeMetadata(`{"affiliate":true}`), []string{`{"affiliate":true}`}},
		{"bridge.setNetworkMetadata", bridge.SetNetworkMetadata(1, 2, `{"n":1}`), []string{`{"n":1}`}},
		{"bridge.revokeUnwrapRequest", bridge.RevokeUnwrapRequest(h, 3), []string{effHash, "3"}},
		{"bridge.nominateGuardians", bridge.NominateGuardians([]types.Address{a, b}), []string{effAddr1 + ", " + effAddr2}},
		{"liquidity.emergency", liquidity.Emergency(), nil},
		{"liquidity.setTokenTuple", liquidity.SetTokenTupleMethod([]string{znn.String(), qsr.String()}, []uint32{7000, 3000}, []uint32{4000, 6000}, []*big.Int{big.NewInt(100), mustBig(t, effBigAmount)}), []string{znn.String() + ", " + qsr.String(), "7000, 3000", "4000, 6000", "100, " + effBigAmount}},
		{"liquidity.setIsHalted", liquidity.SetIsHalted(true), []string{"true"}},
		{"liquidity.setAdditionalReward", liquidity.SetAdditionalReward(big.NewInt(100000000), big.NewInt(1000000000)), []string{"100000000", "1000000000"}},
		{"liquidity.changeAdministrator", liquidity.ChangeAdministrator(b), []string{effAddr2}},
		{"liquidity.nominateGuardians", liquidity.NominateGuardians([]types.Address{a, b}), []string{effAddr1 + ", " + effAddr2}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			effect, err := decodeContractCall(tt.block.ToAddress, tt.block.Data)
			if err != nil {
				t.Fatal(err)
			}
			if effect.Contract == "" || effect.Method == "" {
				t.Fatalf("unnamed effect: %+v", effect)
			}
			assertEffectHasValues(t, effect, tt.want...)
		})
	}
}

func TestDecodeContractCallFailsClosed(t *testing.T) {
	good := embedded.NewBridgeApi(nil).ChangeAdministrator(types.ParseAddressPanic(effAddr1))
	data := good.Data

	// Unknown destination (a regular user address) must refuse.
	user, _ := types.ParseAddress(effAddr1)
	if _, err := decodeContractCall(user, data); err == nil {
		t.Fatal("unknown destination must fail closed")
	}
	// A method selector from a DIFFERENT contract must refuse.
	if _, err := decodeContractCall(types.TokenContract, data); err == nil {
		t.Fatal("destination/method mismatch must fail closed")
	}
	// Truncated data must refuse.
	if _, err := decodeContractCall(good.ToAddress, data[:2]); err == nil {
		t.Fatal("truncated selector must fail closed")
	}
	if _, err := decodeContractCall(good.ToAddress, data[:len(data)-1]); err == nil {
		t.Fatal("truncated arguments must fail closed")
	}
	// Trailing garbage must refuse (round-trip check) — the fields would not
	// fully describe what executes.
	if _, err := decodeContractCall(good.ToAddress, append(append([]byte(nil), data...), 0xde, 0xad)); err == nil {
		t.Fatal("trailing bytes must fail closed")
	}
	// Unknown selector on a known contract must refuse.
	bogus := append([]byte{0xde, 0xad, 0xbe, 0xef}, data[4:]...)
	if _, err := decodeContractCall(good.ToAddress, bogus); err == nil {
		t.Fatal("unknown method must fail closed")
	}
}

// The staking confirmation identifies the exact empty-argument ABI call before
// the frontend explains its protocol outcome (QSR-only rewards). This keeps the
// friendly explanation anchored to the held Stake.CollectReward payload.
func TestDecodeStakeCollectReward(t *testing.T) {
	template := embedded.NewStakeApi(nil).CollectReward()
	effect, err := decodeContractCall(template.ToAddress, template.Data)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if effect.Contract != "Stake" || effect.Method != "CollectReward" {
		t.Fatalf("effect = %s.%s, want Stake.CollectReward", effect.Contract, effect.Method)
	}
	if len(effect.Fields) != 0 {
		t.Fatalf("CollectReward has no ABI arguments, got fields %+v", effect.Fields)
	}
}

// TestDecodeAcceleratorTemplates decodes the exact templates the accelerator
// prepare paths hold, proving name, full description, URL, and base-unit
// amounts all surface (PR-06). Unicode + long strings included.
func TestDecodeAcceleratorTemplates(t *testing.T) {
	accel := embedded.NewAcceleratorApi(nil)
	longDesc := strings.Repeat("véry lông déscription — ", 10) + "🚀 end"
	url := "https://forum.zenon.org/some/very/long/path?with=query&and=more"
	znn := "500000000000"
	qsr := effBigAmount

	t.Run("CreateProject", func(t *testing.T) {
		tmpl := accel.CreateProject("Test Prøject", longDesc, url, mustBig(t, znn), mustBig(t, qsr))
		effect, err := decodeContractCall(types.AcceleratorContract, tmpl.Data)
		if err != nil {
			t.Fatalf("decode: %v", err)
		}
		assertEffectHasValues(t, effect, "Test Prøject", longDesc, url, znn, qsr)
	})
	t.Run("AddPhase", func(t *testing.T) {
		h, _ := types.HexToHash(effHash)
		tmpl := accel.AddPhase(h, "Phase Ⅰ", longDesc, url, mustBig(t, znn), mustBig(t, qsr))
		effect, err := decodeContractCall(types.AcceleratorContract, tmpl.Data)
		if err != nil {
			t.Fatalf("decode: %v", err)
		}
		assertEffectHasValues(t, effect, effHash, "Phase Ⅰ", longDesc, url, znn, qsr)
	})
	t.Run("UpdatePhase", func(t *testing.T) {
		h, _ := types.HexToHash(effHash)
		tmpl := accel.UpdatePhase(h, "Phase Ⅱ", longDesc, url, mustBig(t, znn), mustBig(t, qsr))
		effect, err := decodeContractCall(types.AcceleratorContract, tmpl.Data)
		if err != nil {
			t.Fatalf("decode: %v", err)
		}
		assertEffectHasValues(t, effect, effHash, "Phase Ⅱ", longDesc, url, znn, qsr)
	})
}

// TestDecodeNomWriteTemplates decodes the exact templates the first-party NoM
// prepare paths hold, proving the MATERIAL ABI parameters (mint receiver,
// token owner, pillar producer/reward addresses) surface from the signed
// bytes, not just a form-derived summary (audit GS-04).
func TestDecodeNomWriteTemplates(t *testing.T) {
	addrA := effAddr1 // any valid z1 fixture already used in this package
	a, err := types.ParseAddress(addrA)
	if err != nil {
		t.Fatal(err)
	}
	zts := types.ZnnTokenStandard

	t.Run("Plasma.Fuse", func(t *testing.T) {
		tmpl := embedded.NewPlasmaApi(nil).Fuse(a, mustBig(t, "5000000000"))
		effect, err := decodeContractCall(tmpl.ToAddress, tmpl.Data)
		if err != nil {
			t.Fatalf("decode: %v", err)
		}
		assertEffectHasValues(t, effect, addrA)
	})
	t.Run("Pillar.Register", func(t *testing.T) {
		tmpl := embedded.NewPillarApi(nil).Register("MyPillar", a, a, 10, 90)
		effect, err := decodeContractCall(tmpl.ToAddress, tmpl.Data)
		if err != nil {
			t.Fatalf("decode: %v", err)
		}
		assertEffectHasValues(t, effect, "MyPillar", addrA)
	})
	t.Run("Pillar.UpdatePillar", func(t *testing.T) {
		tmpl := embedded.NewPillarApi(nil).UpdatePillar("MyPillar", a, a, 10, 90)
		effect, err := decodeContractCall(tmpl.ToAddress, tmpl.Data)
		if err != nil {
			t.Fatalf("decode: %v", err)
		}
		assertEffectHasValues(t, effect, "MyPillar", addrA)
	})
	t.Run("Token.IssueToken", func(t *testing.T) {
		tmpl := embedded.NewTokenApi(nil).IssueToken("My Token", "MYT", "example.com", mustBig(t, "1000"), mustBig(t, "2000"), 8, true, true, false)
		effect, err := decodeContractCall(tmpl.ToAddress, tmpl.Data)
		if err != nil {
			t.Fatalf("decode: %v", err)
		}
		assertEffectHasValues(t, effect, "My Token", "MYT", "1000", "2000")
	})
	t.Run("Token.Mint", func(t *testing.T) {
		tmpl := embedded.NewTokenApi(nil).Mint(zts, mustBig(t, "777"), a)
		effect, err := decodeContractCall(tmpl.ToAddress, tmpl.Data)
		if err != nil {
			t.Fatalf("decode: %v", err)
		}
		assertEffectHasValues(t, effect, "777", addrA)
	})
	t.Run("Token.UpdateToken", func(t *testing.T) {
		tmpl := embedded.NewTokenApi(nil).UpdateToken(zts, a, true, false)
		effect, err := decodeContractCall(tmpl.ToAddress, tmpl.Data)
		if err != nil {
			t.Fatalf("decode: %v", err)
		}
		assertEffectHasValues(t, effect, addrA)
	})
}

func assertEffectHasValues(t *testing.T, effect *TransactionEffect, want ...string) {
	t.Helper()
	rendered := make([]string, 0, len(effect.Fields))
	for _, f := range effect.Fields {
		rendered = append(rendered, f.Value)
	}
	for _, w := range want {
		found := false
		for _, v := range rendered {
			if v == w {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected value %q missing from decoded fields %v", w, rendered)
		}
	}
}

func mustBig(t *testing.T, s string) *big.Int {
	t.Helper()
	n, ok := new(big.Int).SetString(s, 10)
	if !ok {
		t.Fatalf("bad big int %q", s)
	}
	return n
}
