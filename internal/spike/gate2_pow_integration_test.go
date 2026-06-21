//go:build integration

package spike

import (
	"testing"
	"time"

	"github.com/0x3639/go-syrius/app"
	"github.com/zenon-network/go-zenon/common/types"
)

// TestGate2PoWReceive proves an end-to-end Proof-of-Work transaction confirms
// on-chain — the Gate-2 carry-forward. It funds an unfused account (index 1)
// from the fused base account (a plasma-covered send), then RECEIVES on the
// unfused account. A receive on an account with no fused plasma must satisfy
// PoW, so the published receive block carries Difficulty > 0 and confirming it
// is a genuine end-to-end PoW proof — without needing the unfused account to
// already hold spendable funds.
func TestGate2PoWReceive(t *testing.T) {
	env := resolveEnv(t)
	assertTestnet(t, env)

	const fundAmount = "50000000" // 0.5 ZNN, base units
	powAddr := activeAddressFor(t, env, 1)

	// 1. Fund the unfused account from the fused base account (plasma-covered).
	{
		a, client := buildApp(t, env, 0)
		preview, err := a.Tx.PrepareSend(app.SendRequest{
			ToAddress: powAddr.String(),
			Zts:       types.ZnnTokenStandard.String(),
			Amount:    fundAmount,
		})
		if err != nil {
			t.Fatalf("fund PrepareSend: %v", err)
		}
		t.Logf("funding index1: to=%s amount=%s hash=%s needsPoW=%v",
			preview.ToAddress, preview.Amount, preview.Hash, preview.NeedsPoW)
		hash, err := a.Tx.ConfirmPublish()
		if err != nil {
			t.Fatalf("fund ConfirmPublish: %v", err)
		}
		pollConfirmed(t, client, hashOf(t, hash))
	}

	// 2. Receive on the unfused account — this requires PoW.
	a, client := buildApp(t, env, 1)

	var fromHash string
	deadline := time.Now().Add(confirmTimeout)
	for {
		unreceived, err := a.Node.GetUnreceived()
		if err != nil {
			t.Fatalf("GetUnreceived: %v", err)
		}
		if len(unreceived) > 0 {
			fromHash = unreceived[0].FromHash
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("no unreceived block appeared for the unfused account within %s", confirmTimeout)
		}
		time.Sleep(pollInterval)
	}

	recvHash, err := a.Tx.Receive(fromHash)
	if err != nil {
		t.Fatalf("Receive (PoW path): %v", err)
	}
	t.Logf("PoW receive published: hash=%s", recvHash)

	// The receive block must have used PoW (Difficulty > 0) because the unfused
	// account has no plasma. Confirm it landed and verify the difficulty.
	pollConfirmed(t, client, hashOf(t, recvHash))
	got, err := client.LedgerApi.GetAccountBlockByHash(hashOf(t, recvHash))
	if err != nil {
		t.Fatalf("GetAccountBlockByHash(%s): %v", recvHash, err)
	}
	if got == nil || got.Difficulty == 0 {
		t.Fatalf("expected PoW receive (Difficulty > 0), got difficulty=%v", got)
	}
	t.Logf("GATE-2 PASSED: end-to-end PoW receive confirmed, difficulty=%d", got.Difficulty)
}
