package app

import (
	"testing"

	embedded "github.com/0x3639/znn-sdk-go/api/embedded"
	"github.com/zenon-network/go-zenon/common/types"
)

func TestActionDTO_MapsFieldsAndVotes(t *testing.T) {
	id := types.HexToHashPanic("0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20")
	// The SDK client Action has FLAT fields (it is NOT the go-zenon server
	// Action that embeds *definition.ActionVariable).
	voteId := types.HexToHashPanic("aabbccddeeff00112233445566778899aabbccddeeff00112233445566778899")
	a := &embedded.Action{
		Id:                    id,
		Owner:                 types.GovernanceContract,
		Name:                  "Act",
		Description:           "desc",
		Url:                   "https://zenon.org",
		Destination:           types.SporkContract,
		Data:                  "AAEC",
		Type:                  1,
		Round:                 0,
		CurrentVoteId:         voteId,
		Status:                0,
		Executed:              false,
		Expired:               false,
		ActivePillarThreshold: 66,
		DirectionalThreshold:  50,
		VotingPeriod:          3888000,
		Votes:                 &embedded.VoteBreakdown{Total: 3, Yes: 2, No: 1},
	}
	d := actionDTO(a)
	if d.Id != id.String() || d.Destination != types.SporkContract.String() {
		t.Fatalf("hash/addr mapping wrong: %+v", d)
	}
	if d.CurrentVoteId != voteId.String() {
		t.Fatalf("currentVoteId mapping wrong: got %q want %q", d.CurrentVoteId, voteId.String())
	}
	if d.Type != 1 || d.ActivePillarThreshold != 66 || d.DirectionalThreshold != 50 {
		t.Fatalf("type/threshold mapping wrong: %+v", d)
	}
	if d.Votes.Yes != 2 || d.Votes.No != 1 || d.Votes.Total != 3 {
		t.Fatalf("votes mapping wrong: %+v", d.Votes)
	}
}

func TestGovernancePrepares_BlockedOnMainnet(t *testing.T) {
	s := newNomService(newTestNode(t), newTestWalletService(t), nil)
	s.node.chainID = mainnetChainID // simulate being connected to mainnet
	valid := "0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20"
	if _, err := s.PrepareGovernanceVote(valid, "P1", 0); err != errGovernanceMainnet {
		t.Fatalf("vote must be blocked on mainnet; got %v", err)
	}
	if _, err := s.PrepareExecuteAction(valid); err != errGovernanceMainnet {
		t.Fatalf("execute must be blocked on mainnet; got %v", err)
	}
	if _, err := s.PrepareProposeAction("Act", "d", "https://zenon.org", "spork.create", map[string]string{"name": "MySpork", "description": "d"}); err != errGovernanceMainnet {
		t.Fatalf("propose must be blocked on mainnet; got %v", err)
	}
}

func TestActionDTO_NilSafe(t *testing.T) {
	d := actionDTO(nil)
	if d.Id != "" || d.Votes.Total != 0 {
		t.Fatalf("nil action must map to zero DTO: %+v", d)
	}
}

func TestGetActions_NotConnected(t *testing.T) {
	s := newNomService(newTestNode(t), newTestWalletService(t), nil)
	if _, err := s.GetActions(0, 20); err == nil || err.Error() != "not connected" {
		t.Fatalf("want not connected; got %v", err)
	}
}

func TestGetAction_BadId(t *testing.T) {
	s := newNomService(newTestNode(t), newTestWalletService(t), nil)
	if _, err := s.GetAction("not-a-hash"); err == nil {
		t.Fatal("bad id must error")
	}
}

func TestPrepareGovernanceVote_Validation(t *testing.T) {
	s := newNomService(newTestNode(t), newTestWalletService(t), nil)
	if _, err := s.PrepareGovernanceVote("bad-id", "P1", 0); err == nil {
		t.Fatal("bad id must error")
	}
	valid := "0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20"
	if _, err := s.PrepareGovernanceVote(valid, "  ", 0); err == nil {
		t.Fatal("empty pillar must error")
	}
	if _, err := s.PrepareGovernanceVote(valid, "P1", 9); err == nil {
		t.Fatal("invalid vote value must error")
	}
	if _, err := s.PrepareGovernanceVote(valid, "P1", 0); err == nil || err.Error() != "not connected" {
		t.Fatalf("valid vote should hit not-connected; got %v", err)
	}
}

func TestPrepareExecuteAction_Validation(t *testing.T) {
	s := newNomService(newTestNode(t), newTestWalletService(t), nil)
	if _, err := s.PrepareExecuteAction("bad-id"); err == nil {
		t.Fatal("bad id must error")
	}
	valid := "0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20"
	if _, err := s.PrepareExecuteAction(valid); err == nil || err.Error() != "not connected" {
		t.Fatalf("valid execute should hit not-connected; got %v", err)
	}
}

// Build the SDK templates directly to catch a pack-panic at construction
// (the lesson from the v0.1.19 UpdatePhase regression).
func TestGovernanceWriteTemplates_NoPanic(t *testing.T) {
	api := embedded.NewGovernanceApi(nil)
	h := types.HexToHashPanic("0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20")
	vote := api.VoteByName(h, "P1", embedded.VoteYes)
	if vote.ToAddress != types.GovernanceContract || vote.TokenStandard != types.ZnnTokenStandard || vote.Amount.Sign() != 0 {
		t.Fatalf("vote template wrong: %+v", vote)
	}
	exec := api.ExecuteAction(h)
	if exec.ToAddress != types.GovernanceContract || exec.Amount.Sign() != 0 {
		t.Fatalf("execute template wrong: %+v", exec)
	}
}
