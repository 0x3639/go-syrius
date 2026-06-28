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
			{Key: "name", Label: "Spork name", Type: "text", Placeholder: "my-spork", Required: true},
			{Key: "description", Label: "Spork description", Type: "text", Placeholder: "What this spork gates", Required: true},
		}},
		{Kind: "spork.activate", Label: "Spork — Activate", Group: "Spork", Fields: []ProposeFieldDTO{
			{Key: "id", Label: "Spork id (hash)", Type: "hash", Placeholder: "0x…", Required: true},
		}},
		{Kind: "custom", Label: "Custom (advanced)", Group: "Custom", Fields: []ProposeFieldDTO{
			{Key: "destination", Label: "Destination contract", Type: "address", Placeholder: "z1…", Required: true},
			{Key: "data", Label: "Call data (base64)", Type: "base64", Placeholder: "base64-encoded ABI call bytes", Required: true},
		}},
	}
}

// ---- dispatcher ----

func buildProposalPayloadWith(api *embedded.GovernanceApi, kind string, p map[string]string) (embedded.ProposalPayload, error) {
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
