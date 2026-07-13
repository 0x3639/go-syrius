package app

import (
	"errors"
	"fmt"
	"math/big"
	"regexp"
	"strings"

	embedded "github.com/0x3639/znn-sdk-go/api/embedded"
	"github.com/zenon-network/go-zenon/common/types"
	constants "github.com/zenon-network/go-zenon/vm/constants"
	definition "github.com/zenon-network/go-zenon/vm/embedded/definition"
)

// Accelerator-Z project/phase status values (mirrors go-zenon definition).
const (
	statusVoting    = 0
	statusActive    = 1
	statusPaid      = 2
	statusClosed    = 3
	statusCompleted = 4
)

// buildVotableItems returns the currently votable projects (Voting + 14-day
// window open) and the open current phase of each Active project, mapped to
// VotableItems WITHOUT per-pillar annotation. nowUnix is the reference time for
// the voting-window check. Pure: no node I/O.
func buildVotableItems(projects []*embedded.Project, nowUnix int64) []VotableItem {
	out := make([]VotableItem, 0)
	for _, p := range projects {
		if p == nil {
			continue
		}
		switch int(p.Status) {
		case statusVoting:
			if p.CreationTimestamp+int64(constants.AcceleratorProjectVotingPeriod) >= nowUnix {
				out = append(out, VotableItem{
					Kind:           "project",
					Id:             p.Id.String(),
					ProjectId:      p.Id.String(),
					ProjectName:    p.Name,
					Name:           p.Name,
					ZnnFundsNeeded: bigStr(p.ZnnFundsNeeded),
					QsrFundsNeeded: bigStr(p.QsrFundsNeeded),
					Votes:          voteBreakdownDTO(p.Votes),
				})
			}
		case statusActive:
			if len(p.Phases) == 0 {
				continue
			}
			cur := p.Phases[len(p.Phases)-1]
			if cur == nil || cur.Phase == nil || int(cur.Phase.Status) != statusVoting {
				continue
			}
			out = append(out, VotableItem{
				Kind:           "phase",
				Id:             cur.Phase.Id.String(),
				ProjectId:      p.Id.String(),
				ProjectName:    p.Name,
				Name:           cur.Phase.Name,
				ZnnFundsNeeded: bigStr(cur.Phase.ZnnFundsNeeded),
				QsrFundsNeeded: bigStr(cur.Phase.QsrFundsNeeded),
				Votes:          voteBreakdownDTO(cur.Votes),
			})
		}
	}
	return out
}

// bigStr renders a possibly-nil *big.Int as a base-10 string ("0" when nil).
func bigStr(v *big.Int) string {
	if v == nil {
		return "0"
	}
	return v.String()
}

func voteBreakdownDTO(v *embedded.VoteBreakdown) VoteBreakdownDTO {
	if v == nil {
		return VoteBreakdownDTO{}
	}
	return VoteBreakdownDTO{Total: v.Total, Yes: v.Yes, No: v.No}
}

func phaseDTO(p *embedded.Phase) PhaseDTO {
	if p == nil || p.Phase == nil {
		return PhaseDTO{}
	}
	pi := p.Phase
	return PhaseDTO{
		Id:                pi.Id.String(),
		ProjectId:         pi.ProjectID.String(),
		Name:              pi.Name,
		Description:       pi.Description,
		Url:               pi.Url,
		ZnnFundsNeeded:    bigStr(pi.ZnnFundsNeeded),
		QsrFundsNeeded:    bigStr(pi.QsrFundsNeeded),
		CreationTimestamp: pi.CreationTimestamp,
		AcceptedTimestamp: pi.AcceptedTimestamp,
		Status:            int(pi.Status),
		Votes:             voteBreakdownDTO(p.Votes),
	}
}

func projectDTO(p *embedded.Project) ProjectDTO {
	if p == nil {
		return ProjectDTO{Phases: []PhaseDTO{}}
	}
	phases := make([]PhaseDTO, 0, len(p.Phases))
	for _, ph := range p.Phases {
		phases = append(phases, phaseDTO(ph))
	}
	return ProjectDTO{
		Id:                  p.Id.String(),
		Owner:               p.Owner.String(),
		Name:                p.Name,
		Description:         p.Description,
		Url:                 p.Url,
		ZnnFundsNeeded:      bigStr(p.ZnnFundsNeeded),
		QsrFundsNeeded:      bigStr(p.QsrFundsNeeded),
		CreationTimestamp:   p.CreationTimestamp,
		LastUpdateTimestamp: p.LastUpdateTimestamp,
		Status:              int(p.Status),
		Votes:               voteBreakdownDTO(p.Votes),
		Phases:              phases,
	}
}

// acceleratorURLRe mirrors the project/phase URL rule enforced on-chain in
// go-zenon vm/embedded/implementation/accelerator.go.
var acceleratorURLRe = regexp.MustCompile(`^([Hh][Tt][Tt][Pp][Ss]?://)?[a-zA-Z0-9]{2,60}\.[a-zA-Z]{1,6}([-a-zA-Z0-9()@:%_+.~#?&/=]{0,100})$`)

// validateProjectFields mirrors the on-chain create/add/update validation and
// parses the funds amounts. It performs no node I/O.
func validateProjectFields(name, description, url, znnNeeded, qsrNeeded string) (*big.Int, *big.Int, error) {
	if l := len(name); l == 0 || l > constants.ProjectNameLengthMax {
		return nil, nil, fmt.Errorf("name must be 1-%d characters", constants.ProjectNameLengthMax)
	}
	if l := len(description); l == 0 || l > constants.ProjectDescriptionLengthMax {
		return nil, nil, fmt.Errorf("description must be 1-%d characters", constants.ProjectDescriptionLengthMax)
	}
	if len(url) == 0 || !acceleratorURLRe.MatchString(url) {
		return nil, nil, errors.New("invalid URL")
	}
	znn, ok := new(big.Int).SetString(strings.TrimSpace(znnNeeded), 10)
	if !ok || znn.Sign() < 0 {
		return nil, nil, errors.New("invalid ZNN funds amount")
	}
	qsr, ok := new(big.Int).SetString(strings.TrimSpace(qsrNeeded), 10)
	if !ok || qsr.Sign() < 0 {
		return nil, nil, errors.New("invalid QSR funds amount")
	}
	if znn.Cmp(constants.ProjectZnnMaximumFunds) > 0 {
		return nil, nil, errors.New("ZNN funds exceed the maximum")
	}
	if qsr.Cmp(constants.ProjectQsrMaximumFunds) > 0 {
		return nil, nil, errors.New("QSR funds exceed the maximum")
	}
	return znn, qsr, nil
}

// parseHash trims and parses a 0x… hash id (project or phase).
func parseHash(id string) (types.Hash, error) {
	return types.HexToHash(strings.TrimSpace(id))
}

// GetProjects returns one page of Accelerator-Z projects (as the node orders
// them). pageSize is clamped to [1,50].
func (s *NomService) GetProjects(pageIndex, pageSize uint32) (ProjectListDTO, error) {
	if pageSize == 0 || pageSize > 50 {
		pageSize = 50
	}
	client := s.node.currentClient()
	if client == nil {
		return ProjectListDTO{}, errors.New("not connected")
	}
	list, err := client.AcceleratorApi.GetAll(pageIndex, pageSize)
	if err != nil {
		return ProjectListDTO{}, err
	}
	out := ProjectListDTO{Count: list.Count, List: make([]ProjectDTO, 0, len(list.List))}
	for _, p := range list.List {
		out.List = append(out.List, projectDTO(p))
	}
	return out, nil
}

// GetProject returns a single project (with embedded phases + vote tally).
func (s *NomService) GetProject(id string) (ProjectDTO, error) {
	h, err := parseHash(id)
	if err != nil {
		return ProjectDTO{}, fmt.Errorf("invalid project id: %w", err)
	}
	client := s.node.currentClient()
	if client == nil {
		return ProjectDTO{}, errors.New("not connected")
	}
	p, err := client.AcceleratorApi.GetProjectById(h)
	if err != nil {
		return ProjectDTO{}, err
	}
	return projectDTO(p), nil
}

// GetPhase returns a single phase (with its vote tally).
func (s *NomService) GetPhase(id string) (PhaseDTO, error) {
	h, err := parseHash(id)
	if err != nil {
		return PhaseDTO{}, fmt.Errorf("invalid phase id: %w", err)
	}
	client := s.node.currentClient()
	if client == nil {
		return PhaseDTO{}, errors.New("not connected")
	}
	ph, err := client.AcceleratorApi.GetPhaseById(h)
	if err != nil {
		return PhaseDTO{}, err
	}
	return phaseDTO(ph), nil
}

// GetVotablePillars returns the names of Pillars the active address owns, used
// to gate and drive the voting UI. Empty slice ⇒ the address cannot vote.
func (s *NomService) GetVotablePillars() ([]string, error) {
	addr, ok := s.wallet.activeAddress()
	if !ok {
		return nil, errLocked
	}
	client := s.node.currentClient()
	if client == nil {
		return nil, errors.New("not connected")
	}
	pillars, err := client.PillarApi.GetByOwner(addr)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(pillars))
	for _, p := range pillars {
		names = append(names, p.Name)
	}
	return names, nil
}

// PrepareDonate builds a Donate template (ZNN or QSR) for the Accelerator and
// routes it through confirm-what-you-sign. Inputs are validated first.
func (s *NomService) PrepareDonate(amount, token string) (CallPreview, error) {
	var ts types.ZenonTokenStandard
	var symbol string
	switch strings.ToUpper(strings.TrimSpace(token)) {
	case "ZNN":
		ts, symbol = types.ZnnTokenStandard, "ZNN"
	case "QSR":
		ts, symbol = types.QsrTokenStandard, "QSR"
	default:
		return CallPreview{}, errors.New("donation token must be ZNN or QSR")
	}
	amt, ok := new(big.Int).SetString(strings.TrimSpace(amount), 10)
	if !ok || amt.Sign() <= 0 {
		return CallPreview{}, errors.New("donation amount must be greater than 0")
	}
	client := s.node.currentClient()
	if client == nil {
		return CallPreview{}, errors.New("not connected")
	}
	template := client.AcceleratorApi.Donate(amt, ts)
	return s.tx.prepareCall(template,
		callExpect{to: types.AcceleratorContract, zts: ts, amount: template.Amount, data: append([]byte(nil), template.Data...)},
		fmt.Sprintf("Donate %s %s to Accelerator-Z", formatBaseAmount(amt.String(), 8), symbol))
}

// PrepareVote builds a VoteByName template for one of the active address's
// Pillars. vote MUST be embedded.VoteYes/VoteNo/VoteAbstain. Field validation
// runs before any node use; Pillar ownership is enforced on-chain.
func (s *NomService) PrepareVote(id, pillarName string, vote uint8) (CallPreview, error) {
	h, err := parseHash(id)
	if err != nil {
		return CallPreview{}, fmt.Errorf("invalid proposal id: %w", err)
	}
	name := strings.TrimSpace(pillarName)
	if name == "" {
		return CallPreview{}, errors.New("pillar name is required")
	}
	if vote != embedded.VoteYes && vote != embedded.VoteNo && vote != embedded.VoteAbstain {
		return CallPreview{}, errors.New("vote must be yes, no, or abstain")
	}
	client := s.node.currentClient()
	if client == nil {
		return CallPreview{}, errors.New("not connected")
	}
	template := client.AcceleratorApi.VoteByName(h, name, vote)
	label := map[uint8]string{embedded.VoteYes: "yes", embedded.VoteNo: "no", embedded.VoteAbstain: "abstain"}[vote]
	return s.tx.prepareCall(template,
		callExpect{to: types.AcceleratorContract, zts: types.ZnnTokenStandard, amount: template.Amount, data: append([]byte(nil), template.Data...)},
		fmt.Sprintf("Vote %s on %s as %s", label, id, name))
}

// PrepareCreateProject builds a CreateProject template. The 1 ZNN fee is read
// from the template, never hardcoded. Fields are validated before node use.
func (s *NomService) PrepareCreateProject(name, description, url, znnNeeded, qsrNeeded string) (CallPreview, error) {
	znn, qsr, err := validateProjectFields(name, description, url, znnNeeded, qsrNeeded)
	if err != nil {
		return CallPreview{}, err
	}
	client := s.node.currentClient()
	if client == nil {
		return CallPreview{}, errors.New("not connected")
	}
	template := client.AcceleratorApi.CreateProject(name, description, url, znn, qsr)
	return s.tx.prepareCall(template,
		callExpect{to: types.AcceleratorContract, zts: types.ZnnTokenStandard, amount: template.Amount, data: append([]byte(nil), template.Data...)},
		fmt.Sprintf("Create project %q — requesting %s ZNN / %s QSR, %s (1 ZNN fee)",
			name, formatBaseAmount(znn.String(), 8), formatBaseAmount(qsr.String(), 8), url))
}

// PrepareAddPhase builds an AddPhase template for an existing project. Project
// ownership is enforced on-chain.
func (s *NomService) PrepareAddPhase(projectId, name, description, url, znnNeeded, qsrNeeded string) (CallPreview, error) {
	h, err := parseHash(projectId)
	if err != nil {
		return CallPreview{}, fmt.Errorf("invalid project id: %w", err)
	}
	znn, qsr, err := validateProjectFields(name, description, url, znnNeeded, qsrNeeded)
	if err != nil {
		return CallPreview{}, err
	}
	client := s.node.currentClient()
	if client == nil {
		return CallPreview{}, errors.New("not connected")
	}
	template := client.AcceleratorApi.AddPhase(h, name, description, url, znn, qsr)
	return s.tx.prepareCall(template,
		callExpect{to: types.AcceleratorContract, zts: types.ZnnTokenStandard, amount: template.Amount, data: append([]byte(nil), template.Data...)},
		fmt.Sprintf("Add phase %q to project %s — requesting %s ZNN / %s QSR",
			name, projectId, formatBaseAmount(znn.String(), 8), formatBaseAmount(qsr.String(), 8)))
}

// PrepareUpdatePhase builds an UpdatePhase template. On-chain UpdatePhase is
// keyed by the PROJECT id (not a phase id): the contract looks up the project
// and updates its current (last) phase, which must still be in voting. The id
// argument is therefore a project id, mirroring AddPhase.
func (s *NomService) PrepareUpdatePhase(projectId, name, description, url, znnNeeded, qsrNeeded string) (CallPreview, error) {
	h, err := parseHash(projectId)
	if err != nil {
		return CallPreview{}, fmt.Errorf("invalid project id: %w", err)
	}
	znn, qsr, err := validateProjectFields(name, description, url, znnNeeded, qsrNeeded)
	if err != nil {
		return CallPreview{}, err
	}
	client := s.node.currentClient()
	if client == nil {
		return CallPreview{}, errors.New("not connected")
	}
	template := client.AcceleratorApi.UpdatePhase(h, name, description, url, znn, qsr)
	return s.tx.prepareCall(template,
		callExpect{to: types.AcceleratorContract, zts: types.ZnnTokenStandard, amount: template.Amount, data: append([]byte(nil), template.Data...)},
		fmt.Sprintf("Update current phase of project %s to %q — requesting %s ZNN / %s QSR",
			projectId, name, formatBaseAmount(znn.String(), 8), formatBaseAmount(qsr.String(), 8)))
}

// annotateMyVotes records, for one pillar, its vote on each item (or -1 = not
// voted), and flags items the pillar still needs to vote on. votes is the result
// of GetPillarVotes(pillarName, ids); entries may be nil (no vote) and are keyed
// by their Id so ordering is irrelevant.
func annotateMyVotes(items []VotableItem, pillarName string, votes []*definition.PillarVote) {
	voted := make(map[string]int, len(votes))
	for _, v := range votes {
		if v != nil {
			voted[v.Id.String()] = int(v.Vote)
		}
	}
	for i := range items {
		vote := -1
		if vv, ok := voted[items[i].Id]; ok {
			vote = vv
		}
		items[i].MyVotes = append(items[i].MyVotes, PillarVoteState{Pillar: pillarName, Vote: vote})
		if vote == -1 {
			items[i].NeedsMyVote = true
		}
	}
}

// GetActivePillarCount returns the number of pillars (for AZ quorum math). Uses
// the AcceleratorApi/PillarApi page count.
func (s *NomService) GetActivePillarCount() (int, error) {
	client := s.node.currentClient()
	if client == nil {
		return 0, errors.New("not connected")
	}
	list, err := client.PillarApi.GetAll(0, 1)
	if err != nil {
		return 0, err
	}
	return list.Count, nil
}

// GetVotableForMyPillars returns the items currently open for voting (projects in
// their voting window + Active projects' open phases), annotated with the vote
// state of each pillar the active address owns. Drives the Vote view + the
// top-bar badge. Read-only.
func (s *NomService) GetVotableForMyPillars() ([]VotableItem, error) {
	addr, ok := s.wallet.activeAddress()
	if !ok {
		return nil, errLocked
	}
	client := s.node.currentClient()
	if client == nil {
		return nil, errors.New("not connected")
	}

	// Collect all projects (page through GetAll).
	all := make([]*embedded.Project, 0)
	var pageIndex uint32 = 0
	const pageSize uint32 = 50
	for {
		list, err := client.AcceleratorApi.GetAll(pageIndex, pageSize)
		if err != nil {
			return nil, err
		}
		all = append(all, list.List...)
		if len(all) >= list.Count || len(list.List) == 0 {
			break
		}
		pageIndex++
	}

	// Chain time, not wall-clock: the voting window is enforced on-chain against
	// momentum timestamps, so local clock skew must not hide valid votes or show
	// expired ones (mirrors the staking maturity check).
	m, err := client.LedgerApi.GetFrontierMomentum()
	if err != nil {
		return nil, err
	}
	items := buildVotableItems(all, frontierUnix(m))
	if len(items) == 0 {
		return items, nil
	}

	// Annotate with each owned pillar's vote across all votable ids.
	pillars, err := client.PillarApi.GetByOwner(addr)
	if err != nil {
		return nil, err
	}
	if len(pillars) == 0 {
		return items, nil // no pillar → nothing actionable; NeedsMyVote stays false
	}
	ids := make([]types.Hash, 0, len(items))
	for _, it := range items {
		if h, err := types.HexToHash(it.Id); err == nil {
			ids = append(ids, h)
		}
	}
	for _, p := range pillars {
		votes, err := client.AcceleratorApi.GetPillarVotes(p.Name, ids)
		if err != nil {
			return nil, err
		}
		annotateMyVotes(items, p.Name, votes)
	}
	return items, nil
}

// myActiveProjects filters projects to those owned by addr with Active status
// (approved, not yet finished) — the candidates for requesting a phase payout.
// Pure: no node I/O.
func myActiveProjects(projects []*embedded.Project, addr types.Address) []ProjectDTO {
	out := []ProjectDTO{}
	for _, p := range projects {
		if p == nil {
			continue
		}
		if p.Owner == addr && int(p.Status) == statusActive {
			out = append(out, projectDTO(p))
		}
	}
	return out
}

// GetMyProjects returns the active address's Active (approved, unfinished)
// Accelerator-Z projects — populates the "request a phase payout" project
// picker. Read-only.
func (s *NomService) GetMyProjects() ([]ProjectDTO, error) {
	addr, ok := s.wallet.activeAddress()
	if !ok {
		return nil, errLocked
	}
	client := s.node.currentClient()
	if client == nil {
		return nil, errors.New("not connected")
	}
	out := []ProjectDTO{}
	var pageIndex uint32 = 0
	const pageSize uint32 = 50
	seen := 0
	for {
		list, err := client.AcceleratorApi.GetAll(pageIndex, pageSize)
		if err != nil {
			return nil, err
		}
		out = append(out, myActiveProjects(list.List, addr)...)
		seen += len(list.List)
		if seen >= list.Count || len(list.List) == 0 {
			break
		}
		pageIndex++
	}
	return out, nil
}
