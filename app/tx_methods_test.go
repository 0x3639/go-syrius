package app

import (
	"testing"

	"github.com/zenon-network/go-zenon/common/types"
	"github.com/zenon-network/go-zenon/vm/embedded/definition"
)

func TestDecodeMethod(t *testing.T) {
	data, err := definition.ABIPillars.PackMethod("CollectReward")
	if err != nil {
		t.Fatalf("PackMethod: %v", err)
	}
	if got := decodeMethod(types.PillarContract, data); got != "CollectReward" {
		t.Fatalf("decodeMethod = %q, want CollectReward", got)
	}
	// A non-embedded address yields no method.
	if got := decodeMethod(types.ParseAddressPanic("z1qqjnwjjpnue8xmmpanz6csze6tcmtzzdtfsww7"), data); got != "" {
		t.Fatalf("decodeMethod(non-contract) = %q, want empty", got)
	}
	// Too-short data yields no method.
	if got := decodeMethod(types.PillarContract, []byte{1, 2}); got != "" {
		t.Fatalf("decodeMethod(short) = %q, want empty", got)
	}
}
