package app

import (
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

// clientTokenDecimals returns a lookup that reads a token's decimals from the
// node's TokenApi (the same GetByZts path GetTokenByZts uses). A nil/missing
// token reports defaultDecimals via a nil error so the caller renders 8.
func clientTokenDecimals(client *rpc_client.RpcClient) ztsDecimalsLookup {
	return func(zts types.ZenonTokenStandard) (int, error) {
		tok, err := client.TokenApi.GetByZts(zts)
		if err != nil {
			return defaultDecimals, err
		}
		if tok == nil || tok.TokenStandard == types.ZeroTokenStandard {
			return defaultDecimals, nil
		}
		return int(tok.Decimals), nil
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
