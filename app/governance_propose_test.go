package app

import (
	"math/big"
	"testing"

	embedded "github.com/0x3639/znn-sdk-go/api/embedded"
	"github.com/zenon-network/go-zenon/common/types"
)

func TestGetProposeKinds_HasSporkAndCustom(t *testing.T) {
	s := newNomService(newTestNode(t), newTestWalletService(t), nil)
	kinds, err := s.GetProposeKinds()
	if err != nil {
		t.Fatalf("GetProposeKinds err: %v", err)
	}
	byId := map[string]ProposeKindDTO{}
	for _, k := range kinds {
		byId[k.Kind] = k
	}
	for _, want := range []string{"spork.create", "spork.activate", "custom"} {
		if _, ok := byId[want]; !ok {
			t.Fatalf("missing kind %q", want)
		}
	}
	if byId["spork.create"].Group != "Spork" || len(byId["spork.create"].Fields) != 2 {
		t.Fatalf("spork.create schema wrong: %+v", byId["spork.create"])
	}
	if byId["custom"].Group != "Custom" {
		t.Fatalf("custom group wrong: %+v", byId["custom"])
	}
}

func TestBuildProposalPayload_SporkCreate(t *testing.T) {
	api := embedded.NewGovernanceApi(nil)
	// build directly via the SDK helper to confirm our dispatcher mirrors it
	want := api.PayloadSporkCreate("MySpork", "desc")
	got, err := buildProposalPayloadWith(api, "spork.create", map[string]string{"name": "MySpork", "description": "desc"})
	if err != nil {
		t.Fatalf("build err: %v", err)
	}
	if got.Destination != want.Destination || got.Data != want.Data {
		t.Fatalf("spork.create payload mismatch: got %+v want %+v", got, want)
	}
}

func TestBuildProposalPayload_Custom(t *testing.T) {
	api := embedded.NewGovernanceApi(nil)
	dest := types.SporkContract.String()
	got, err := buildProposalPayloadWith(api, "custom", map[string]string{"destination": dest, "data": "AAEC"})
	if err != nil {
		t.Fatalf("custom err: %v", err)
	}
	if got.Destination != types.SporkContract || got.Data != "AAEC" {
		t.Fatalf("custom payload wrong: %+v", got)
	}
	if _, err := buildProposalPayloadWith(api, "custom", map[string]string{"destination": dest, "data": "not base64!!"}); err == nil {
		t.Fatal("invalid base64 data must error")
	}
	if _, err := buildProposalPayloadWith(api, "custom", map[string]string{"destination": "nope", "data": "AAEC"}); err == nil {
		t.Fatal("invalid destination must error")
	}
}

func TestBuildProposalPayload_UnknownKind(t *testing.T) {
	api := embedded.NewGovernanceApi(nil)
	if _, err := buildProposalPayloadWith(api, "bogus.kind", map[string]string{}); err == nil {
		t.Fatal("unknown kind must error")
	}
}

func TestPrepareProposeAction_Validation(t *testing.T) {
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
	api := embedded.NewGovernanceApi(nil)
	cases := []struct {
		kind   string
		params map[string]string
		want   embedded.ProposalPayload
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
