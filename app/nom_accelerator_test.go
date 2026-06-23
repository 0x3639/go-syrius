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
