package app

import (
	"encoding/base64"
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"strings"

	embedded "github.com/0x3639/znn-sdk-go/api/embedded"
	rpc_client "github.com/0x3639/znn-sdk-go/rpc_client"
	"github.com/zenon-network/go-zenon/common/types"
)

// ---- parsing toolkit (shared by all kinds) ----

func reqParam(p map[string]string, key string) (string, error) {
	v := strings.TrimSpace(p[key])
	if v == "" {
		return "", fmt.Errorf("%s is required", key)
	}
	return v, nil
}
func optParam(p map[string]string, key string) string { return strings.TrimSpace(p[key]) }

func parseU32Param(p map[string]string, key string) (uint32, error) {
	s, err := reqParam(p, key)
	if err != nil {
		return 0, err
	}
	n, err := strconv.ParseUint(s, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("%s must be a non-negative whole number", key)
	}
	return uint32(n), nil
}
func parseU64Param(p map[string]string, key string) (uint64, error) {
	s, err := reqParam(p, key)
	if err != nil {
		return 0, err
	}
	n, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%s must be a non-negative whole number", key)
	}
	return n, nil
}
func parseBoolParam(p map[string]string, key string) (bool, error) {
	s, err := reqParam(p, key)
	if err != nil {
		return false, err
	}
	b, err := strconv.ParseBool(s)
	if err != nil {
		return false, fmt.Errorf("%s must be true or false", key)
	}
	return b, nil
}
func parseBigIntParam(p map[string]string, key string) (*big.Int, error) {
	s, err := reqParam(p, key)
	if err != nil {
		return nil, err
	}
	n, ok := new(big.Int).SetString(s, 10)
	if !ok || n.Sign() < 0 {
		return nil, fmt.Errorf("%s must be a non-negative integer amount", key)
	}
	return n, nil
}
func parseAddrParam(p map[string]string, key string) (types.Address, error) {
	s, err := reqParam(p, key)
	if err != nil {
		return types.Address{}, err
	}
	a, err := types.ParseAddress(s)
	if err != nil {
		return types.Address{}, fmt.Errorf("%s is not a valid address", key)
	}
	return a, nil
}
func parseHashParam(p map[string]string, key string) (types.Hash, error) {
	s, err := reqParam(p, key)
	if err != nil {
		return types.Hash{}, err
	}
	h, err := types.HexToHash(s)
	if err != nil {
		return types.Hash{}, fmt.Errorf("%s is not a valid hash", key)
	}
	return h, nil
}
func parseZtsParam(p map[string]string, key string) (types.ZenonTokenStandard, error) {
	s, err := reqParam(p, key)
	if err != nil {
		return types.ZenonTokenStandard{}, err
	}
	z, err := types.ParseZTS(s)
	if err != nil {
		return types.ZenonTokenStandard{}, fmt.Errorf("%s is not a valid token standard", key)
	}
	return z, nil
}
func splitList(p map[string]string, key string) ([]string, error) {
	s, err := reqParam(p, key)
	if err != nil {
		return nil, err
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, x := range parts {
		if t := strings.TrimSpace(x); t != "" {
			out = append(out, t)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("%s must have at least one value", key)
	}
	return out, nil
}
func parseStrList(p map[string]string, key string) ([]string, error) { return splitList(p, key) }
func parseAddrList(p map[string]string, key string) ([]types.Address, error) {
	items, err := splitList(p, key)
	if err != nil {
		return nil, err
	}
	out := make([]types.Address, 0, len(items))
	for _, s := range items {
		a, err := types.ParseAddress(s)
		if err != nil {
			return nil, fmt.Errorf("%s contains an invalid address: %s", key, s)
		}
		out = append(out, a)
	}
	return out, nil
}
func parseU32List(p map[string]string, key string) ([]uint32, error) {
	items, err := splitList(p, key)
	if err != nil {
		return nil, err
	}
	out := make([]uint32, 0, len(items))
	for _, s := range items {
		n, err := strconv.ParseUint(s, 10, 32)
		if err != nil {
			return nil, fmt.Errorf("%s contains an invalid number: %s", key, s)
		}
		out = append(out, uint32(n))
	}
	return out, nil
}
func parseBigIntList(p map[string]string, key string) ([]*big.Int, error) {
	items, err := splitList(p, key)
	if err != nil {
		return nil, err
	}
	out := make([]*big.Int, 0, len(items))
	for _, s := range items {
		n, ok := new(big.Int).SetString(s, 10)
		if !ok || n.Sign() < 0 {
			return nil, fmt.Errorf("%s contains an invalid amount: %s", key, s)
		}
		out = append(out, n)
	}
	return out, nil
}

// ---- catalog (single source of truth for the form) ----

func proposeKinds() []ProposeKindDTO {
	return []ProposeKindDTO{
		{Kind: "spork.create", Label: "Spork — Create", Group: "Spork", Fields: []ProposeFieldDTO{
			{Key: "name", Label: "Spork name", Type: "text", Placeholder: "my-spork", Required: true, Min: 5, Max: 40},
			{Key: "description", Label: "Spork description", Type: "text", Placeholder: "What this spork gates", Required: true, Max: 400},
		}},
		{Kind: "spork.activate", Label: "Spork — Activate", Group: "Spork", Fields: []ProposeFieldDTO{
			{Key: "id", Label: "Spork id (hash)", Type: "hash", Placeholder: "0x…", Required: true},
		}},
		{Kind: "bridge.addNetwork", Label: "Bridge — Add Network", Group: "Bridge", Fields: []ProposeFieldDTO{
			{Key: "networkClass", Label: "Network class", Type: "number", Placeholder: "1", Required: true},
			{Key: "chainId", Label: "Chain id", Type: "number", Placeholder: "1", Required: true},
			{Key: "name", Label: "Name", Type: "text", Placeholder: "Ethereum", Required: true},
			{Key: "contractAddress", Label: "Contract address", Type: "text", Placeholder: "0x…", Required: true},
			{Key: "metadata", Label: "Metadata (JSON)", Type: "text", Placeholder: "{}", Required: false},
		}},
		{Kind: "bridge.removeNetwork", Label: "Bridge — Remove Network", Group: "Bridge", Fields: []ProposeFieldDTO{
			{Key: "networkClass", Label: "Network class", Type: "number", Placeholder: "1", Required: true},
			{Key: "chainId", Label: "Chain id", Type: "number", Placeholder: "1", Required: true},
		}},
		{Kind: "bridge.setTokenPair", Label: "Bridge — Set Token Pair", Group: "Bridge", Fields: []ProposeFieldDTO{
			{Key: "networkClass", Label: "Network class", Type: "number", Placeholder: "1", Required: true},
			{Key: "chainId", Label: "Chain id", Type: "number", Placeholder: "1", Required: true},
			{Key: "tokenStandard", Label: "Token standard (ZTS)", Type: "text", Placeholder: "zts1…", Required: true},
			{Key: "tokenAddress", Label: "Foreign token address", Type: "text", Placeholder: "0x…", Required: true},
			{Key: "bridgeable", Label: "Bridgeable", Type: "bool", Placeholder: "", Required: true},
			{Key: "redeemable", Label: "Redeemable", Type: "bool", Placeholder: "", Required: true},
			{Key: "owned", Label: "Owned", Type: "bool", Placeholder: "", Required: true},
			{Key: "minAmount", Label: "Min amount", Type: "amount", Placeholder: "0", Required: true},
			{Key: "fee", Label: "Fee (per-ten-thousand)", Type: "number", Placeholder: "0", Required: true},
			{Key: "redeemDelay", Label: "Redeem delay (momentums)", Type: "number", Placeholder: "0", Required: true},
			{Key: "metadata", Label: "Metadata (JSON)", Type: "text", Placeholder: "{}", Required: false},
		}},
		{Kind: "bridge.removeTokenPair", Label: "Bridge — Remove Token Pair", Group: "Bridge", Fields: []ProposeFieldDTO{
			{Key: "networkClass", Label: "Network class", Type: "number", Placeholder: "1", Required: true},
			{Key: "chainId", Label: "Chain id", Type: "number", Placeholder: "1", Required: true},
			{Key: "tokenStandard", Label: "Token standard (ZTS)", Type: "text", Placeholder: "zts1…", Required: true},
			{Key: "tokenAddress", Label: "Foreign token address", Type: "text", Placeholder: "0x…", Required: true},
		}},
		{Kind: "bridge.halt", Label: "Bridge — Halt", Group: "Bridge", Fields: []ProposeFieldDTO{
			{Key: "signature", Label: "Signature", Type: "text", Placeholder: "", Required: true},
		}},
		{Kind: "bridge.unhalt", Label: "Bridge — Unhalt", Group: "Bridge", Fields: []ProposeFieldDTO{}},
		{Kind: "bridge.emergency", Label: "Bridge — Emergency", Group: "Bridge", Fields: []ProposeFieldDTO{}},
		{Kind: "bridge.changeAdministrator", Label: "Bridge — Change Administrator", Group: "Bridge", Fields: []ProposeFieldDTO{
			{Key: "administrator", Label: "New administrator", Type: "address", Placeholder: "z1…", Required: true},
		}},
		{Kind: "bridge.changeTssECDSAPubKey", Label: "Bridge — Change TSS Pubkey", Group: "Bridge", Fields: []ProposeFieldDTO{
			{Key: "pubKey", Label: "New TSS pubkey", Type: "text", Placeholder: "", Required: true},
			{Key: "signature", Label: "Old-key signature", Type: "text", Placeholder: "", Required: true},
			{Key: "newSignature", Label: "New-key signature", Type: "text", Placeholder: "", Required: true},
		}},
		{Kind: "bridge.setAllowKeygen", Label: "Bridge — Set Allow Keygen", Group: "Bridge", Fields: []ProposeFieldDTO{
			{Key: "allowKeygen", Label: "Allow keygen", Type: "bool", Placeholder: "", Required: true},
		}},
		{Kind: "bridge.setOrchestratorInfo", Label: "Bridge — Set Orchestrator Info", Group: "Bridge", Fields: []ProposeFieldDTO{
			{Key: "windowSize", Label: "Window size", Type: "number", Placeholder: "", Required: true},
			{Key: "keyGenThreshold", Label: "Keygen threshold", Type: "number", Placeholder: "", Required: true},
			{Key: "confirmationsToFinality", Label: "Confirmations to finality", Type: "number", Placeholder: "", Required: true},
			{Key: "estimatedMomentumTime", Label: "Estimated momentum time", Type: "number", Placeholder: "", Required: true},
		}},
		{Kind: "bridge.setMetadata", Label: "Bridge — Set Metadata", Group: "Bridge", Fields: []ProposeFieldDTO{
			{Key: "metadata", Label: "Metadata (JSON)", Type: "text", Placeholder: "{}", Required: true},
		}},
		{Kind: "bridge.setNetworkMetadata", Label: "Bridge — Set Network Metadata", Group: "Bridge", Fields: []ProposeFieldDTO{
			{Key: "networkClass", Label: "Network class", Type: "number", Placeholder: "1", Required: true},
			{Key: "chainId", Label: "Chain id", Type: "number", Placeholder: "1", Required: true},
			{Key: "metadata", Label: "Metadata (JSON)", Type: "text", Placeholder: "{}", Required: true},
		}},
		{Kind: "bridge.revokeUnwrapRequest", Label: "Bridge — Revoke Unwrap Request", Group: "Bridge", Fields: []ProposeFieldDTO{
			{Key: "transactionHash", Label: "Transaction hash", Type: "hash", Placeholder: "0x…", Required: true},
			{Key: "logIndex", Label: "Log index", Type: "number", Placeholder: "0", Required: true},
		}},
		{Kind: "bridge.nominateGuardians", Label: "Bridge — Nominate Guardians", Group: "Bridge", Fields: []ProposeFieldDTO{
			{Key: "guardians", Label: "Guardian addresses", Type: "list", Placeholder: "z1…,z1…", Required: true},
		}},
		{Kind: "liquidity.fund", Label: "Liquidity — Fund", Group: "Liquidity", Fields: []ProposeFieldDTO{
			{Key: "znnReward", Label: "ZNN reward", Type: "amount", Placeholder: "0", Required: true},
			{Key: "qsrReward", Label: "QSR reward", Type: "amount", Placeholder: "0", Required: true},
		}},
		{Kind: "liquidity.burnZnn", Label: "Liquidity — Burn ZNN", Group: "Liquidity", Fields: []ProposeFieldDTO{
			{Key: "burnAmount", Label: "Burn amount", Type: "amount", Placeholder: "0", Required: true},
		}},
		{Kind: "liquidity.setTokenTuple", Label: "Liquidity — Set Token Tuple", Group: "Liquidity", Fields: []ProposeFieldDTO{
			{Key: "tokenStandards", Label: "Token standards", Type: "list", Placeholder: "zts1…,zts1…", Required: true},
			{Key: "znnPercentages", Label: "ZNN percentages", Type: "list", Placeholder: "5000,5000", Required: true},
			{Key: "qsrPercentages", Label: "QSR percentages", Type: "list", Placeholder: "5000,5000", Required: true},
			{Key: "minAmounts", Label: "Min amounts", Type: "list", Placeholder: "100,100", Required: true},
		}},
		{Kind: "liquidity.setIsHalted", Label: "Liquidity — Set Halted", Group: "Liquidity", Fields: []ProposeFieldDTO{
			{Key: "value", Label: "Halted", Type: "bool", Placeholder: "", Required: true},
		}},
		{Kind: "liquidity.unlockStakeEntries", Label: "Liquidity — Unlock Stake Entries", Group: "Liquidity", Fields: []ProposeFieldDTO{
			{Key: "zts", Label: "Token standard (ZTS)", Type: "text", Placeholder: "zts1…", Required: true},
		}},
		{Kind: "liquidity.setAdditionalReward", Label: "Liquidity — Set Additional Reward", Group: "Liquidity", Fields: []ProposeFieldDTO{
			{Key: "znnReward", Label: "ZNN reward", Type: "amount", Placeholder: "0", Required: true},
			{Key: "qsrAmount", Label: "QSR amount", Type: "amount", Placeholder: "0", Required: true},
		}},
		{Kind: "liquidity.changeAdministrator", Label: "Liquidity — Change Administrator", Group: "Liquidity", Fields: []ProposeFieldDTO{
			{Key: "administrator", Label: "New administrator", Type: "address", Placeholder: "z1…", Required: true},
		}},
		{Kind: "liquidity.nominateGuardians", Label: "Liquidity — Nominate Guardians", Group: "Liquidity", Fields: []ProposeFieldDTO{
			{Key: "guardians", Label: "Guardian addresses", Type: "list", Placeholder: "z1…,z1…", Required: true},
		}},
		{Kind: "liquidity.emergency", Label: "Liquidity — Emergency", Group: "Liquidity", Fields: []ProposeFieldDTO{}},
		{Kind: "custom", Label: "Custom (advanced)", Group: "Custom", Fields: []ProposeFieldDTO{
			{Key: "destination", Label: "Destination contract", Type: "address", Placeholder: "z1…", Required: true},
			{Key: "data", Label: "Call data (base64)", Type: "base64", Placeholder: "base64-encoded ABI call bytes", Required: true},
		}},
	}
}

// ---- dispatcher ----

// validateFieldLengths enforces the catalog's Min/Max byte-length bounds for the
// kind's fields before the per-kind builder runs, so an out-of-range value is
// rejected client-side instead of costing the 1 ZNN fee and failing on-chain.
// Empty values are skipped here (required-ness is enforced by the builder's
// reqParam); byte length matches the node's len() checks.
func validateFieldLengths(kind string, p map[string]string) error {
	for _, k := range proposeKinds() {
		if k.Kind != kind {
			continue
		}
		for _, f := range k.Fields {
			v := strings.TrimSpace(p[f.Key])
			if v == "" {
				continue
			}
			if f.Min > 0 && len(v) < f.Min {
				return fmt.Errorf("%s must be at least %d characters", f.Label, f.Min)
			}
			if f.Max > 0 && len(v) > f.Max {
				return fmt.Errorf("%s must be at most %d characters", f.Label, f.Max)
			}
		}
		break
	}
	return nil
}

func buildProposalPayloadWith(api *embedded.GovernanceApi, kind string, p map[string]string) (embedded.ProposalPayload, error) {
	if err := validateFieldLengths(kind, p); err != nil {
		return embedded.ProposalPayload{}, err
	}
	switch kind {
	case "spork.create":
		name, err := reqParam(p, "name")
		if err != nil {
			return embedded.ProposalPayload{}, err
		}
		desc, err := reqParam(p, "description")
		if err != nil {
			return embedded.ProposalPayload{}, err
		}
		return api.PayloadSporkCreate(name, desc), nil
	case "spork.activate":
		id, err := parseHashParam(p, "id")
		if err != nil {
			return embedded.ProposalPayload{}, err
		}
		return api.PayloadSporkActivate(id), nil
	case "custom":
		dest, err := parseAddrParam(p, "destination")
		if err != nil {
			return embedded.ProposalPayload{}, err
		}
		data, err := reqParam(p, "data")
		if err != nil {
			return embedded.ProposalPayload{}, err
		}
		if _, err := base64.StdEncoding.DecodeString(data); err != nil {
			return embedded.ProposalPayload{}, errors.New("data must be valid standard base64")
		}
		return embedded.ProposalPayload{Destination: dest, Data: data}, nil
	case "bridge.addNetwork":
		nc, err := parseU32Param(p, "networkClass")
		if err != nil {
			return embedded.ProposalPayload{}, err
		}
		cid, err := parseU32Param(p, "chainId")
		if err != nil {
			return embedded.ProposalPayload{}, err
		}
		name, err := reqParam(p, "name")
		if err != nil {
			return embedded.ProposalPayload{}, err
		}
		ca, err := reqParam(p, "contractAddress")
		if err != nil {
			return embedded.ProposalPayload{}, err
		}
		return api.PayloadBridgeAddNetwork(nc, cid, name, ca, optParam(p, "metadata")), nil
	case "bridge.removeNetwork":
		nc, err := parseU32Param(p, "networkClass")
		if err != nil {
			return embedded.ProposalPayload{}, err
		}
		cid, err := parseU32Param(p, "chainId")
		if err != nil {
			return embedded.ProposalPayload{}, err
		}
		return api.PayloadBridgeRemoveNetwork(nc, cid), nil
	case "bridge.setTokenPair":
		nc, err := parseU32Param(p, "networkClass")
		if err != nil {
			return embedded.ProposalPayload{}, err
		}
		cid, err := parseU32Param(p, "chainId")
		if err != nil {
			return embedded.ProposalPayload{}, err
		}
		zts, err := parseZtsParam(p, "tokenStandard")
		if err != nil {
			return embedded.ProposalPayload{}, err
		}
		ta, err := reqParam(p, "tokenAddress")
		if err != nil {
			return embedded.ProposalPayload{}, err
		}
		bridgeable, err := parseBoolParam(p, "bridgeable")
		if err != nil {
			return embedded.ProposalPayload{}, err
		}
		redeemable, err := parseBoolParam(p, "redeemable")
		if err != nil {
			return embedded.ProposalPayload{}, err
		}
		owned, err := parseBoolParam(p, "owned")
		if err != nil {
			return embedded.ProposalPayload{}, err
		}
		minAmt, err := parseBigIntParam(p, "minAmount")
		if err != nil {
			return embedded.ProposalPayload{}, err
		}
		fee, err := parseU32Param(p, "fee")
		if err != nil {
			return embedded.ProposalPayload{}, err
		}
		rd, err := parseU32Param(p, "redeemDelay")
		if err != nil {
			return embedded.ProposalPayload{}, err
		}
		return api.PayloadBridgeSetTokenPair(nc, cid, zts, ta, bridgeable, redeemable, owned, minAmt, fee, rd, optParam(p, "metadata")), nil
	case "bridge.removeTokenPair":
		nc, err := parseU32Param(p, "networkClass")
		if err != nil {
			return embedded.ProposalPayload{}, err
		}
		cid, err := parseU32Param(p, "chainId")
		if err != nil {
			return embedded.ProposalPayload{}, err
		}
		zts, err := parseZtsParam(p, "tokenStandard")
		if err != nil {
			return embedded.ProposalPayload{}, err
		}
		ta, err := reqParam(p, "tokenAddress")
		if err != nil {
			return embedded.ProposalPayload{}, err
		}
		return api.PayloadBridgeRemoveTokenPair(nc, cid, zts, ta), nil
	case "bridge.halt":
		sig, err := reqParam(p, "signature")
		if err != nil {
			return embedded.ProposalPayload{}, err
		}
		return api.PayloadBridgeHalt(sig), nil
	case "bridge.unhalt":
		return api.PayloadBridgeUnhalt(), nil
	case "bridge.emergency":
		return api.PayloadBridgeEmergency(), nil
	case "bridge.changeAdministrator":
		a, err := parseAddrParam(p, "administrator")
		if err != nil {
			return embedded.ProposalPayload{}, err
		}
		return api.PayloadBridgeChangeAdministrator(a), nil
	case "bridge.changeTssECDSAPubKey":
		pk, err := reqParam(p, "pubKey")
		if err != nil {
			return embedded.ProposalPayload{}, err
		}
		sig, err := reqParam(p, "signature")
		if err != nil {
			return embedded.ProposalPayload{}, err
		}
		ns, err := reqParam(p, "newSignature")
		if err != nil {
			return embedded.ProposalPayload{}, err
		}
		return api.PayloadBridgeChangeTssECDSAPubKey(pk, sig, ns), nil
	case "bridge.setAllowKeygen":
		b, err := parseBoolParam(p, "allowKeygen")
		if err != nil {
			return embedded.ProposalPayload{}, err
		}
		return api.PayloadBridgeSetAllowKeygen(b), nil
	case "bridge.setOrchestratorInfo":
		ws, err := parseU64Param(p, "windowSize")
		if err != nil {
			return embedded.ProposalPayload{}, err
		}
		kt, err := parseU32Param(p, "keyGenThreshold")
		if err != nil {
			return embedded.ProposalPayload{}, err
		}
		cf, err := parseU32Param(p, "confirmationsToFinality")
		if err != nil {
			return embedded.ProposalPayload{}, err
		}
		et, err := parseU32Param(p, "estimatedMomentumTime")
		if err != nil {
			return embedded.ProposalPayload{}, err
		}
		return api.PayloadBridgeSetOrchestratorInfo(ws, kt, cf, et), nil
	case "bridge.setMetadata":
		m, err := reqParam(p, "metadata")
		if err != nil {
			return embedded.ProposalPayload{}, err
		}
		return api.PayloadBridgeSetMetadata(m), nil
	case "bridge.setNetworkMetadata":
		nc, err := parseU32Param(p, "networkClass")
		if err != nil {
			return embedded.ProposalPayload{}, err
		}
		cid, err := parseU32Param(p, "chainId")
		if err != nil {
			return embedded.ProposalPayload{}, err
		}
		m, err := reqParam(p, "metadata")
		if err != nil {
			return embedded.ProposalPayload{}, err
		}
		return api.PayloadBridgeSetNetworkMetadata(nc, cid, m), nil
	case "bridge.revokeUnwrapRequest":
		h, err := parseHashParam(p, "transactionHash")
		if err != nil {
			return embedded.ProposalPayload{}, err
		}
		li, err := parseU32Param(p, "logIndex")
		if err != nil {
			return embedded.ProposalPayload{}, err
		}
		return api.PayloadBridgeRevokeUnwrapRequest(h, li), nil
	case "bridge.nominateGuardians":
		gs, err := parseAddrList(p, "guardians")
		if err != nil {
			return embedded.ProposalPayload{}, err
		}
		return api.PayloadBridgeNominateGuardians(gs), nil
	case "liquidity.fund":
		znn, err := parseBigIntParam(p, "znnReward")
		if err != nil {
			return embedded.ProposalPayload{}, err
		}
		qsr, err := parseBigIntParam(p, "qsrReward")
		if err != nil {
			return embedded.ProposalPayload{}, err
		}
		return api.PayloadLiquidityFund(znn, qsr), nil
	case "liquidity.burnZnn":
		amt, err := parseBigIntParam(p, "burnAmount")
		if err != nil {
			return embedded.ProposalPayload{}, err
		}
		return api.PayloadLiquidityBurnZnn(amt), nil
	case "liquidity.setTokenTuple":
		zs, err := parseStrList(p, "tokenStandards")
		if err != nil {
			return embedded.ProposalPayload{}, err
		}
		zp, err := parseU32List(p, "znnPercentages")
		if err != nil {
			return embedded.ProposalPayload{}, err
		}
		qp, err := parseU32List(p, "qsrPercentages")
		if err != nil {
			return embedded.ProposalPayload{}, err
		}
		ma, err := parseBigIntList(p, "minAmounts")
		if err != nil {
			return embedded.ProposalPayload{}, err
		}
		if len(zs) != len(zp) || len(zs) != len(qp) || len(zs) != len(ma) {
			return embedded.ProposalPayload{}, errors.New("setTokenTuple lists (tokenStandards, znnPercentages, qsrPercentages, minAmounts) must all have the same length")
		}
		return api.PayloadLiquiditySetTokenTuple(zs, zp, qp, ma), nil
	case "liquidity.setIsHalted":
		v, err := parseBoolParam(p, "value")
		if err != nil {
			return embedded.ProposalPayload{}, err
		}
		return api.PayloadLiquiditySetIsHalted(v), nil
	case "liquidity.unlockStakeEntries":
		z, err := parseZtsParam(p, "zts")
		if err != nil {
			return embedded.ProposalPayload{}, err
		}
		return api.PayloadLiquidityUnlockStakeEntries(z), nil
	case "liquidity.setAdditionalReward":
		znn, err := parseBigIntParam(p, "znnReward")
		if err != nil {
			return embedded.ProposalPayload{}, err
		}
		qsr, err := parseBigIntParam(p, "qsrAmount")
		if err != nil {
			return embedded.ProposalPayload{}, err
		}
		return api.PayloadLiquiditySetAdditionalReward(znn, qsr), nil
	case "liquidity.changeAdministrator":
		a, err := parseAddrParam(p, "administrator")
		if err != nil {
			return embedded.ProposalPayload{}, err
		}
		return api.PayloadLiquidityChangeAdministrator(a), nil
	case "liquidity.nominateGuardians":
		gs, err := parseAddrList(p, "guardians")
		if err != nil {
			return embedded.ProposalPayload{}, err
		}
		return api.PayloadLiquidityNominateGuardians(gs), nil
	case "liquidity.emergency":
		return api.PayloadLiquidityEmergency(), nil
	}
	return embedded.ProposalPayload{}, fmt.Errorf("unknown action kind %q", kind)
}

func buildProposalPayload(client *rpc_client.RpcClient, kind string, p map[string]string) (embedded.ProposalPayload, error) {
	return buildProposalPayloadWith(client.GovernanceApi, kind, p)
}

// ---- bound methods ----

// GetProposeKinds returns the static catalog of proposable action kinds + their
// input schema. No node I/O; safe before connection (the form renders from it).
func (s *NomService) GetProposeKinds() ([]ProposeKindDTO, error) {
	return proposeKinds(), nil
}

// PrepareProposeAction validates the metadata + per-kind params server-side,
// builds destination+data via the SDK Payload helper, and wraps ProposeAction
// (1 ZNN fee, read from the template). Confirm-what-you-sign via prepareCall.
func (s *NomService) PrepareProposeAction(name, description, url, kind string, params map[string]string) (CallPreview, error) {
	name = strings.TrimSpace(name)
	description = strings.TrimSpace(description)
	url = strings.TrimSpace(url)
	if name == "" {
		return CallPreview{}, errors.New("action name is required")
	}
	if description == "" {
		return CallPreview{}, errors.New("action description is required")
	}
	if url == "" || !acceleratorURLRe.MatchString(url) {
		return CallPreview{}, errors.New("invalid URL")
	}
	client := s.node.currentClient()
	if client == nil {
		return CallPreview{}, errors.New("not connected")
	}
	payload, err := buildProposalPayload(client, kind, params)
	if err != nil {
		return CallPreview{}, err
	}
	template := client.GovernanceApi.ProposeAction(name, description, url, payload.Destination, payload.Data)
	label := kind
	for _, k := range proposeKinds() {
		if k.Kind == kind {
			label = k.Label
			break
		}
	}
	return s.tx.prepareCall(template,
		callExpect{to: types.GovernanceContract, zts: types.ZnnTokenStandard, amount: template.Amount, data: append([]byte(nil), template.Data...)},
		fmt.Sprintf("Propose %q (1 ZNN) — %s calls %s", name, label, payload.Destination.String()))
}
