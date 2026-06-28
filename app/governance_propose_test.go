package app

import (
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
