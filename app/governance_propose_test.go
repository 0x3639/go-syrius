package app

import (
	"math/big"
	"strings"
	"testing"

	"github.com/0x3639/go-syrius/internal/governance"
	"github.com/zenon-network/go-zenon/common/types"
)

func TestGetProposeKinds_HasSporkNotCustom(t *testing.T) {
	enableGovernance(t)
	s := newNomService(newTestNode(t), newTestWalletService(t), nil)
	kinds, err := s.GetProposeKinds()
	if err != nil {
		t.Fatalf("GetProposeKinds err: %v", err)
	}
	byId := map[string]ProposeKindDTO{}
	for _, k := range kinds {
		byId[k.Kind] = k
	}
	for _, want := range []string{"spork.create", "spork.activate"} {
		if _, ok := byId[want]; !ok {
			t.Fatalf("missing kind %q", want)
		}
	}
	if byId["spork.create"].Group != "Spork" || len(byId["spork.create"].Fields) != 2 {
		t.Fatalf("spork.create schema wrong: %+v", byId["spork.create"])
	}
	// custom was dropped for the testnet release.
	if _, ok := byId["custom"]; ok {
		t.Fatal("custom kind must not be present (dropped for release)")
	}
}

func TestBuildProposalPayload_SporkCreate(t *testing.T) {
	api := governance.NewAPI(nil)
	// Build directly via the compatibility helper to confirm our dispatcher mirrors it.
	want := api.PayloadSporkCreate("MySpork", "desc")
	got, err := buildProposalPayloadWith(api, "spork.create", map[string]string{"name": "MySpork", "description": "desc"})
	if err != nil {
		t.Fatalf("build err: %v", err)
	}
	if got.Destination != want.Destination || got.Data != want.Data {
		t.Fatalf("spork.create payload mismatch: got %+v want %+v", got, want)
	}
}

func TestBuildProposalPayload_CustomRemoved(t *testing.T) {
	api := governance.NewAPI(nil)
	// custom was dropped for the testnet release → now an unknown kind.
	if _, err := buildProposalPayloadWith(api, "custom", map[string]string{"destination": types.SporkContract.String(), "data": "AAEC"}); err == nil {
		t.Fatal("custom kind must be rejected (dropped for release)")
	}
}

func TestBuildProposalPayload_UnknownKind(t *testing.T) {
	api := governance.NewAPI(nil)
	if _, err := buildProposalPayloadWith(api, "bogus.kind", map[string]string{}); err == nil {
		t.Fatal("unknown kind must error")
	}
}

func TestPrepareProposeAction_Validation(t *testing.T) {
	enableGovernance(t)
	s := newNomService(newTestNode(t), newTestWalletService(t), nil)
	good := map[string]string{"name": "S", "description": "d"}
	if _, err := s.PrepareProposeAction("", "d", "https://zenon.org", "spork.create", good); err == nil {
		t.Fatal("empty action name must error")
	}
	if _, err := s.PrepareProposeAction("Act", "d", "bad-url", "spork.create", good); err == nil {
		t.Fatal("bad url must error")
	}
	if _, err := s.PrepareProposeAction("Act", "d", "https://zenon.org", "spork.create", good); err == nil || err.Error() != "not connected" {
		t.Fatalf("valid propose should hit not-connected; got %v", err)
	}
}

func TestBuildProposalPayload_BridgeKinds(t *testing.T) {
	api := governance.NewAPI(nil)
	cases := []struct {
		kind   string
		params map[string]string
		want   governance.ProposalPayload
	}{
		{"bridge.addNetwork", map[string]string{"networkClass": "1", "chainId": "2", "name": "eth", "contractAddress": "0xabc", "metadata": "{}"}, api.PayloadBridgeAddNetwork(1, 2, "eth", "0xabc", "{}")},
		{"bridge.removeNetwork", map[string]string{"networkClass": "1", "chainId": "2"}, api.PayloadBridgeRemoveNetwork(1, 2)},
		{"bridge.unhalt", map[string]string{}, api.PayloadBridgeUnhalt()},
		{"bridge.emergency", map[string]string{}, api.PayloadBridgeEmergency()},
		{"bridge.halt", map[string]string{"signature": "sig"}, api.PayloadBridgeHalt("sig")},
		{"bridge.setAllowKeygen", map[string]string{"allowKeygen": "true"}, api.PayloadBridgeSetAllowKeygen(true)},
		{"bridge.changeAdministrator", map[string]string{"administrator": types.SporkContract.String()}, api.PayloadBridgeChangeAdministrator(types.SporkContract)},
		{"bridge.setOrchestratorInfo", map[string]string{"windowSize": "10", "keyGenThreshold": "2", "confirmationsToFinality": "3", "estimatedMomentumTime": "10"}, api.PayloadBridgeSetOrchestratorInfo(10, 2, 3, 10)},
		{"bridge.setMetadata", map[string]string{"metadata": "{}"}, api.PayloadBridgeSetMetadata("{}")},
		{"bridge.setNetworkMetadata", map[string]string{"networkClass": "1", "chainId": "2", "metadata": "{}"}, api.PayloadBridgeSetNetworkMetadata(1, 2, "{}")},
		{"bridge.revokeUnwrapRequest", map[string]string{"transactionHash": "0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20", "logIndex": "0"}, api.PayloadBridgeRevokeUnwrapRequest(types.HexToHashPanic("0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20"), 0)},
		{"bridge.nominateGuardians", map[string]string{"guardians": types.SporkContract.String() + "," + types.PillarContract.String()}, api.PayloadBridgeNominateGuardians([]types.Address{types.SporkContract, types.PillarContract})},
		{"bridge.changeTssECDSAPubKey", map[string]string{"pubKey": "pk", "signature": "s", "newSignature": "ns"}, api.PayloadBridgeChangeTssECDSAPubKey("pk", "s", "ns")},
		{"bridge.removeTokenPair", map[string]string{"networkClass": "1", "chainId": "2", "tokenStandard": "zts1znnxxxxxxxxxxxxx9z4ulx", "tokenAddress": "0xabc"}, api.PayloadBridgeRemoveTokenPair(1, 2, types.ParseZTSPanic("zts1znnxxxxxxxxxxxxx9z4ulx"), "0xabc")},
		{"bridge.setTokenPair", map[string]string{"networkClass": "1", "chainId": "2", "tokenStandard": "zts1znnxxxxxxxxxxxxx9z4ulx", "tokenAddress": "0xabc", "bridgeable": "true", "redeemable": "true", "owned": "false", "minAmount": "100", "fee": "5", "redeemDelay": "10", "metadata": "{}"}, api.PayloadBridgeSetTokenPair(1, 2, types.ParseZTSPanic("zts1znnxxxxxxxxxxxxx9z4ulx"), "0xabc", true, true, false, big.NewInt(100), 5, 10, "{}")},
	}
	for _, c := range cases {
		got, err := buildProposalPayloadWith(api, c.kind, c.params)
		if err != nil {
			t.Fatalf("%s: unexpected err %v", c.kind, err)
		}
		if got.Destination != c.want.Destination || got.Data != c.want.Data {
			t.Fatalf("%s: payload mismatch got %+v want %+v", c.kind, got, c.want)
		}
	}
	// a representative bad-params case
	if _, err := buildProposalPayloadWith(api, "bridge.addNetwork", map[string]string{"networkClass": "x", "chainId": "2", "name": "e", "contractAddress": "c", "metadata": "m"}); err == nil {
		t.Fatal("non-numeric networkClass must error")
	}
}

func TestProposeKinds_IncludesAllBridge(t *testing.T) {
	have := map[string]bool{}
	for _, k := range proposeKinds() {
		have[k.Kind] = true
	}
	for _, want := range []string{"bridge.addNetwork", "bridge.removeNetwork", "bridge.setTokenPair", "bridge.removeTokenPair", "bridge.halt", "bridge.unhalt", "bridge.emergency", "bridge.changeAdministrator", "bridge.changeTssECDSAPubKey", "bridge.setAllowKeygen", "bridge.setOrchestratorInfo", "bridge.setMetadata", "bridge.setNetworkMetadata", "bridge.revokeUnwrapRequest", "bridge.nominateGuardians"} {
		if !have[want] {
			t.Fatalf("missing bridge kind %q", want)
		}
	}
}

func TestBuildProposalPayload_LiquidityKinds(t *testing.T) {
	api := governance.NewAPI(nil)
	cases := []struct {
		kind   string
		params map[string]string
		want   governance.ProposalPayload
	}{
		{"liquidity.fund", map[string]string{"znnReward": "10", "qsrReward": "20"}, api.PayloadLiquidityFund(big.NewInt(10), big.NewInt(20))},
		{"liquidity.burnZnn", map[string]string{"burnAmount": "5"}, api.PayloadLiquidityBurnZnn(big.NewInt(5))},
		{"liquidity.setIsHalted", map[string]string{"value": "true"}, api.PayloadLiquiditySetIsHalted(true)},
		{"liquidity.setAdditionalReward", map[string]string{"znnReward": "1", "qsrAmount": "2"}, api.PayloadLiquiditySetAdditionalReward(big.NewInt(1), big.NewInt(2))},
		{"liquidity.changeAdministrator", map[string]string{"administrator": types.SporkContract.String()}, api.PayloadLiquidityChangeAdministrator(types.SporkContract)},
		{"liquidity.nominateGuardians", map[string]string{"guardians": types.SporkContract.String() + "," + types.PillarContract.String()}, api.PayloadLiquidityNominateGuardians([]types.Address{types.SporkContract, types.PillarContract})},
		{"liquidity.emergency", map[string]string{}, api.PayloadLiquidityEmergency()},
		{"liquidity.setTokenTuple", map[string]string{"tokenStandards": "zts1znnxxxxxxxxxxxxx9z4ulx", "znnPercentages": "5000", "qsrPercentages": "5000", "minAmounts": "100"}, api.PayloadLiquiditySetTokenTuple([]string{"zts1znnxxxxxxxxxxxxx9z4ulx"}, []uint32{5000}, []uint32{5000}, []*big.Int{big.NewInt(100)})},
	}
	for _, c := range cases {
		got, err := buildProposalPayloadWith(api, c.kind, c.params)
		if err != nil {
			t.Fatalf("%s: unexpected err %v", c.kind, err)
		}
		if got.Destination != c.want.Destination || got.Data != c.want.Data {
			t.Fatalf("%s: payload mismatch got %+v want %+v", c.kind, got, c.want)
		}
	}
	if _, err := buildProposalPayloadWith(api, "liquidity.fund", map[string]string{"znnReward": "-1", "qsrReward": "2"}); err == nil {
		t.Fatal("negative znnReward must error")
	}
}

func TestBuildProposalPayload_SetTokenTuple_LengthMismatch(t *testing.T) {
	api := governance.NewAPI(nil)
	// 2 token standards but 1 znnPercentage → must error before the SDK call
	_, err := buildProposalPayloadWith(api, "liquidity.setTokenTuple", map[string]string{
		"tokenStandards": "zts1znnxxxxxxxxxxxxx9z4ulx,zts1qsrxxxxxxxxxxxxxjv8v62",
		"znnPercentages": "5000",
		"qsrPercentages": "5000,5000",
		"minAmounts":     "100,100",
	})
	if err == nil {
		t.Fatal("mismatched setTokenTuple list lengths must error")
	}
}

func TestProposeKinds_IncludesAllLiquidity(t *testing.T) {
	have := map[string]bool{}
	for _, k := range proposeKinds() {
		have[k.Kind] = true
	}
	for _, want := range []string{"liquidity.fund", "liquidity.burnZnn", "liquidity.setTokenTuple", "liquidity.setIsHalted", "liquidity.setAdditionalReward", "liquidity.changeAdministrator", "liquidity.nominateGuardians", "liquidity.emergency"} {
		if !have[want] {
			t.Fatalf("missing liquidity kind %q", want)
		}
	}
}

func TestBuildProposalPayload_SporkLengthBounds(t *testing.T) {
	api := governance.NewAPI(nil)
	// name shorter than 5 → error
	if _, err := buildProposalPayloadWith(api, "spork.create", map[string]string{"name": "abc", "description": "ok"}); err == nil {
		t.Fatal("spork name < 5 chars must error")
	}
	// name longer than 40 → error
	long := "thisisaverylongsporknamethatexceedsfortychars!"
	if _, err := buildProposalPayloadWith(api, "spork.create", map[string]string{"name": long, "description": "ok"}); err == nil {
		t.Fatal("spork name > 40 chars must error")
	}
	// description longer than 400 → error
	desc := strings.Repeat("x", 401)
	if _, err := buildProposalPayloadWith(api, "spork.create", map[string]string{"name": "valid name", "description": desc}); err == nil {
		t.Fatal("spork description > 400 chars must error")
	}
	// a name WITH SPACES within bounds → ok
	if _, err := buildProposalPayloadWith(api, "spork.create", map[string]string{"name": "My Test Spork", "description": "ok"}); err != nil {
		t.Fatalf("valid spork name with spaces must succeed: %v", err)
	}
}

// TestUnlockStakeEntriesKindRemoved: round-3 review P1 — the governance
// envelope cannot carry the token standard UnlockLiquidityStakeEntries selects
// its target with, so the kind must be absent from the catalog and refuse to
// build (never silently propose a ZNN-targeting action the user didn't ask for).
func TestUnlockStakeEntriesKindRemoved(t *testing.T) {
	for _, k := range proposeKinds() {
		if k.Kind == "liquidity.unlockStakeEntries" {
			t.Fatal("liquidity.unlockStakeEntries must not be offered in the propose catalog")
		}
	}
	_, err := buildProposalPayloadWith(nil, "liquidity.unlockStakeEntries", map[string]string{"zts": "zts1znnxxxxxxxxxxxxx9z4ulx"})
	if err == nil {
		t.Fatal("building the removed kind must fail closed")
	}
}
