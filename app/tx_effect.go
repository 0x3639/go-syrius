package app

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"reflect"
	"strconv"
	"strings"

	"github.com/zenon-network/go-zenon/common/types"
	"github.com/zenon-network/go-zenon/vm/abi"
	definition "github.com/zenon-network/go-zenon/vm/embedded/definition"
)

// contractABIs maps every embedded-contract address to its name and ABI. These
// are the SAME definitions the SDK payload helpers encode with, so a decode
// here cannot drift from the encode the way a hand-maintained summary could.
var contractABIs = map[types.Address]struct {
	name string
	abi  *abi.ABIContract
}{
	types.PlasmaContract:      {"Plasma", &definition.ABIPlasma},
	types.PillarContract:      {"Pillar", &definition.ABIPillars},
	types.TokenContract:       {"Token", &definition.ABIToken},
	types.SentinelContract:    {"Sentinel", &definition.ABISentinel},
	types.SwapContract:        {"Swap", &definition.ABISwap},
	types.StakeContract:       {"Stake", &definition.ABIStake},
	types.SporkContract:       {"Spork", &definition.ABISpork},
	types.LiquidityContract:   {"Liquidity", &definition.ABILiquidity},
	types.AcceleratorContract: {"Accelerator", &definition.ABIAccelerator},
	types.HtlcContract:        {"Htlc", &definition.ABIHtlc},
	types.BridgeContract:      {"Bridge", &definition.ABIBridge},
	types.GovernanceContract:  {"Governance", &definition.ABIGovernance},
}

// decodeContractCall decodes ABI call data against the destination contract's
// definition into a structured, human-verifiable effect. It FAILS CLOSED: an
// unknown destination, an unknown method, undecodable arguments, or decoded
// values that do not re-encode to the exact input bytes all return an error —
// an incomplete friendly summary must never stand in for the real effect.
func decodeContractCall(destination types.Address, data []byte) (*TransactionEffect, error) {
	entry, ok := contractABIs[destination]
	if !ok {
		return nil, fmt.Errorf("destination %s is not a known embedded contract; refusing to summarize an undecodable call", destination)
	}
	if len(data) < 4 {
		return nil, errors.New("call data is too short to contain a method selector")
	}
	method, err := entry.abi.MethodById(data[:4])
	if err != nil {
		return nil, fmt.Errorf("unknown method on the %s contract: %w", entry.name, err)
	}
	values, err := method.Inputs.UnpackValues(data[4:])
	if err != nil {
		return nil, fmt.Errorf("cannot decode %s.%s arguments: %w", entry.name, method.Name, err)
	}
	// Round-trip check: the decoded values must re-encode to the exact input
	// bytes. Anything the fields do not fully describe (trailing bytes,
	// malleable encodings) is refused instead of partially rendered.
	repacked, err := entry.abi.PackMethod(method.Name, values...)
	if err != nil || !bytes.Equal(repacked, data) {
		return nil, fmt.Errorf("decoded %s.%s does not round-trip to the exact call data; refusing to summarize", entry.name, method.Name)
	}
	fields := make([]EffectField, 0, len(values))
	for i, v := range values {
		label := method.Inputs[i].Name
		if label == "" {
			label = fmt.Sprintf("arg%d", i)
		}
		fields = append(fields, EffectField{Label: label, Value: renderAbiValue(v)})
	}
	return &TransactionEffect{Contract: entry.name, Method: method.Name, Fields: fields}, nil
}

// renderAbiValue renders one decoded ABI value exactly and unambiguously:
// full addresses/hashes/token standards, full base-unit integers, explicit
// booleans, hex for raw bytes, and comma-joined elements for lists.
func renderAbiValue(v interface{}) string {
	switch x := v.(type) {
	case types.Address:
		return x.String()
	case types.ZenonTokenStandard:
		return x.String()
	case types.Hash:
		return x.String()
	case *big.Int:
		if x == nil {
			return "0"
		}
		return x.String()
	case bool:
		return strconv.FormatBool(x)
	case string:
		return x
	case []byte:
		if len(x) == 0 {
			return "(empty)"
		}
		return "0x" + hex.EncodeToString(x)
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Slice, reflect.Array:
		// Fixed-byte arrays render as hex; other lists element-by-element.
		if rv.Type().Elem().Kind() == reflect.Uint8 {
			b := make([]byte, rv.Len())
			reflect.Copy(reflect.ValueOf(b), rv)
			return "0x" + hex.EncodeToString(b)
		}
		parts := make([]string, 0, rv.Len())
		for i := 0; i < rv.Len(); i++ {
			parts = append(parts, renderAbiValue(rv.Index(i).Interface()))
		}
		return strings.Join(parts, ", ")
	default:
		return fmt.Sprintf("%v", v)
	}
}
