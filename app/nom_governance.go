package app

import (
	"errors"
	"fmt"
	"strings"

	embedded "github.com/0x3639/znn-sdk-go/api/embedded"
	"github.com/zenon-network/go-zenon/common/types"
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
		CurrentVoteId:         a.CurrentVoteId.String(),
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

// PrepareGovernanceVote builds a VoteByName template for one of the active
// address's Pillars. vote MUST be embedded.VoteYes/VoteNo/VoteAbstain. Field
// validation runs before any node use; Pillar ownership is enforced on-chain.
func (s *NomService) PrepareGovernanceVote(id, pillarName string, vote uint8) (CallPreview, error) {
	h, err := parseHash(id)
	if err != nil {
		return CallPreview{}, fmt.Errorf("invalid action id: %w", err)
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
	// Votes are keyed by the action's CURRENT-round votable hash (CurrentVoteId),
	// not the action id. They coincide only in round 0 (ActionVoteId(id,0)==id);
	// after the first ratchet the action id is no longer a registered votable
	// hash and a vote against it is rejected on-chain. Fetch the live action and
	// vote on its CurrentVoteId so the vote always targets the open round (also
	// avoids a stale id if the round advanced since the UI loaded the action).
	action, err := client.GovernanceApi.GetActionById(h)
	if err != nil {
		return CallPreview{}, err
	}
	if action.CurrentVoteId.IsZero() {
		return CallPreview{}, errors.New("action is not open for voting")
	}
	template := client.GovernanceApi.VoteByName(action.CurrentVoteId, name, vote)
	label := map[uint8]string{embedded.VoteYes: "yes", embedded.VoteNo: "no", embedded.VoteAbstain: "abstain"}[vote]
	return s.tx.prepareCall(template,
		callExpect{to: types.GovernanceContract, zts: types.ZnnTokenStandard, amount: template.Amount, data: append([]byte(nil), template.Data...)},
		fmt.Sprintf("Vote %s on governance action %q (round %d) as %s", label, action.Name, action.Round+1, name))
}

// PrepareExecuteAction builds an ExecuteAction template. It first fetches the
// action so the confirm summary names the action and the contract it will call
// (confirm-what-you-sign: the on-chain ToAddress is the governance contract, so
// the real effect — the destination call — is surfaced in the summary).
func (s *NomService) PrepareExecuteAction(id string) (CallPreview, error) {
	h, err := parseHash(id)
	if err != nil {
		return CallPreview{}, fmt.Errorf("invalid action id: %w", err)
	}
	client := s.node.currentClient()
	if client == nil {
		return CallPreview{}, errors.New("not connected")
	}
	a, err := client.GovernanceApi.GetActionById(h)
	if err != nil {
		return CallPreview{}, err
	}
	d := actionDTO(a)
	template := client.GovernanceApi.ExecuteAction(h)
	return s.tx.prepareCall(template,
		callExpect{to: types.GovernanceContract, zts: types.ZnnTokenStandard, amount: template.Amount, data: append([]byte(nil), template.Data...)},
		fmt.Sprintf("Execute governance action %q (calls %s)", d.Name, d.Destination))
}
