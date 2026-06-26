package app

import (
	"github.com/zenon-network/go-zenon/common/types"
	"github.com/zenon-network/go-zenon/vm/abi"
	"github.com/zenon-network/go-zenon/vm/embedded/definition"
)

// embeddedABIs maps each embedded-contract address to its ABI, so an action
// block targeting one can be named by the method it calls (CollectReward,
// Delegate, Fuse, …) — the Type/Method column, mirroring nomscan.
var embeddedABIs = map[types.Address]abi.ABIContract{
	types.PillarContract:      definition.ABIPillars,
	types.SentinelContract:    definition.ABISentinel,
	types.StakeContract:       definition.ABIStake,
	types.PlasmaContract:      definition.ABIPlasma,
	types.TokenContract:       definition.ABIToken,
	types.AcceleratorContract: definition.ABIAccelerator,
	types.LiquidityContract:   definition.ABILiquidity,
	types.SporkContract:       definition.ABISpork,
}

// decodeMethod returns the embedded-contract method name a send's data calls, or
// "" if the target isn't a known embedded contract or the data can't be decoded.
func decodeMethod(to types.Address, data []byte) string {
	c, ok := embeddedABIs[to]
	if !ok || len(data) < 4 {
		return ""
	}
	m, err := c.MethodById(data[:4])
	if err != nil || m == nil {
		return ""
	}
	return m.Name
}
