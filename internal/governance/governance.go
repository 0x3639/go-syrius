// Package governance preserves the testnet governance extension that syrius
// consumed from a pre-v0.2 SDK snapshot. Governance is not part of the stable
// SDK surface, so the wallet owns this small adapter while using the SDK's
// public transport and embedded-contract APIs.
package governance

import (
	"encoding/base64"

	embedded "github.com/0x3639/znn-sdk-go/api/embedded"
	"github.com/0x3639/znn-sdk-go/transport"
	"github.com/zenon-network/go-zenon/chain/nom"
	"github.com/zenon-network/go-zenon/common"
	"github.com/zenon-network/go-zenon/common/types"
	"github.com/zenon-network/go-zenon/vm/constants"
	"github.com/zenon-network/go-zenon/vm/embedded/definition"
)

// API exposes the testnet governance RPC reads and transaction templates.
type API struct {
	caller transport.Caller
}

// NewAPI constructs a governance adapter. A nil caller is valid for callers
// that only need the pure transaction-template and payload helpers.
func NewAPI(caller transport.Caller) *API {
	return &API{caller: transport.NewNormalizingCaller(caller)}
}

// Action is the wire model returned by embedded.governance RPC methods.
type Action struct {
	Id                    types.Hash              `json:"Id"`
	Owner                 types.Address           `json:"Owner"`
	Name                  string                  `json:"Name"`
	Description           string                  `json:"Description"`
	Url                   string                  `json:"Url"`
	Destination           types.Address           `json:"Destination"`
	Data                  string                  `json:"Data"`
	CreationTimestamp     int64                   `json:"CreationTimestamp"`
	Type                  uint8                   `json:"Type"`
	Round                 uint8                   `json:"Round"`
	CurrentVoteId         types.Hash              `json:"CurrentVoteId"`
	RoundStartTimestamp   int64                   `json:"RoundStartTimestamp"`
	Status                uint8                   `json:"Status"`
	Executed              bool                    `json:"Executed"`
	Expired               bool                    `json:"Expired"`
	ActivePillarThreshold uint32                  `json:"ActivePillarThreshold"`
	DirectionalThreshold  uint32                  `json:"DirectionalThreshold"`
	VotingPeriod          int64                   `json:"VotingPeriod"`
	Votes                 *embedded.VoteBreakdown `json:"Votes"`
}

// DecodedData returns the action's standard-base64 ABI payload.
func (a *Action) DecodedData() ([]byte, error) {
	return base64.StdEncoding.DecodeString(a.Data)
}

// ActionList is a paginated governance response.
type ActionList struct {
	Count int       `json:"count"`
	List  []*Action `json:"list"`
}

// GetAllActions fetches one page of governance actions.
func (g *API) GetAllActions(pageIndex, pageSize uint32) (*ActionList, error) {
	ans := new(ActionList)
	if err := g.caller.Call(ans, "embedded.governance.getAllActions", pageIndex, pageSize); err != nil {
		return nil, err
	}
	return ans, nil
}

// GetActionById fetches a governance action by its proposal-block hash.
func (g *API) GetActionById(id types.Hash) (*Action, error) {
	ans := new(Action)
	if err := g.caller.Call(ans, "embedded.governance.getActionById", id.String()); err != nil {
		return nil, err
	}
	return ans, nil
}

// ProposeAction builds an unsigned governance proposal template.
func (g *API) ProposeAction(name, description, url string, destination types.Address, data string) *nom.AccountBlock {
	return &nom.AccountBlock{
		BlockType:     nom.BlockTypeUserSend,
		ToAddress:     types.GovernanceContract,
		TokenStandard: types.ZnnTokenStandard,
		Amount:        constants.ProjectCreationAmount,
		Data: definition.ABIGovernance.PackMethodPanic(
			definition.ProposeActionMethodName,
			name,
			description,
			url,
			destination,
			data,
		),
	}
}

// ExecuteAction builds an unsigned governance execute/advance template.
func (g *API) ExecuteAction(id types.Hash) *nom.AccountBlock {
	return &nom.AccountBlock{
		BlockType:     nom.BlockTypeUserSend,
		ToAddress:     types.GovernanceContract,
		TokenStandard: types.ZnnTokenStandard,
		Amount:        common.Big0,
		Data:          definition.ABIGovernance.PackMethodPanic(definition.ExecuteActionMethodName, id),
	}
}

// VoteByName builds an unsigned pillar-name governance vote template.
func (g *API) VoteByName(id types.Hash, pillarName string, vote uint8) *nom.AccountBlock {
	return &nom.AccountBlock{
		BlockType:     nom.BlockTypeUserSend,
		ToAddress:     types.GovernanceContract,
		TokenStandard: types.ZnnTokenStandard,
		Amount:        common.Big0,
		Data: definition.ABIGovernance.PackMethodPanic(
			definition.VoteByNameMethodName,
			id,
			pillarName,
			vote,
		),
	}
}
