package governance

import (
	"encoding/base64"
	"math/big"

	embedded "github.com/0x3639/znn-sdk-go/api/embedded"
	"github.com/zenon-network/go-zenon/chain/nom"
	"github.com/zenon-network/go-zenon/common"
	"github.com/zenon-network/go-zenon/common/types"
	"github.com/zenon-network/go-zenon/vm/embedded/definition"
)

// ProposalPayload is the destination and standard-base64 ABI data wrapped by
// a governance proposal.
type ProposalPayload struct {
	Destination types.Address
	Data        string
}

// EncodeProposalPayload converts an embedded-contract template to a proposal.
func EncodeProposalPayload(block *nom.AccountBlock) ProposalPayload {
	return ProposalPayload{
		Destination: block.ToAddress,
		Data:        base64.StdEncoding.EncodeToString(block.Data),
	}
}

func (g *API) PayloadSporkCreate(name, description string) ProposalPayload {
	return EncodeProposalPayload(embedded.NewSporkApi(nil).CreateSpork(name, description))
}

func (g *API) PayloadSporkActivate(id types.Hash) ProposalPayload {
	return EncodeProposalPayload(embedded.NewSporkApi(nil).ActivateSpork(id))
}

func (g *API) PayloadBridgeAddNetwork(networkClass, chainID uint32, name, contractAddress, metadata string) ProposalPayload {
	return EncodeProposalPayload(embedded.NewBridgeApi(nil).AddNetwork(networkClass, chainID, name, contractAddress, metadata))
}

func (g *API) PayloadBridgeRemoveNetwork(networkClass, chainID uint32) ProposalPayload {
	return EncodeProposalPayload(embedded.NewBridgeApi(nil).RemoveNetwork(networkClass, chainID))
}

func (g *API) PayloadBridgeSetTokenPair(networkClass, chainID uint32, tokenStandard types.ZenonTokenStandard, tokenAddress string, bridgeable, redeemable, owned bool, minAmount *big.Int, fee, redeemDelay uint32, metadata string) ProposalPayload {
	return EncodeProposalPayload(embedded.NewBridgeApi(nil).SetTokenPair(networkClass, chainID, tokenStandard, tokenAddress, bridgeable, redeemable, owned, minAmount, fee, redeemDelay, metadata))
}

func (g *API) PayloadBridgeRemoveTokenPair(networkClass, chainID uint32, tokenStandard types.ZenonTokenStandard, tokenAddress string) ProposalPayload {
	return EncodeProposalPayload(embedded.NewBridgeApi(nil).RemoveTokenPair(networkClass, chainID, tokenStandard, tokenAddress))
}

func (g *API) PayloadBridgeHalt(signature string) ProposalPayload {
	return EncodeProposalPayload(embedded.NewBridgeApi(nil).Halt(signature))
}

func (g *API) PayloadBridgeUnhalt() ProposalPayload {
	return EncodeProposalPayload(embedded.NewBridgeApi(nil).Unhalt())
}

func (g *API) PayloadBridgeEmergency() ProposalPayload {
	return EncodeProposalPayload(embedded.NewBridgeApi(nil).Emergency())
}

func (g *API) PayloadBridgeChangeAdministrator(administrator types.Address) ProposalPayload {
	return EncodeProposalPayload(embedded.NewBridgeApi(nil).ChangeAdministrator(administrator))
}

func (g *API) PayloadBridgeChangeTssECDSAPubKey(pubKey, signature, newSignature string) ProposalPayload {
	return EncodeProposalPayload(embedded.NewBridgeApi(nil).ChangeTssECDSAPubKey(pubKey, signature, newSignature))
}

func (g *API) PayloadBridgeSetAllowKeygen(allowKeygen bool) ProposalPayload {
	return EncodeProposalPayload(embedded.NewBridgeApi(nil).SetAllowKeygen(allowKeygen))
}

func (g *API) PayloadBridgeSetOrchestratorInfo(windowSize uint64, keyGenThreshold, confirmationsToFinality, estimatedMomentumTime uint32) ProposalPayload {
	return EncodeProposalPayload(embedded.NewBridgeApi(nil).SetOrchestratorInfo(windowSize, keyGenThreshold, confirmationsToFinality, estimatedMomentumTime))
}

func (g *API) PayloadBridgeSetMetadata(metadata string) ProposalPayload {
	return EncodeProposalPayload(embedded.NewBridgeApi(nil).SetBridgeMetadata(metadata))
}

func (g *API) PayloadBridgeSetNetworkMetadata(networkClass, chainID uint32, metadata string) ProposalPayload {
	return EncodeProposalPayload(embedded.NewBridgeApi(nil).SetNetworkMetadata(networkClass, chainID, metadata))
}

func (g *API) PayloadBridgeRevokeUnwrapRequest(transactionHash types.Hash, logIndex uint32) ProposalPayload {
	return EncodeProposalPayload(embedded.NewBridgeApi(nil).RevokeUnwrapRequest(transactionHash, logIndex))
}

func (g *API) PayloadBridgeNominateGuardians(guardians []types.Address) ProposalPayload {
	return EncodeProposalPayload(embedded.NewBridgeApi(nil).NominateGuardians(guardians))
}

func (g *API) PayloadLiquidityFund(znnReward, qsrReward *big.Int) ProposalPayload {
	return EncodeProposalPayload(&nom.AccountBlock{
		BlockType:     nom.BlockTypeUserSend,
		ToAddress:     types.LiquidityContract,
		TokenStandard: types.ZnnTokenStandard,
		Amount:        common.Big0,
		Data:          definition.ABILiquidity.PackMethodPanic(definition.FundMethodName, znnReward, qsrReward),
	})
}

func (g *API) PayloadLiquidityBurnZnn(burnAmount *big.Int) ProposalPayload {
	return EncodeProposalPayload(&nom.AccountBlock{
		BlockType:     nom.BlockTypeUserSend,
		ToAddress:     types.LiquidityContract,
		TokenStandard: types.ZnnTokenStandard,
		Amount:        common.Big0,
		Data:          definition.ABILiquidity.PackMethodPanic(definition.BurnZnnMethodName, burnAmount),
	})
}

func (g *API) PayloadLiquiditySetTokenTuple(tokenStandards []string, znnPercentages, qsrPercentages []uint32, minAmounts []*big.Int) ProposalPayload {
	return EncodeProposalPayload(embedded.NewLiquidityApi(nil).SetTokenTupleMethod(tokenStandards, znnPercentages, qsrPercentages, minAmounts))
}

func (g *API) PayloadLiquiditySetIsHalted(value bool) ProposalPayload {
	return EncodeProposalPayload(embedded.NewLiquidityApi(nil).SetIsHalted(value))
}

func (g *API) PayloadLiquidityUnlockStakeEntries(zts types.ZenonTokenStandard) ProposalPayload {
	return EncodeProposalPayload(embedded.NewLiquidityApi(nil).UnlockLiquidityStakeEntries(zts))
}

func (g *API) PayloadLiquiditySetAdditionalReward(znnReward, qsrAmount *big.Int) ProposalPayload {
	return EncodeProposalPayload(embedded.NewLiquidityApi(nil).SetAdditionalReward(znnReward, qsrAmount))
}

func (g *API) PayloadLiquidityChangeAdministrator(administrator types.Address) ProposalPayload {
	return EncodeProposalPayload(embedded.NewLiquidityApi(nil).ChangeAdministrator(administrator))
}

func (g *API) PayloadLiquidityNominateGuardians(guardians []types.Address) ProposalPayload {
	return EncodeProposalPayload(embedded.NewLiquidityApi(nil).NominateGuardians(guardians))
}

func (g *API) PayloadLiquidityEmergency() ProposalPayload {
	return EncodeProposalPayload(embedded.NewLiquidityApi(nil).Emergency())
}
