//go:build integration

package spike

import (
	"os"
	"testing"

	"github.com/0x3639/znn-sdk-go/rpc_client"
	"github.com/zenon-network/go-zenon/common/types"
)

// Env:
//   ZNN_NODE_URL  — wss:// or ws:// node URL (required)
//   ZNN_TEST_ADDR — a z1… address to read balances for (required)
func TestReadOnlyRPC(t *testing.T) {
	url := os.Getenv("ZNN_NODE_URL")
	addrStr := os.Getenv("ZNN_TEST_ADDR")
	if url == "" || addrStr == "" {
		t.Skip("set ZNN_NODE_URL and ZNN_TEST_ADDR to run")
	}

	client, err := rpc_client.NewRpcClient(url)
	if err != nil {
		t.Fatalf("NewRpcClient: %v", err)
	}
	defer client.Stop()

	momentum, err := client.LedgerApi.GetFrontierMomentum()
	if err != nil {
		t.Fatalf("GetFrontierMomentum: %v", err)
	}
	if momentum.Height == 0 {
		t.Fatalf("frontier momentum height is 0, expected a live chain")
	}
	t.Logf("frontier height=%d chainId=%d", momentum.Height, momentum.ChainIdentifier)

	addr := types.ParseAddressPanic(addrStr)
	info, err := client.LedgerApi.GetAccountInfoByAddress(addr)
	if err != nil {
		t.Fatalf("GetAccountInfoByAddress: %v", err)
	}
	for zts, bal := range info.BalanceInfoMap {
		t.Logf("balance %s = %v", zts, bal.Balance)
	}
}

// TestReadOnlyPillars exercises the Phase-5c pillar read path against a live
// node. It proves the node exposes the `embedded` RPC namespace (a node serving
// only `ledger` returns "embedded.* does not exist") and that the exact PillarApi
// calls NomService relies on succeed end-to-end. Read-only: no PoW, no signing.
//
// Env:
//   ZNN_NODE_URL  — ws:// or wss:// node URL (required)
//   ZNN_TEST_ADDR — a z1… address; if set, its delegation + reward are read too
func TestReadOnlyPillars(t *testing.T) {
	url := os.Getenv("ZNN_NODE_URL")
	if url == "" {
		t.Skip("set ZNN_NODE_URL to run")
	}

	client, err := rpc_client.NewRpcClient(url)
	if err != nil {
		t.Fatalf("NewRpcClient: %v", err)
	}
	defer client.Stop()

	list, err := client.PillarApi.GetAll(0, 50)
	if err != nil {
		t.Fatalf("PillarApi.GetAll (embedded namespace enabled?): %v", err)
	}
	t.Logf("pillars: count=%d returned=%d", list.Count, len(list.List))
	for i, p := range list.List {
		if i >= 5 {
			break
		}
		t.Logf("  pillar rank=%d name=%q weight=%v giveReward%%=%d", p.Rank, p.Name, p.Weight, p.GiveDelegateRewardPercentage)
	}

	addrStr := os.Getenv("ZNN_TEST_ADDR")
	if addrStr == "" {
		t.Log("ZNN_TEST_ADDR not set — skipping delegation/reward reads")
		return
	}
	addr := types.ParseAddressPanic(addrStr)

	d, err := client.PillarApi.GetDelegatedPillar(addr)
	if err != nil {
		t.Fatalf("GetDelegatedPillar: %v", err)
	}
	if d == nil || d.Name == "" {
		t.Logf("delegation: not delegated")
	} else {
		t.Logf("delegation: name=%q status=%d weight=%v", d.Name, d.Status, d.Weight)
	}

	r, err := client.PillarApi.GetUncollectedReward(addr)
	if err != nil {
		t.Fatalf("GetUncollectedReward: %v", err)
	}
	t.Logf("uncollected delegation reward: znn=%v qsr=%v", r.ZnnAmount, r.QsrAmount)
}

// TestReadOnlySentinels exercises the Phase-5d sentinel read path against a live
// node (proves the embedded namespace + the exact SentinelApi calls NomService
// uses). Read-only: no PoW, no signing.
//
// Env:
//   ZNN_NODE_URL  — ws:// or wss:// node URL (required)
//   ZNN_TEST_ADDR — a z1… address (required)
func TestReadOnlySentinels(t *testing.T) {
	url := os.Getenv("ZNN_NODE_URL")
	addrStr := os.Getenv("ZNN_TEST_ADDR")
	if url == "" || addrStr == "" {
		t.Skip("set ZNN_NODE_URL and ZNN_TEST_ADDR to run")
	}
	client, err := rpc_client.NewRpcClient(url)
	if err != nil {
		t.Fatalf("NewRpcClient: %v", err)
	}
	defer client.Stop()
	addr := types.ParseAddressPanic(addrStr)

	info, err := client.SentinelApi.GetByOwner(addr)
	if err != nil {
		t.Fatalf("GetByOwner (embedded namespace enabled?): %v", err)
	}
	t.Logf("sentinel: registrationTimestamp=%d active=%v isRevocable=%v cooldown=%d", info.RegistrationTimestamp, info.Active, info.IsRevocable, info.RevokeCooldown)

	q, err := client.SentinelApi.GetDepositedQsr(addr)
	if err != nil {
		t.Fatalf("GetDepositedQsr: %v", err)
	}
	t.Logf("deposited QSR: %v", q)

	r, err := client.SentinelApi.GetUncollectedReward(addr)
	if err != nil {
		t.Fatalf("GetUncollectedReward: %v", err)
	}
	t.Logf("uncollected sentinel reward: znn=%v qsr=%v", r.ZnnAmount, r.QsrAmount)
}
