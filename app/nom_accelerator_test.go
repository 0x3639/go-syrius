package app

import (
	"math/big"
	"testing"

	embedded "github.com/0x3639/znn-sdk-go/api/embedded"
	"github.com/zenon-network/go-zenon/common/types"
)

func TestProjectDTONilSafe(t *testing.T) {
	// A project with nil funds/votes/phases must map without panicking.
	p := &embedded.Project{Name: "Proj", Status: 1}
	dto := projectDTO(p)
	if dto.Name != "Proj" || dto.Status != 1 {
		t.Fatalf("unexpected dto: %+v", dto)
	}
	if dto.ZnnFundsNeeded != "0" || dto.QsrFundsNeeded != "0" {
		t.Fatalf("nil funds must map to \"0\": %+v", dto)
	}
	if dto.Votes.Total != 0 || dto.Phases == nil {
		t.Fatalf("nil votes/phases must map to zero/empty, got %+v", dto)
	}
}

func TestProjectDTOMapsFieldsAndPhases(t *testing.T) {
	owner, _ := types.ParseAddress("z1qzal6c5s9rjnnxd2z7dvdhjxpmmj4fmw56a0mz")
	p := &embedded.Project{
		Id:             types.HexToHashPanic("0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20"),
		Owner:          owner,
		Name:           "Proj",
		ZnnFundsNeeded: big.NewInt(150),
		QsrFundsNeeded: big.NewInt(250),
		Status:         2,
		Votes:          &embedded.VoteBreakdown{Total: 5, Yes: 3, No: 2},
		Phases: []*embedded.Phase{{
			Phase: &embedded.PhaseInfo{Name: "P1", ZnnFundsNeeded: big.NewInt(10), QsrFundsNeeded: big.NewInt(20), Status: 1},
			Votes: &embedded.VoteBreakdown{Total: 4, Yes: 4, No: 0},
		}},
	}
	dto := projectDTO(p)
	if dto.Owner != owner.String() || dto.ZnnFundsNeeded != "150" || dto.Votes.Yes != 3 {
		t.Fatalf("project fields not mapped: %+v", dto)
	}
	if len(dto.Phases) != 1 || dto.Phases[0].Name != "P1" || dto.Phases[0].Votes.Yes != 4 {
		t.Fatalf("phase fields not mapped: %+v", dto.Phases)
	}
}

func TestAcceleratorReadsGuardInputs(t *testing.T) {
	s := newNomService(newTestNode(t), newTestWalletService(t), nil)

	// Invalid hash is rejected before any node use.
	if _, err := s.GetProject("not-a-hash"); err == nil {
		t.Fatal("GetProject: invalid hash must error")
	}
	if _, err := s.GetPhase("not-a-hash"); err == nil {
		t.Fatal("GetPhase: invalid hash must error")
	}
	// Valid hash with a disconnected node surfaces "not connected".
	valid := "0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20"
	if _, err := s.GetProject(valid); err == nil || err.Error() != "not connected" {
		t.Fatalf("GetProject: want not-connected, got %v", err)
	}
	// Browse list also needs a node.
	if _, err := s.GetProjects(0, 20); err == nil || err.Error() != "not connected" {
		t.Fatalf("GetProjects: want not-connected, got %v", err)
	}
	// Voting eligibility needs an unlocked wallet (locked in this test).
	if _, err := s.GetVotablePillars(); err == nil {
		t.Fatal("GetVotablePillars: locked wallet must error")
	}
}

func TestPrepareDonateValidatesInput(t *testing.T) {
	s := newNomService(newTestNode(t), newTestWalletService(t), nil)
	bad := []struct{ amount, token string }{
		{"0", "ZNN"},    // non-positive
		{"-5", "ZNN"},   // negative
		{"abc", "ZNN"},  // unparseable
		{"100", "DOGE"}, // unknown token
		{"100", ""},     // empty token
	}
	for _, c := range bad {
		if _, err := s.PrepareDonate(c.amount, c.token); err == nil {
			t.Fatalf("donate(%q,%q): expected validation error", c.amount, c.token)
		}
	}
	// Valid input passes validation and only then hits the disconnected node.
	if _, err := s.PrepareDonate("100", "QSR"); err == nil || err.Error() != "not connected" {
		t.Fatalf("valid donate should hit not-connected; got %v", err)
	}
}

func TestDonateTemplateTokenStandards(t *testing.T) {
	api := embedded.NewAcceleratorApi(nil) // builder needs no client
	amt := big.NewInt(123)
	d := api.Donate(amt, types.QsrTokenStandard)
	if d.ToAddress != types.AcceleratorContract || d.TokenStandard != types.QsrTokenStandard || d.Amount.Cmp(amt) != 0 {
		t.Fatalf("donate template wrong: %+v", d)
	}
}

func TestVoteConstantsMatchOnChainAuthority(t *testing.T) {
	// Regression guard for the v0.1.18 SDK fix: a "yes" vote MUST serialize as 0.
	if embedded.VoteYes != 0 || embedded.VoteNo != 1 || embedded.VoteAbstain != 2 {
		t.Fatalf("vote constants drifted: yes=%d no=%d abstain=%d", embedded.VoteYes, embedded.VoteNo, embedded.VoteAbstain)
	}
}

func TestPrepareVoteValidatesInput(t *testing.T) {
	s := newNomService(newTestNode(t), newTestWalletService(t), nil)
	valid := "0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20"
	// bad hash
	if _, err := s.PrepareVote("nope", "MyPillar", embedded.VoteYes); err == nil {
		t.Fatal("vote: bad id must error")
	}
	// empty pillar name
	if _, err := s.PrepareVote(valid, "  ", embedded.VoteYes); err == nil {
		t.Fatal("vote: empty pillar must error")
	}
	// out-of-range vote value
	if _, err := s.PrepareVote(valid, "MyPillar", 7); err == nil {
		t.Fatal("vote: out-of-range vote must error")
	}
	// valid input → disconnected node
	if _, err := s.PrepareVote(valid, "MyPillar", embedded.VoteNo); err == nil || err.Error() != "not connected" {
		t.Fatalf("valid vote should hit not-connected; got %v", err)
	}
}

func TestVoteTemplate(t *testing.T) {
	api := embedded.NewAcceleratorApi(nil)
	h := types.HexToHashPanic("0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20")
	v := api.VoteByName(h, "MyPillar", embedded.VoteYes)
	if v.ToAddress != types.AcceleratorContract || v.TokenStandard != types.ZnnTokenStandard || v.Amount.Sign() != 0 {
		t.Fatalf("vote template wrong: %+v", v)
	}
}
