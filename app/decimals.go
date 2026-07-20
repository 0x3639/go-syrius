package app

import (
	"fmt"

	"github.com/0x3639/znn-sdk-go/rpc_client"
	"github.com/zenon-network/go-zenon/common/types"
)

// defaultDecimals is the fallback used for ZNN/QSR and whenever a token's real
// decimals cannot be resolved (a missing token or a node error). ZNN and QSR are
// both 8-decimal, so a hardcoded 8 is only ever wrong for custom tokens, which
// this resolver looks up via the node.
const defaultDecimals = 8

// ztsDecimalsLookup resolves the on-chain decimals for a parsed ZTS. It is the
// node-touching seam, injected so tests can supply a stub without a live node.
type ztsDecimalsLookup func(zts types.ZenonTokenStandard) (int, error)

// resolveDecimals returns the number of decimals for a ZTS string. ZNN and QSR
// resolve to 8 WITHOUT any node call; any other token is looked up via lookup,
// falling back to 8 on a parse error, a lookup error, or a missing token.
func resolveDecimals(zts string, lookup ztsDecimalsLookup) int {
	switch zts {
	case types.ZnnTokenStandard.String(), types.QsrTokenStandard.String():
		return defaultDecimals
	}
	parsed, err := types.ParseZTS(zts)
	if err != nil {
		return defaultDecimals
	}
	if lookup == nil {
		return defaultDecimals
	}
	d, err := lookup(parsed)
	if err != nil {
		return defaultDecimals
	}
	return d
}

// resolveDecimalsChecked is the confirmation-strict variant of resolveDecimals:
// ZNN and QSR resolve to their protocol-fixed 8 without a node call, but a
// custom token whose decimals cannot be resolved is an ERROR, never a guessed
// 8 — a confirmation dialog must not render an amount with assumed decimals.
func resolveDecimalsChecked(zts string, lookup ztsDecimalsLookup) (int, error) {
	switch zts {
	case types.ZnnTokenStandard.String(), types.QsrTokenStandard.String():
		return defaultDecimals, nil
	}
	parsed, err := types.ParseZTS(zts)
	if err != nil {
		return 0, fmt.Errorf("cannot resolve token decimals: invalid ZTS %q: %w", zts, err)
	}
	if lookup == nil {
		return 0, fmt.Errorf("cannot resolve decimals for token %s: no node lookup available", zts)
	}
	d, err := lookup(parsed)
	if err != nil {
		return 0, fmt.Errorf("cannot resolve decimals for token %s: %w", zts, err)
	}
	return d, nil
}

// errTokenNotFound reports token metadata that does not exist on the connected
// node. Display paths (resolveDecimals) degrade to defaultDecimals; the strict
// confirmation path (resolveDecimalsChecked) must treat it as a failure — a
// confirmation can never render an amount with guessed decimals.
var errTokenNotFound = fmt.Errorf("token metadata not found on the connected node")

// boundTokenDecimals rejects node-reported decimals outside the protocol range
// [0,18] (issuance validates 0..18 on-chain, nom_service.go; a node reporting
// anything else is lying and must not skew the human-readable amount — GS-07).
func boundTokenDecimals(d int, zts types.ZenonTokenStandard) (int, error) {
	if d < 0 || d > 18 {
		return 0, fmt.Errorf("node reports implausible decimals %d for %s (valid range 0-18)", d, zts)
	}
	return d, nil
}

// clientTokenDecimals returns a lookup that reads a token's decimals from the
// node's TokenApi (the same GetByZts path GetTokenByZts uses). A nil/missing
// token is an ERROR; fail-open callers fall back to 8 themselves.
func clientTokenDecimals(client *rpc_client.RpcClient) ztsDecimalsLookup {
	return func(zts types.ZenonTokenStandard) (int, error) {
		tok, err := client.TokenApi.GetByZts(zts)
		if err != nil {
			return defaultDecimals, err
		}
		if tok == nil || tok.TokenStandard == types.ZeroTokenStandard {
			return 0, errTokenNotFound
		}
		return boundTokenDecimals(int(tok.Decimals), zts)
	}
}

// decimalsCache memoizes resolved decimals per ZTS for a single list pass so a
// page of rows sharing a token only queries the node once. It is NOT safe for
// concurrent use; construct one per list call.
type decimalsCache struct {
	lookup ztsDecimalsLookup
	cache  map[string]int
}

func newDecimalsCache(lookup ztsDecimalsLookup) *decimalsCache {
	return &decimalsCache{lookup: lookup, cache: map[string]int{}}
}

func (c *decimalsCache) get(zts string) int {
	if d, ok := c.cache[zts]; ok {
		return d
	}
	d := resolveDecimals(zts, c.lookup)
	c.cache[zts] = d
	return d
}
