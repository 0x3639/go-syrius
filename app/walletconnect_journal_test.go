package app

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"testing"

	"github.com/0x3639/znn-sdk-go/rpc_client"
	sdkwallet "github.com/0x3639/znn-sdk-go/wallet"
	"github.com/zenon-network/go-zenon/chain/nom"
	"github.com/zenon-network/go-zenon/common/types"
)

// wcTestHold manufactures a WalletConnect hold the way PrepareWalletConnectSend
// would, bypassing the node-dependent prepare path (chain 3 so the mainnet
// opt-in is not required unless a test wants it).
func wcTestHold(t *testing.T, tx *TxService, chainID uint64, topic string, requestID uint64) (uint64, *nom.AccountBlock) {
	t.Helper()
	unlockTestWallet(t, tx.wallet)
	from, ok := tx.wallet.activeAddress()
	if !ok {
		t.Fatal("no active address after unlock")
	}
	tx.node.chainID = chainID
	tx.node.client = &rpc_client.RpcClient{}
	req, _ := validWalletConnectRequest(t)
	data, err := base64.StdEncoding.DecodeString(req.AccountBlock.Data)
	if err != nil {
		t.Fatal(err)
	}
	template := &nom.AccountBlock{
		Version:         1,
		ChainIdentifier: chainID,
		BlockType:       nom.BlockTypeUserSend,
		Address:         from,
		ToAddress:       types.BridgeContract,
		Amount:          big.NewInt(100000000),
		TokenStandard:   types.ZnnTokenStandard,
		Data:            data,
	}
	expect := callExpect{from: from, to: types.BridgeContract, zts: types.ZnnTokenStandard, amount: big.NewInt(100000000), data: append([]byte(nil), data...)}
	holdID := mustHoldPending(t, tx, template, expect, tx.wallet.sessionGen())
	if !tx.attachWalletConnectIdentity(holdID, topic, requestID, walletConnectIntentHash(template)) {
		t.Fatal("attachWalletConnectIdentity refused a live hold")
	}
	return holdID, template
}

func stubBuilt(template *nom.AccountBlock, hashHex string) *nom.AccountBlock {
	built := *template
	h, err := types.HexToHash(hashHex)
	if err != nil {
		panic(err)
	}
	built.Hash = h
	return &built
}

func TestWalletConnectJournalPersistsAcrossInstances(t *testing.T) {
	tx := newTestTxService(t)
	key := wcJournalKey("topic-a", 7)
	rec := wcPublicationRecord{IntentHash: "abc", State: wcStateSigned, BlockJSON: json.RawMessage(`{"hash":"00"}`), Hash: "00", CreatedAt: 1}
	if err := tx.wcJournal.put(key, rec); err != nil {
		t.Fatal(err)
	}
	// A fresh service over the same data dir must read the same record: the
	// journal is the restart-survival boundary, not process memory.
	tx2 := newTxService(tx.config, tx.wallet, tx.node)
	got, ok, err := tx2.wcJournal.get(key)
	if err != nil || !ok || got.IntentHash != "abc" || got.State != wcStateSigned {
		t.Fatalf("journal did not survive restart: %+v ok=%v err=%v", got, ok, err)
	}
	if err := tx2.wcJournal.delete(key); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := tx.wcJournal.get(key); ok {
		t.Fatal("deleted journal record still present")
	}
}

func TestWalletConnectJournalBoundsRetention(t *testing.T) {
	tx := newTestTxService(t)
	for i := 0; i < wcJournalMaxRecords+8; i++ {
		rec := wcPublicationRecord{IntentHash: "x", State: wcStatePublished, Hash: "00", CreatedAt: int64(i)}
		if err := tx.wcJournal.put(wcJournalKey("t", uint64(i)), rec); err != nil {
			t.Fatal(err)
		}
	}
	all, err := tx.wcJournal.load()
	if err != nil {
		t.Fatal(err)
	}
	if len(all) > wcJournalMaxRecords {
		t.Fatalf("journal grew to %d records; cap is %d", len(all), wcJournalMaxRecords)
	}
	// The newest record must have survived eviction.
	if _, ok, _ := tx.wcJournal.get(wcJournalKey("t", uint64(wcJournalMaxRecords+7))); !ok {
		t.Fatal("newest record was evicted")
	}
}

func TestConfirmWalletConnectPublishPersistsSignedBlockBeforeBroadcast(t *testing.T) {
	tx := newTestTxService(t)
	holdID, template := wcTestHold(t, tx, 3, "topic-b", 9)
	var journaledAtPublish bool
	tx.prepareBlockFn = func(_ *rpc_client.RpcClient, tpl *nom.AccountBlock, _ *sdkwallet.KeyPair) (*nom.AccountBlock, error) {
		return stubBuilt(tpl, "1111111111111111111111111111111111111111111111111111111111111111"), nil
	}
	tx.publishFn = func(_ *rpc_client.RpcClient, block *nom.AccountBlock) error {
		_, ok, _ := tx.wcJournal.get(wcJournalKey("topic-b", 9))
		journaledAtPublish = ok
		return errors.New("write: broken pipe")
	}
	_, err := tx.ConfirmWalletConnectPublish(holdID)
	if err == nil || !strings.Contains(err.Error(), "outcome") {
		t.Fatalf("got %v, want an outcome-unknown error, not a definite failure", err)
	}
	if !journaledAtPublish {
		t.Fatal("signed block was not journaled BEFORE the broadcast attempt")
	}
	rec, ok, _ := tx.wcJournal.get(wcJournalKey("topic-b", 9))
	if !ok || rec.State != wcStateSigned {
		t.Fatalf("after an uncertain broadcast the record must remain signed/unknown: %+v ok=%v", rec, ok)
	}
	if rec.IntentHash != walletConnectIntentHash(template) {
		t.Fatal("journal record does not carry the validated intent hash")
	}
	// The hold is released; the journal now owns the block.
	if tx.pending != nil {
		t.Fatal("hold should be cleared once the signed block is journaled")
	}
}

func TestConfirmWalletConnectPublishMarksPublishedOnSuccess(t *testing.T) {
	tx := newTestTxService(t)
	holdID, _ := wcTestHold(t, tx, 3, "topic-c", 11)
	tx.prepareBlockFn = func(_ *rpc_client.RpcClient, tpl *nom.AccountBlock, _ *sdkwallet.KeyPair) (*nom.AccountBlock, error) {
		return stubBuilt(tpl, "2222222222222222222222222222222222222222222222222222222222222222"), nil
	}
	tx.publishFn = func(_ *rpc_client.RpcClient, _ *nom.AccountBlock) error { return nil }
	result, err := tx.ConfirmWalletConnectPublish(holdID)
	if err != nil {
		t.Fatal(err)
	}
	if result["hash"] != "2222222222222222222222222222222222222222222222222222222222222222" {
		t.Fatalf("published result hash = %v", result["hash"])
	}
	rec, ok, _ := tx.wcJournal.get(wcJournalKey("topic-c", 11))
	if !ok || rec.State != wcStatePublished {
		t.Fatalf("record must be published after node acceptance: %+v ok=%v", rec, ok)
	}
	if err := tx.AckWalletConnectResult("topic-c", 11); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := tx.wcJournal.get(wcJournalKey("topic-c", 11)); ok {
		t.Fatal("acked record must be deleted")
	}
}

func TestReconcileWalletConnectPublicationByQueryAndRebroadcast(t *testing.T) {
	tx := newTestTxService(t)
	holdID, _ := wcTestHold(t, tx, 3, "topic-d", 13)
	tx.prepareBlockFn = func(_ *rpc_client.RpcClient, tpl *nom.AccountBlock, _ *sdkwallet.KeyPair) (*nom.AccountBlock, error) {
		return stubBuilt(tpl, "3333333333333333333333333333333333333333333333333333333333333333"), nil
	}
	broadcasts := 0
	tx.publishFn = func(_ *rpc_client.RpcClient, _ *nom.AccountBlock) error {
		broadcasts++
		return errors.New("connection reset")
	}
	if _, err := tx.ConfirmWalletConnectPublish(holdID); err == nil {
		t.Fatal("expected outcome-unknown error")
	}

	// 1. Query finds the block on chain: no rebroadcast, record published.
	tx.blockByHashFn = func(_ *rpc_client.RpcClient, h types.Hash) (bool, error) { return true, nil }
	result, err := tx.ReconcileWalletConnectPublication("topic-d", 13)
	if err != nil {
		t.Fatal(err)
	}
	if result["hash"] != "3333333333333333333333333333333333333333333333333333333333333333" {
		t.Fatalf("reconciled hash = %v", result["hash"])
	}
	if broadcasts != 1 {
		t.Fatalf("query-confirmed reconcile must not rebroadcast; broadcasts=%d", broadcasts)
	}
	rec, ok, _ := tx.wcJournal.get(wcJournalKey("topic-d", 13))
	if !ok || rec.State != wcStatePublished {
		t.Fatalf("reconciled record must be published: %+v", rec)
	}

	// 2. Reconciling an already-published record returns the same stored result
	// without touching the node.
	tx.blockByHashFn = func(_ *rpc_client.RpcClient, _ types.Hash) (bool, error) {
		t.Fatal("published record must not re-query")
		return false, nil
	}
	if again, err := tx.ReconcileWalletConnectPublication("topic-d", 13); err != nil || again["hash"] != result["hash"] {
		t.Fatalf("published reconcile: %v %v", again, err)
	}
}

func TestReconcileWalletConnectPublicationRebroadcastsExactBlock(t *testing.T) {
	tx := newTestTxService(t)
	holdID, _ := wcTestHold(t, tx, 3, "topic-e", 17)
	tx.prepareBlockFn = func(_ *rpc_client.RpcClient, tpl *nom.AccountBlock, _ *sdkwallet.KeyPair) (*nom.AccountBlock, error) {
		return stubBuilt(tpl, "4444444444444444444444444444444444444444444444444444444444444444"), nil
	}
	tx.publishFn = func(_ *rpc_client.RpcClient, _ *nom.AccountBlock) error { return errors.New("timeout") }
	if _, err := tx.ConfirmWalletConnectPublish(holdID); err == nil {
		t.Fatal("expected outcome-unknown error")
	}

	// Not on chain; the rebroadcast must send the EXACT journaled block.
	tx.blockByHashFn = func(_ *rpc_client.RpcClient, _ types.Hash) (bool, error) { return false, nil }
	var rebroadcast *nom.AccountBlock
	tx.publishFn = func(_ *rpc_client.RpcClient, block *nom.AccountBlock) error {
		rebroadcast = block
		return nil
	}
	result, err := tx.ReconcileWalletConnectPublication("topic-e", 17)
	if err != nil {
		t.Fatal(err)
	}
	if rebroadcast == nil || rebroadcast.Hash.String() != "4444444444444444444444444444444444444444444444444444444444444444" {
		t.Fatalf("rebroadcast block = %+v; must be the journaled signed block", rebroadcast)
	}
	if rebroadcast.Amount.String() != "100000000" || rebroadcast.ToAddress != types.BridgeContract {
		t.Fatal("rebroadcast block does not round-trip the journaled funds-moving fields")
	}
	if result["hash"] != "4444444444444444444444444444444444444444444444444444444444444444" {
		t.Fatalf("result hash = %v", result["hash"])
	}

	// A rebroadcast failure keeps the record signed/unknown and stays retryable.
	tx2 := newTestTxService(t)
	holdID2, _ := wcTestHold(t, tx2, 3, "topic-f", 19)
	tx2.prepareBlockFn = tx.prepareBlockFn
	tx2.publishFn = func(_ *rpc_client.RpcClient, _ *nom.AccountBlock) error { return errors.New("timeout") }
	if _, err := tx2.ConfirmWalletConnectPublish(holdID2); err == nil {
		t.Fatal("expected outcome-unknown error")
	}
	tx2.blockByHashFn = func(_ *rpc_client.RpcClient, _ types.Hash) (bool, error) { return false, nil }
	if _, err := tx2.ReconcileWalletConnectPublication("topic-f", 19); err == nil {
		t.Fatal("expected reconcile to report the still-unknown outcome")
	}
	rec, ok, _ := tx2.wcJournal.get(wcJournalKey("topic-f", 19))
	if !ok || rec.State != wcStateSigned {
		t.Fatalf("failed reconcile must keep the signed record: %+v ok=%v", rec, ok)
	}
}

func TestPrepareWalletConnectSendReplaysJournaledOutcome(t *testing.T) {
	tx := newTestTxService(t)
	unlockTestWallet(t, tx.wallet)
	active, _ := tx.wallet.activeAddress()
	tx.node.chainID = mainnetChainID
	req, _ := validWalletConnectRequest(t)
	req.FromAddress = active.String()
	req.Topic = "replay-topic"
	req.RequestID = 23
	template, _, _, err := walletConnectBridgeTemplate(req, active)
	if err != nil {
		t.Fatal(err)
	}
	built := stubBuilt(template, "5555555555555555555555555555555555555555555555555555555555555555")
	blockJSON, err := json.Marshal(built)
	if err != nil {
		t.Fatal(err)
	}
	rec := wcPublicationRecord{
		IntentHash: walletConnectIntentHash(template),
		State:      wcStatePublished,
		BlockJSON:  blockJSON,
		Hash:       built.Hash.String(),
		CreatedAt:  1,
	}
	if err := tx.wcJournal.put(wcJournalKey("replay-topic", 23), rec); err != nil {
		t.Fatal(err)
	}

	// Same identity + same intent: the stored result comes back and NO new
	// block/hold is ever created (restart-shaped replay).
	result, err := tx.PrepareWalletConnectSend(req)
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != "published" || result.Published == nil || result.Published["hash"] != built.Hash.String() {
		t.Fatalf("replay result = %+v", result)
	}
	if tx.pending != nil {
		t.Fatal("a replayed request must never create a fresh hold")
	}

	// Unknown state: surfaced for the reconcile flow, with a preview rebuilt
	// from the journaled block, still without a new hold.
	rec.State = wcStateSigned
	if err := tx.wcJournal.put(wcJournalKey("replay-topic", 23), rec); err != nil {
		t.Fatal(err)
	}
	result, err = tx.PrepareWalletConnectSend(req)
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != "unknown" || result.PublishedHash != built.Hash.String() {
		t.Fatalf("unknown replay result = %+v", result)
	}
	if result.Preview == nil || result.Preview.ToAddress != types.BridgeContract.String() || result.Preview.Amount != "100000000" {
		t.Fatalf("unknown replay must carry a preview from the journaled block: %+v", result.Preview)
	}
	if tx.pending != nil {
		t.Fatal("an unknown-outcome replay must never create a fresh hold")
	}

	// Same identity but DIFFERENT intent: fail closed.
	tampered := req
	tampered.AccountBlock.Amount = "200000000"
	if _, err := tx.PrepareWalletConnectSend(tampered); err == nil || !strings.Contains(err.Error(), "different") {
		t.Fatalf("got %v, want reused-identity refusal", err)
	}
}

func TestPrepareWalletConnectSendRequiresRequestIdentity(t *testing.T) {
	tx := newTestTxService(t)
	unlockTestWallet(t, tx.wallet)
	active, _ := tx.wallet.activeAddress()
	tx.node.chainID = mainnetChainID
	req, _ := validWalletConnectRequest(t)
	req.FromAddress = active.String()
	req.Topic = ""
	req.RequestID = 0
	if _, err := tx.PrepareWalletConnectSend(req); err == nil || !strings.Contains(err.Error(), "identity") {
		t.Fatalf("got %v, want missing-identity refusal", err)
	}
}

func TestConfirmPublishReenforcesMainnetOptInAfterPoW(t *testing.T) {
	tx := newTestTxService(t)
	holdID, _ := wcTestHold(t, tx, mainnetChainID, "topic-g", 29)
	if err := tx.config.SetAllowMainnetSend(true); err != nil {
		t.Fatal(err)
	}
	published := false
	tx.publishFn = func(_ *rpc_client.RpcClient, _ *nom.AccountBlock) error {
		published = true
		return nil
	}
	// WC-04: the opt-in is revoked while PoW runs; the publisher must never be
	// reached even though the early guard passed.
	tx.prepareBlockFn = func(_ *rpc_client.RpcClient, tpl *nom.AccountBlock, _ *sdkwallet.KeyPair) (*nom.AccountBlock, error) {
		if err := tx.config.SetAllowMainnetSend(false); err != nil {
			return nil, fmt.Errorf("flip opt-in: %w", err)
		}
		return stubBuilt(tpl, "6666666666666666666666666666666666666666666666666666666666666666"), nil
	}
	_, err := tx.ConfirmWalletConnectPublish(holdID)
	if err == nil || !strings.Contains(err.Error(), "mainnet") {
		t.Fatalf("got %v, want the revoked mainnet opt-in to refuse broadcast", err)
	}
	if published {
		t.Fatal("publisher was reached after the mainnet opt-in was revoked mid-PoW")
	}
	if tx.pending == nil {
		t.Fatal("the approved block must survive a guard refusal for an explicit retry")
	}
}
