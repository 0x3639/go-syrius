package app

import (
	"encoding/base64"
	"math/big"
	"strings"
	"testing"

	embedded "github.com/0x3639/znn-sdk-go/api/embedded"
	"github.com/zenon-network/go-zenon/common/types"
)

const (
	effAddr1 = "z1qzal6c5s9rjnnxd2z7dvdhjxpmmj4fmw56a0mz"
	effAddr2 = "z1qr4pexnnfaexqqz8nscjjcsajy5hdqfkgadvwx"
	effHash  = "00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff"
	// A value far beyond uint64 — exercises big.Int rendering.
	effBigAmount = "123456789012345678901234567890"
)

// proposeKindSample returns, per action kind, the params to build it with and
// the exact rendered values that MUST appear in the decoded effect fields.
func proposeKindSample(t *testing.T, kind string) (params map[string]string, want []string) {
	t.Helper()
	znn := types.ZnnTokenStandard.String()
	qsr := types.QsrTokenStandard.String()
	switch kind {
	case "spork.create":
		return map[string]string{"name": "test-spork", "description": "gates the new hash algo"},
			[]string{"test-spork", "gates the new hash algo"}
	case "spork.activate":
		return map[string]string{"id": effHash}, []string{effHash}
	case "bridge.addNetwork":
		return map[string]string{"networkClass": "2", "chainId": "31337", "name": "Ethereum", "contractAddress": "0xabcdef0123", "metadata": `{"rpc":"wss://x"}`},
			[]string{"2", "31337", "Ethereum", "0xabcdef0123", `{"rpc":"wss://x"}`}
	case "bridge.removeNetwork":
		return map[string]string{"networkClass": "2", "chainId": "31337"}, []string{"2", "31337"}
	case "bridge.setTokenPair":
		return map[string]string{
				"networkClass": "1", "chainId": "2", "tokenStandard": znn, "tokenAddress": "0xdeadbeef",
				"bridgeable": "true", "redeemable": "false", "owned": "true",
				"minAmount": effBigAmount, "fee": "15", "redeemDelay": "40", "metadata": "{}",
			},
			[]string{znn, "0xdeadbeef", "true", "false", effBigAmount, "15", "40", "{}"}
	case "bridge.removeTokenPair":
		return map[string]string{"networkClass": "1", "chainId": "2", "tokenStandard": qsr, "tokenAddress": "0xdeadbeef"},
			[]string{qsr, "0xdeadbeef"}
	case "bridge.halt":
		return map[string]string{"signature": "halt-signature-data"}, []string{"halt-signature-data"}
	case "bridge.unhalt", "bridge.emergency", "liquidity.emergency":
		return map[string]string{}, nil
	case "bridge.changeAdministrator":
		return map[string]string{"administrator": effAddr1}, []string{effAddr1}
	case "bridge.changeTssECDSAPubKey":
		return map[string]string{"pubKey": "pubkey-b64", "signature": "old-sig", "newSignature": "new-sig"},
			[]string{"pubkey-b64", "old-sig", "new-sig"}
	case "bridge.setAllowKeygen":
		return map[string]string{"allowKeygen": "true"}, []string{"true"}
	case "bridge.setOrchestratorInfo":
		return map[string]string{"windowSize": "6", "keyGenThreshold": "66", "confirmationsToFinality": "20", "estimatedMomentumTime": "10"},
			[]string{"6", "66", "20", "10"}
	case "bridge.setMetadata":
		return map[string]string{"metadata": `{"affiliate":true}`}, []string{`{"affiliate":true}`}
	case "bridge.setNetworkMetadata":
		return map[string]string{"networkClass": "1", "chainId": "2", "metadata": `{"n":1}`},
			[]string{`{"n":1}`}
	case "bridge.revokeUnwrapRequest":
		return map[string]string{"transactionHash": effHash, "logIndex": "3"}, []string{effHash, "3"}
	case "bridge.nominateGuardians", "liquidity.nominateGuardians":
		return map[string]string{"guardians": effAddr1 + "," + effAddr2},
			[]string{effAddr1 + ", " + effAddr2}
	case "liquidity.fund":
		return map[string]string{"znnReward": effBigAmount, "qsrReward": "500000000"},
			[]string{effBigAmount, "500000000"}
	case "liquidity.burnZnn":
		return map[string]string{"burnAmount": effBigAmount}, []string{effBigAmount}
	case "liquidity.setTokenTuple":
		return map[string]string{
				"tokenStandards": znn + "," + qsr,
				"znnPercentages": "7000,3000",
				"qsrPercentages": "4000,6000",
				"minAmounts":     "100," + effBigAmount,
			},
			[]string{znn + ", " + qsr, "7000, 3000", "4000, 6000", "100, " + effBigAmount}
	case "liquidity.setIsHalted":
		return map[string]string{"value": "true"}, []string{"true"}
	case "liquidity.unlockStakeEntries":
		// NOTE: the on-chain UnlockLiquidityStakeEntries method takes NO ABI
		// arguments — the SDK helper puts the zts on the wrapped template's
		// TokenStandard, which a ProposalPayload (destination+data only) does
		// not carry. The decoded effect is truthfully empty; the zts form field
		// never reaches the chain through this propose kind.
		return map[string]string{"zts": znn}, nil
	case "liquidity.setAdditionalReward":
		return map[string]string{"znnReward": "100000000", "qsrAmount": "1000000000"},
			[]string{"100000000", "1000000000"}
	case "liquidity.changeAdministrator":
		return map[string]string{"administrator": effAddr2}, []string{effAddr2}
	}
	t.Fatalf("no sample params for propose kind %q — add one so every kind stays decode-tested", kind)
	return nil, nil
}

// TestDecodeEveryProposeKind builds the exact payload for EVERY proposable
// governance action kind, decodes the returned bytes, and asserts every
// supplied parameter appears in the structured effect with its exact rendered
// value (full addresses, full big-int amounts, explicit booleans, joined
// lists). This is the confirm-what-you-sign gate for governance proposals.
func TestDecodeEveryProposeKind(t *testing.T) {
	api := embedded.NewGovernanceApi(nil) // payload helpers are pure template builders
	for _, k := range proposeKinds() {
		k := k
		t.Run(k.Kind, func(t *testing.T) {
			params, want := proposeKindSample(t, k.Kind)
			payload, err := buildProposalPayloadWith(api, k.Kind, params)
			if err != nil {
				t.Fatalf("build payload: %v", err)
			}
			data, err := base64.StdEncoding.DecodeString(payload.Data)
			if err != nil {
				t.Fatalf("payload data is not base64: %v", err)
			}
			effect, err := decodeContractCall(payload.Destination, data)
			if err != nil {
				t.Fatalf("decode: %v", err)
			}
			if effect.Contract == "" || effect.Method == "" {
				t.Fatalf("effect must name the contract and method, got %+v", effect)
			}
			rendered := make([]string, 0, len(effect.Fields))
			for _, f := range effect.Fields {
				rendered = append(rendered, f.Value)
			}
			for _, w := range want {
				found := false
				for _, v := range rendered {
					if v == w {
						found = true
						break
					}
				}
				if !found {
					t.Fatalf("expected value %q missing from decoded fields %v", w, rendered)
				}
			}
		})
	}
}

func TestDecodeContractCallFailsClosed(t *testing.T) {
	api := embedded.NewGovernanceApi(nil)
	good, err := buildProposalPayloadWith(api, "bridge.changeAdministrator", map[string]string{"administrator": effAddr1})
	if err != nil {
		t.Fatal(err)
	}
	data, _ := base64.StdEncoding.DecodeString(good.Data)

	// Unknown destination (a regular user address) must refuse.
	user, _ := types.ParseAddress(effAddr1)
	if _, err := decodeContractCall(user, data); err == nil {
		t.Fatal("unknown destination must fail closed")
	}
	// A method selector from a DIFFERENT contract must refuse.
	if _, err := decodeContractCall(types.TokenContract, data); err == nil {
		t.Fatal("destination/method mismatch must fail closed")
	}
	// Truncated data must refuse.
	if _, err := decodeContractCall(good.Destination, data[:2]); err == nil {
		t.Fatal("truncated selector must fail closed")
	}
	if _, err := decodeContractCall(good.Destination, data[:len(data)-1]); err == nil {
		t.Fatal("truncated arguments must fail closed")
	}
	// Trailing garbage must refuse (round-trip check) — the fields would not
	// fully describe what executes.
	if _, err := decodeContractCall(good.Destination, append(append([]byte(nil), data...), 0xde, 0xad)); err == nil {
		t.Fatal("trailing bytes must fail closed")
	}
	// Unknown selector on a known contract must refuse.
	bogus := append([]byte{0xde, 0xad, 0xbe, 0xef}, data[4:]...)
	if _, err := decodeContractCall(good.Destination, bogus); err == nil {
		t.Fatal("unknown method must fail closed")
	}
}

// TestDecodeAcceleratorTemplates decodes the exact templates the accelerator
// prepare paths hold, proving name, full description, URL, and base-unit
// amounts all surface (PR-06). Unicode + long strings included.
func TestDecodeAcceleratorTemplates(t *testing.T) {
	accel := embedded.NewAcceleratorApi(nil)
	longDesc := strings.Repeat("véry lông déscription — ", 10) + "🚀 end"
	url := "https://forum.zenon.org/some/very/long/path?with=query&and=more"
	znn := "500000000000"
	qsr := effBigAmount

	t.Run("CreateProject", func(t *testing.T) {
		tmpl := accel.CreateProject("Test Prøject", longDesc, url, mustBig(t, znn), mustBig(t, qsr))
		effect, err := decodeContractCall(types.AcceleratorContract, tmpl.Data)
		if err != nil {
			t.Fatalf("decode: %v", err)
		}
		assertEffectHasValues(t, effect, "Test Prøject", longDesc, url, znn, qsr)
	})
	t.Run("AddPhase", func(t *testing.T) {
		h, _ := types.HexToHash(effHash)
		tmpl := accel.AddPhase(h, "Phase Ⅰ", longDesc, url, mustBig(t, znn), mustBig(t, qsr))
		effect, err := decodeContractCall(types.AcceleratorContract, tmpl.Data)
		if err != nil {
			t.Fatalf("decode: %v", err)
		}
		assertEffectHasValues(t, effect, effHash, "Phase Ⅰ", longDesc, url, znn, qsr)
	})
	t.Run("UpdatePhase", func(t *testing.T) {
		h, _ := types.HexToHash(effHash)
		tmpl := accel.UpdatePhase(h, "Phase Ⅱ", longDesc, url, mustBig(t, znn), mustBig(t, qsr))
		effect, err := decodeContractCall(types.AcceleratorContract, tmpl.Data)
		if err != nil {
			t.Fatalf("decode: %v", err)
		}
		assertEffectHasValues(t, effect, effHash, "Phase Ⅱ", longDesc, url, znn, qsr)
	})
}

func assertEffectHasValues(t *testing.T, effect *TransactionEffect, want ...string) {
	t.Helper()
	rendered := make([]string, 0, len(effect.Fields))
	for _, f := range effect.Fields {
		rendered = append(rendered, f.Value)
	}
	for _, w := range want {
		found := false
		for _, v := range rendered {
			if v == w {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected value %q missing from decoded fields %v", w, rendered)
		}
	}
}

func mustBig(t *testing.T, s string) *big.Int {
	t.Helper()
	n, ok := new(big.Int).SetString(s, 10)
	if !ok {
		t.Fatalf("bad big int %q", s)
	}
	return n
}
