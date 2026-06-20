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
