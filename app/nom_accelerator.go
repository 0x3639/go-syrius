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
