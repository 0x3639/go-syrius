package app

import (
	"errors"
	"fmt"

	embedded "github.com/0x3639/znn-sdk-go/api/embedded"
)

// actionDTO maps an SDK governance Action (flat client struct: the on-chain
// fields plus the node's computed current-round fields) to the wire DTO.
// Nil-safe. Reuses voteBreakdownDTO from nom_accelerator.go.
func actionDTO(a *embedded.Action) ActionDTO {
	if a == nil {
		return ActionDTO{}
	}
	return ActionDTO{
		Id:                    a.Id.String(),
		Owner:                 a.Owner.String(),
		Name:                  a.Name,
		Description:           a.Description,
		Url:                   a.Url,
		Destination:           a.Destination.String(),
		Data:                  a.Data,
		Type:                  int(a.Type),
		Round:                 int(a.Round),
		Status:                int(a.Status),
		Executed:              a.Executed,
		Expired:               a.Expired,
		CreationTimestamp:     a.CreationTimestamp,
		RoundStartTimestamp:   a.RoundStartTimestamp,
		ActivePillarThreshold: a.ActivePillarThreshold,
		DirectionalThreshold:  a.DirectionalThreshold,
		VotingPeriod:          a.VotingPeriod,
		Votes:                 voteBreakdownDTO(a.Votes),
	}
}

// GetActions returns one page of governance actions (node ordering). pageSize is
// clamped to [1,50]. This is also the source the frontend Vote view filters to
// open actions in Phase 1.
func (s *NomService) GetActions(pageIndex, pageSize uint32) (ActionListDTO, error) {
	if pageSize == 0 || pageSize > 50 {
		pageSize = 50
	}
	client := s.node.currentClient()
	if client == nil {
		return ActionListDTO{}, errors.New("not connected")
	}
	list, err := client.GovernanceApi.GetAllActions(pageIndex, pageSize)
	if err != nil {
		return ActionListDTO{}, err
	}
	out := ActionListDTO{Count: list.Count, List: make([]ActionDTO, 0, len(list.List))}
	for _, a := range list.List {
		out.List = append(out.List, actionDTO(a))
	}
	return out, nil
}

// GetAction returns a single governance action by id.
func (s *NomService) GetAction(id string) (ActionDTO, error) {
	h, err := parseHash(id)
	if err != nil {
		return ActionDTO{}, fmt.Errorf("invalid action id: %w", err)
	}
	client := s.node.currentClient()
	if client == nil {
		return ActionDTO{}, errors.New("not connected")
	}
	a, err := client.GovernanceApi.GetActionById(h)
	if err != nil {
		return ActionDTO{}, err
	}
	return actionDTO(a), nil
}
