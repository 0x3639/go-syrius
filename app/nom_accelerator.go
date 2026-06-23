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
)

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
