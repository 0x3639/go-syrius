package governance

import (
	"bytes"
	"errors"
	"reflect"
	"testing"

	"github.com/0x3639/znn-sdk-go/transport"
	"github.com/zenon-network/go-zenon/common/types"
	"github.com/zenon-network/go-zenon/vm/embedded/definition"
)

type recordingCaller struct {
	method string
	args   []interface{}
	err    error
}

func (c *recordingCaller) Call(result interface{}, method string, args ...interface{}) error {
	c.method = method
	c.args = append([]interface{}(nil), args...)
	if c.err != nil {
		return c.err
	}
	switch out := result.(type) {
	case *ActionList:
		out.Count = 1
	case *Action:
		out.Name = "upgrade"
	}
	return nil
}

func TestGovernanceReadsUseCanonicalRPCMethods(t *testing.T) {
	caller := new(recordingCaller)
	api := NewAPI(caller)

	list, err := api.GetAllActions(2, 50)
	if err != nil || list.Count != 1 {
		t.Fatalf("GetAllActions() = (%+v, %v)", list, err)
	}
	if caller.method != "embedded.governance.getAllActions" || !reflect.DeepEqual(caller.args, []interface{}{uint32(2), uint32(50)}) {
		t.Fatalf("GetAllActions call = %s %#v", caller.method, caller.args)
	}

	id := types.HexToHashPanic("0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20")
	action, err := api.GetActionById(id)
	if err != nil || action.Name != "upgrade" {
		t.Fatalf("GetActionById() = (%+v, %v)", action, err)
	}
	if caller.method != "embedded.governance.getActionById" || !reflect.DeepEqual(caller.args, []interface{}{id.String()}) {
		t.Fatalf("GetActionById call = %s %#v", caller.method, caller.args)
	}
}

func TestGovernanceReadErrorsAreNormalized(t *testing.T) {
	api := NewAPI(nil)
	_, err := api.GetAllActions(0, 50)
	var rpcErr *transport.RPCError
	if !errors.As(err, &rpcErr) || rpcErr.Method != "embedded.governance.getAllActions" {
		t.Fatalf("error = %#v, want normalized governance RPC error", err)
	}
}

func TestGovernanceTemplatesUseCanonicalABI(t *testing.T) {
	api := NewAPI(nil)
	id := types.HexToHashPanic("0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20")

	proposal := api.ProposeAction("name", "description", "https://zenon.org", types.SporkContract, "AAEC")
	if proposal.ToAddress != types.GovernanceContract || proposal.TokenStandard != types.ZnnTokenStandard || proposal.Amount.Sign() <= 0 {
		t.Fatalf("proposal template = %+v", proposal)
	}
	wantProposalData := definition.ABIGovernance.PackMethodPanic(definition.ProposeActionMethodName, "name", "description", "https://zenon.org", types.SporkContract, "AAEC")
	if !bytes.Equal(proposal.Data, wantProposalData) {
		t.Fatal("proposal does not use the canonical governance ABI")
	}

	vote := api.VoteByName(id, "Pillar", definition.VoteYes)
	if vote.ToAddress != types.GovernanceContract || vote.Amount.Sign() != 0 {
		t.Fatalf("vote template = %+v", vote)
	}
	wantVoteData := definition.ABIGovernance.PackMethodPanic(definition.VoteByNameMethodName, id, "Pillar", uint8(definition.VoteYes))
	if !bytes.Equal(vote.Data, wantVoteData) {
		t.Fatal("vote does not use the canonical governance ABI")
	}

	execute := api.ExecuteAction(id)
	if execute.ToAddress != types.GovernanceContract || execute.Amount.Sign() != 0 {
		t.Fatalf("execute template = %+v", execute)
	}
	wantExecuteData := definition.ABIGovernance.PackMethodPanic(definition.ExecuteActionMethodName, id)
	if !bytes.Equal(execute.Data, wantExecuteData) {
		t.Fatal("execute does not use the canonical governance ABI")
	}
}
