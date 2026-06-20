//go:build integration

package spike

import (
	"math/big"
	"os"
	"strings"
	"testing"
	"time"

	sdkwallet "github.com/0x3639/znn-sdk-go/wallet"
	"github.com/0x3639/znn-sdk-go/rpc_client"
	"github.com/0x3639/znn-sdk-go/zenon"
	gzwallet "github.com/zenon-network/go-zenon/wallet"
	"github.com/zenon-network/go-zenon/common/types"
)

// Env:
//   ZNN_TESTNET_URL       testnet node URL (required)
//   ZNN_KEYSTORE          path to a syrius keystore (default secrets/pillar.json)
//   ZNN_KEYSTORE_PASSWORD keystore password (else read from secrets/pillar-password.txt)
//   ZNN_SEND_TO           recipient z1… (default: the wallet's own address — self-send)
//
// Flow (per the keystore + no-SDK-modification findings):
//  1. Read the keystore with go-zenon (canonical; the SDK can't read syrius keystores).
//  2. Bridge to an SDK *wallet.KeyPair via the recovered mnemonic, asserting the two
//     derivations agree (cross-checks SDK vs go-zenon BIP-44 derivation).
//  3. Build a send template and publish it through the SDK's zenon.Send
//     (autofill → required-PoW → canonical PoW → sign → publish).
//  4. Poll for on-chain confirmation.
func TestTestnetSend(t *testing.T) {
	url := os.Getenv("ZNN_TESTNET_URL")
	if url == "" {
		t.Skip("set ZNN_TESTNET_URL to run")
	}
	keystorePath := os.Getenv("ZNN_KEYSTORE")
	if keystorePath == "" {
		keystorePath = "../../secrets/pillar.json"
	}
	password := os.Getenv("ZNN_KEYSTORE_PASSWORD")
	if password == "" {
		raw, err := os.ReadFile("../../secrets/pillar-password.txt")
		if err != nil {
			t.Skip("no keystore password (set ZNN_KEYSTORE_PASSWORD or provide secrets/pillar-password.txt)")
		}
		password = strings.TrimSpace(string(raw))
	}

	// 1. Read keystore with go-zenon (canonical reader).
	kf, err := gzwallet.ReadKeyFile(keystorePath)
	if err != nil {
		t.Fatalf("go-zenon ReadKeyFile: %v", err)
	}
	gzKs, err := kf.Decrypt(password)
	if err != nil {
		t.Fatalf("go-zenon Decrypt: %v", err)
	}

	// 2. Bridge to an SDK keypair via the mnemonic and cross-check derivation.
	sdkKs, err := sdkwallet.NewKeyStoreFromMnemonic(gzKs.Mnemonic)
	if err != nil {
		t.Fatalf("SDK NewKeyStoreFromMnemonic: %v", err)
	}
	kp, err := sdkKs.GetKeyPair(0)
	if err != nil {
		t.Fatalf("SDK GetKeyPair(0): %v", err)
	}
	addr, err := kp.GetAddress()
	if err != nil {
		t.Fatalf("SDK GetAddress: %v", err)
	}
	if *addr != kf.BaseAddress {
		t.Fatalf("SDK-derived address %s != go-zenon baseAddress %s (derivation mismatch)", addr.String(), kf.BaseAddress.String())
	}

	toAddr := *addr // self-send by default
	if to := os.Getenv("ZNN_SEND_TO"); to != "" {
		toAddr = types.ParseAddressPanic(to)
	}

	// 3. Build and publish via the SDK send facade.
	client, err := rpc_client.NewRpcClient(url)
	if err != nil {
		t.Fatalf("NewRpcClient: %v", err)
	}
	defer client.Stop()

	amount := big.NewInt(10_000_000) // 0.1 ZNN
	template := client.LedgerApi.SendTemplate(toAddr, types.ZnnTokenStandard, amount, nil)

	z := zenon.NewZenon(client)
	published, err := z.Send(template, kp)
	if err != nil {
		t.Fatalf("zenon.Send: %v", err)
	}
	t.Logf("published tx hash=%s height=%d", published.Hash, published.Height)

	// 4. Poll for confirmation.
	deadline := time.Now().Add(90 * time.Second)
	for {
		got, err := client.LedgerApi.GetAccountBlockByHash(published.Hash)
		if err == nil && got != nil && got.ConfirmationDetail != nil {
			t.Logf("confirmed at momentum height %d", got.ConfirmationDetail.MomentumHeight)
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("tx %s not confirmed within deadline", published.Hash)
		}
		time.Sleep(3 * time.Second)
	}
}
