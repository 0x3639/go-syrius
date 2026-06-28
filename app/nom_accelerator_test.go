package app

import (
	"math/big"
	"strings"
	"testing"

	embedded "github.com/0x3639/znn-sdk-go/api/embedded"
	"github.com/zenon-network/go-zenon/common/types"
	constants "github.com/zenon-network/go-zenon/vm/constants"
	definition "github.com/zenon-network/go-zenon/vm/embedded/definition"
)

// strings32 is 31 zero bytes (hex) so a 1-byte prefix forms a 32-byte hash.
var strings32 = "00000000000000000000000000000000000000000000000000000000000000"[:62]

func TestMyActiveProjects(t *testing.T) {
	mine, _ := types.ParseAddress("z1qzal6c5s9rjnnxd2z7dvdhjxpmmj4fmw56a0mz")
	other, _ := types.ParseAddress("z1qr4pexnnfaexqqz8nscjjcsajy5hdqfkgadvwx")
	projects := []*embedded.Project{
		{Id: types.HexToHashPanic("a1" + strings32), Name: "MineActive", Owner: mine, Status: 1},
		{Id: types.HexToHashPanic("a2" + strings32), Name: "MineVoting", Owner: mine, Status: 0},
		{Id: types.HexToHashPanic("a3" + strings32), Name: "MineDone", Owner: mine, Status: 4},
		{Id: types.HexToHashPanic("a4" + strings32), Name: "OtherActive", Owner: other, Status: 1},
		nil,
	}
	out := myActiveProjects(projects, mine)
	if len(out) != 1 {
		t.Fatalf("expected only the owner's Active project, got %d: %+v", len(out), out)
	}
	if out[0].Name != "MineActive" {
		t.Fatalf("expected MineActive, got %q", out[0].Name)
	}
}

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

func TestPrepareProjectWritesValidateInput(t *testing.T) {
	s := newNomService(newTestNode(t), newTestWalletService(t), nil)
	longName := strings.Repeat("x", 31)
	goodURL := "https://zenon.org"

	// CreateProject field validation (no id involved).
	bad := []struct{ name, desc, url, znn, qsr string }{
		{"", "desc", goodURL, "1", "1"},         // empty name
		{longName, "desc", goodURL, "1", "1"},   // name too long
		{"Proj", "", goodURL, "1", "1"},         // empty description
		{"Proj", "desc", "not a url", "1", "1"}, // bad url
		{"Proj", "desc", goodURL, "x", "1"},     // bad znn
		{"Proj", "desc", goodURL, "1", "x"},     // bad qsr
	}
	for i, c := range bad {
		if _, err := s.PrepareCreateProject(c.name, c.desc, c.url, c.znn, c.qsr); err == nil {
			t.Fatalf("create case %d: expected validation error", i)
		}
	}
	// Valid create → not connected.
	if _, err := s.PrepareCreateProject("Proj", "A real description", goodURL, "100", "1000"); err == nil || err.Error() != "not connected" {
		t.Fatalf("valid create should hit not-connected; got %v", err)
	}

	// AddPhase / UpdatePhase additionally validate the id.
	valid := "0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20"
	if _, err := s.PrepareAddPhase("bad-id", "Proj", "desc", goodURL, "1", "1"); err == nil {
		t.Fatal("addphase: bad id must error")
	}
	if _, err := s.PrepareAddPhase(valid, "", "desc", goodURL, "1", "1"); err == nil {
		t.Fatal("addphase: empty name must error")
	}
	if _, err := s.PrepareAddPhase(valid, "Phase", "desc", goodURL, "1", "1"); err == nil || err.Error() != "not connected" {
		t.Fatalf("valid addphase should hit not-connected; got %v", err)
	}
	if _, err := s.PrepareUpdatePhase("bad-id", "Phase", "desc", goodURL, "1", "1"); err == nil {
		t.Fatal("updatephase: bad id must error")
	}
	if _, err := s.PrepareUpdatePhase(valid, "Phase", "desc", goodURL, "1", "1"); err == nil || err.Error() != "not connected" {
		t.Fatalf("valid updatephase should hit not-connected; got %v", err)
	}
}

func TestProjectWriteTemplateTokenStandards(t *testing.T) {
	api := embedded.NewAcceleratorApi(nil)
	h := types.HexToHashPanic("0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20")
	create := api.CreateProject("Proj", "desc", "https://zenon.org", big.NewInt(100), big.NewInt(1000))
	if create.ToAddress != types.AcceleratorContract || create.TokenStandard != types.ZnnTokenStandard {
		t.Fatalf("create template wrong: %+v", create)
	}
	if create.Amount.Cmp(constants.ProjectCreationAmount) != 0 {
		t.Fatalf("create fee=%v want %v", create.Amount, constants.ProjectCreationAmount)
	}
	add := api.AddPhase(h, "Phase", "desc", "https://zenon.org", big.NewInt(1), big.NewInt(1))
	if add.ToAddress != types.AcceleratorContract || add.Amount.Sign() != 0 {
		t.Fatalf("addphase template wrong: %+v", add)
	}
	// UpdatePhase must build without panicking — the SDK <v0.1.19 packed the
	// wrong ABI method ("Update", 0 inputs) with 6 args and panicked here.
	update := api.UpdatePhase(h, "Phase", "desc", "https://zenon.org", big.NewInt(1), big.NewInt(1))
	if update.ToAddress != types.AcceleratorContract || update.TokenStandard != types.ZnnTokenStandard || update.Amount.Sign() != 0 {
		t.Fatalf("updatephase template wrong: %+v", update)
	}
}

func TestBuildVotableItems(t *testing.T) {
	now := int64(1_000_000)
	openProj := &embedded.Project{
		Id:                types.HexToHashPanic("01" + strings32),
		Name:              "OpenAZ",
		Status:            0, // Voting
		CreationTimestamp: now - 10,
		ZnnFundsNeeded:    big.NewInt(100), QsrFundsNeeded: big.NewInt(200),
		Votes: &embedded.VoteBreakdown{Total: 1, Yes: 1, No: 0},
	}
	expiredProj := &embedded.Project{
		Id:                types.HexToHashPanic("02" + strings32),
		Name:              "ExpiredAZ", Status: 0,
		CreationTimestamp: now - int64(constants.AcceleratorProjectVotingPeriod) - 1,
		Votes:             &embedded.VoteBreakdown{},
	}
	activeWithOpenPhase := &embedded.Project{
		Id:   types.HexToHashPanic("03" + strings32),
		Name: "ActiveAZ", Status: 1, // Active
		Phases: []*embedded.Phase{{
			Phase: &embedded.PhaseInfo{
				Id:   types.HexToHashPanic("04" + strings32),
				Name: "PhaseOne", Status: 0, // Voting
				ZnnFundsNeeded: big.NewInt(5), QsrFundsNeeded: big.NewInt(6),
			},
			Votes: &embedded.VoteBreakdown{Total: 2, Yes: 1, No: 1},
		}},
	}
	activePaidPhase := &embedded.Project{
		Id: types.HexToHashPanic("05" + strings32), Name: "DoneAZ", Status: 1,
		Phases: []*embedded.Phase{{Phase: &embedded.PhaseInfo{Name: "Paid", Status: 2}, Votes: &embedded.VoteBreakdown{}}},
	}

	items := buildVotableItems([]*embedded.Project{openProj, expiredProj, activeWithOpenPhase, activePaidPhase}, now)
	if len(items) != 2 {
		t.Fatalf("expected 2 votable items (open project + open phase), got %d: %+v", len(items), items)
	}
	if items[0].Kind != "project" || items[0].Name != "OpenAZ" || items[0].Votes.Yes != 1 {
		t.Fatalf("project item wrong: %+v", items[0])
	}
	if items[1].Kind != "phase" || items[1].Name != "PhaseOne" || items[1].ProjectName != "ActiveAZ" {
		t.Fatalf("phase item wrong: %+v", items[1])
	}
	if items[1].ZnnFundsNeeded != "5" || items[1].Votes.Total != 2 {
		t.Fatalf("phase funds/votes wrong: %+v", items[1])
	}
	// Default annotation: no pillars yet → not flagged needs-vote.
	if items[0].NeedsMyVote || items[0].MyVotes != nil {
		t.Fatalf("expected unannotated item: %+v", items[0])
	}
}

func TestAnnotateMyVotes(t *testing.T) {
	idA := types.HexToHashPanic("0a" + strings32)
	idB := types.HexToHashPanic("0b" + strings32)
	items := []VotableItem{{Id: idA.String()}, {Id: idB.String()}}
	// Pillar voted "no" (1) on A only; B is nil/absent → not voted.
	votes := []*definition.PillarVote{{Id: idA, Name: "MyPillar", Vote: 1}, nil}
	annotateMyVotes(items, "MyPillar", votes)
	if len(items[0].MyVotes) != 1 || items[0].MyVotes[0].Pillar != "MyPillar" || items[0].MyVotes[0].Vote != 1 {
		t.Fatalf("A should be voted no by MyPillar: %+v", items[0])
	}
	if items[0].NeedsMyVote {
		t.Fatalf("A is voted, must not need vote: %+v", items[0])
	}
	if items[1].MyVotes[0].Vote != -1 || !items[1].NeedsMyVote {
		t.Fatalf("B unvoted must be -1 + needsMyVote: %+v", items[1])
	}
}

func TestAcceleratorVoteReadsGuard(t *testing.T) {
	s := newNomService(newTestNode(t), newTestWalletService(t), nil)
	// Not connected (test node has no client) → both reads error, no panic.
	if _, err := s.GetActivePillarCount(); err == nil {
		t.Fatal("GetActivePillarCount must error when not connected")
	}
	if _, err := s.GetVotableForMyPillars(); err == nil {
		t.Fatal("GetVotableForMyPillars must error when not connected/locked")
	}
}
