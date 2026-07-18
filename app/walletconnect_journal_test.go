package app

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
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

func TestPrepareWalletConnectSendReplaysWithoutLiveWalletGates(t *testing.T) {
	// Round-2 finding 1: a redelivered request whose outcome is journaled must
	// resolve from the journal BEFORE the locked-wallet / configured-chain /
	// connected-node gates — funds may already have moved, and a gate error
	// would be reported to the dapp as an ordinary rejection.
	tx := newTestTxService(t)
	req, _ := validWalletConnectRequest(t)
	req.Topic = "gateless-topic"
	req.RequestID = 31
	from := types.ParseAddressPanic(req.FromAddress)
	template, _, _, err := walletConnectValidateIntent(req, from)
	if err != nil {
		t.Fatal(err)
	}
	built := stubBuilt(template, "7777777777777777777777777777777777777777777777777777777777777777")
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
	if err := tx.wcJournal.put(wcJournalKey("gateless-topic", 31), rec); err != nil {
		t.Fatal(err)
	}

	// Wallet locked, no configured chain gate satisfied, no node connected.
	result, err := tx.PrepareWalletConnectSend(req)
	if err != nil {
		t.Fatalf("locked-wallet replay must return the stored outcome, got %v", err)
	}
	if result.Outcome != "published" || result.Published["hash"] != built.Hash.String() {
		t.Fatalf("replay result = %+v", result)
	}
	if result.Preview == nil {
		t.Fatal("published replay must carry a preview for the delivery-retry dialog")
	}

	// A journaled identity with DIFFERENT intent still fails closed while locked.
	tampered := req
	tampered.AccountBlock.Amount = "31337"
	if _, err := tx.PrepareWalletConnectSend(tampered); err == nil || !strings.Contains(err.Error(), "different") {
		t.Fatalf("got %v, want reused-identity refusal", err)
	}
}

func TestWalletConnectJournalFailsClosedWhenFull(t *testing.T) {
	// Round-2 finding 2: every retained record is potential duplicate
	// protection; eviction is never allowed. A full journal refuses NEW writes
	// (which refuses new broadcasts) instead of dropping old outcomes.
	tx := newTestTxService(t)
	for i := 0; i < wcJournalMaxRecords; i++ {
		rec := wcPublicationRecord{IntentHash: "x", State: wcStatePublished, Hash: "00", CreatedAt: int64(i + 1)}
		if err := tx.wcJournal.put(wcJournalKey("t", uint64(i+1)), rec); err != nil {
			t.Fatal(err)
		}
	}
	err := tx.wcJournal.put(wcJournalKey("overflow", 1), wcPublicationRecord{IntentHash: "y", State: wcStateSigned, Hash: "01", CreatedAt: 99})
	if err == nil {
		t.Fatal("a full journal must refuse new records, never evict old outcomes")
	}
	all, loadErr := tx.wcJournal.load()
	if loadErr != nil {
		t.Fatal(loadErr)
	}
	if len(all) != wcJournalMaxRecords {
		t.Fatalf("journal has %d records after refused write, want %d intact", len(all), wcJournalMaxRecords)
	}
	if _, ok := all[wcJournalKey("t", 1)]; !ok {
		t.Fatal("oldest record was evicted by the refused write")
	}
	// Overwriting an EXISTING key (signed -> published) must still work at cap.
	if err := tx.wcJournal.put(wcJournalKey("t", 1), wcPublicationRecord{IntentHash: "x", State: wcStateSigned, Hash: "00", CreatedAt: 1}); err != nil {
		t.Fatalf("updating an existing record at cap must succeed: %v", err)
	}
}

func TestReconcileWalletConnectRefusesWhileAnotherPublishIsInFlight(t *testing.T) {
	// Round-2 finding 4: reconciliation rebroadcasts on the same account
	// frontier as normal sends/receives; it must serialize under publishMu.
	tx := newTestTxService(t)
	holdID, _ := wcTestHold(t, tx, 3, "topic-lock", 37)
	tx.prepareBlockFn = func(_ *rpc_client.RpcClient, tpl *nom.AccountBlock, _ *sdkwallet.KeyPair) (*nom.AccountBlock, error) {
		return stubBuilt(tpl, "8888888888888888888888888888888888888888888888888888888888888888"), nil
	}
	tx.publishFn = func(_ *rpc_client.RpcClient, _ *nom.AccountBlock) error { return errors.New("timeout") }
	if _, err := tx.ConfirmWalletConnectPublish(holdID); err == nil {
		t.Fatal("expected outcome-unknown error")
	}
	tx.publishMu.Lock()
	defer tx.publishMu.Unlock()
	if _, err := tx.ReconcileWalletConnectPublication("topic-lock", 37); err == nil || !strings.Contains(err.Error(), "already being published") {
		t.Fatalf("got %v, want publish-serialization refusal", err)
	}
}

func TestReconcileWalletConnectRefusesChainMismatchBeforeQuery(t *testing.T) {
	// Round-2 finding 4: a "not found" answer from a node on ANOTHER chain is
	// not evidence of non-publication, and rebroadcasting there is wrong.
	tx := newTestTxService(t)
	holdID, _ := wcTestHold(t, tx, 3, "topic-chain", 41)
	tx.prepareBlockFn = func(_ *rpc_client.RpcClient, tpl *nom.AccountBlock, _ *sdkwallet.KeyPair) (*nom.AccountBlock, error) {
		return stubBuilt(tpl, "9999999999999999999999999999999999999999999999999999999999999999"), nil
	}
	tx.publishFn = func(_ *rpc_client.RpcClient, _ *nom.AccountBlock) error { return errors.New("timeout") }
	if _, err := tx.ConfirmWalletConnectPublish(holdID); err == nil {
		t.Fatal("expected outcome-unknown error")
	}
	tx.node.chainID = 12 // node switched networks since the block was signed
	queried := false
	tx.blockByHashFn = func(_ *rpc_client.RpcClient, _ types.Hash) (bool, error) { queried = true; return false, nil }
	published := false
	tx.publishFn = func(_ *rpc_client.RpcClient, _ *nom.AccountBlock) error { published = true; return nil }
	if _, err := tx.ReconcileWalletConnectPublication("topic-chain", 41); err == nil || !strings.Contains(err.Error(), "chain") {
		t.Fatalf("got %v, want chain-mismatch refusal", err)
	}
	if queried || published {
		t.Fatalf("wrong-chain node was consulted (queried=%v published=%v)", queried, published)
	}
	rec, ok, _ := tx.wcJournal.get(wcJournalKey("topic-chain", 41))
	if !ok || rec.State != wcStateSigned {
		t.Fatalf("record must stay signed/unknown after a chain refusal: %+v", rec)
	}
}

func TestLookupWalletConnectPublicationIsGateless(t *testing.T) {
	// Round-3 finding 1: the lookup-only method resolves a journaled outcome
	// with a locked wallet and no node, and never creates a hold.
	tx := newTestTxService(t)
	req, _ := validWalletConnectRequest(t)
	req.Topic = "lookup-topic"
	req.RequestID = 61
	from := types.ParseAddressPanic(req.FromAddress)
	template, _, _, err := walletConnectValidateIntent(req, from)
	if err != nil {
		t.Fatal(err)
	}
	built := stubBuilt(template, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	blockJSON, _ := json.Marshal(built)
	if err := tx.wcJournal.put(wcJournalKey("lookup-topic", 61), wcPublicationRecord{
		IntentHash: walletConnectIntentHash(template), State: wcStatePublished, BlockJSON: blockJSON, Hash: built.Hash.String(), CreatedAt: 1,
	}); err != nil {
		t.Fatal(err)
	}

	// Wallet locked, no node connected — lookup still replays.
	res, err := tx.LookupWalletConnectPublication(req)
	if err != nil {
		t.Fatalf("gateless lookup failed: %v", err)
	}
	if res.Outcome != "published" || res.Published["hash"] != built.Hash.String() {
		t.Fatalf("lookup result = %+v", res)
	}
	if tx.pending != nil {
		t.Fatal("lookup must never create a hold")
	}

	// A genuinely fresh request (new id AND a different intent) is Outcome none
	// so the fresh path runs.
	fresh := req
	fresh.RequestID = 62
	fresh.AccountBlock.Amount = "500000000"
	if res, err := tx.LookupWalletConnectPublication(fresh); err != nil || res.Outcome != "none" {
		t.Fatalf("unjournaled lookup = %+v, %v; want none, nil", res, err)
	}

	// Reused id + different intent is a resolved "conflict" outcome, NOT a Go
	// error — a journal READ failure must stay distinguishable from a
	// deliberate reuse refusal so the frontend can classify each correctly.
	tampered := req
	tampered.AccountBlock.Amount = "999"
	res, err = tx.LookupWalletConnectPublication(tampered)
	if err != nil {
		t.Fatalf("reused-intent lookup must not be a Go error: %v", err)
	}
	if res.Outcome != "conflict" {
		t.Fatalf("reused-intent lookup outcome = %q, want conflict", res.Outcome)
	}
}

func TestNodeConnectionSnapshotReadsClientAndChainTogether(t *testing.T) {
	tx := newTestTxService(t)
	tx.node.client = &rpc_client.RpcClient{}
	tx.node.chainID = 7
	client, chain := tx.node.connectionSnapshot()
	if client == nil || chain != 7 {
		t.Fatalf("snapshot = %v, %d; want non-nil client and chain 7", client, chain)
	}
}

func journalSignedRecord(t *testing.T, tx *TxService, topic string, reqID uint64, req WalletConnectSendRequest, state wcPublicationState, hashHex string) *nom.AccountBlock {
	t.Helper()
	from := types.ParseAddressPanic(req.FromAddress)
	template, _, _, err := walletConnectValidateIntent(req, from)
	if err != nil {
		t.Fatal(err)
	}
	built := stubBuilt(template, hashHex)
	blockJSON, err := json.Marshal(built)
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.wcJournal.put(wcJournalKey(topic, reqID), wcPublicationRecord{
		Topic: topic, RequestID: reqID, IntentHash: walletConnectIntentHash(template),
		State: state, BlockJSON: blockJSON, Hash: built.Hash.String(), CreatedAt: 1,
	}); err != nil {
		t.Fatal(err)
	}
	return built
}

func TestLookupMatchesRetainedUnresolvedIntentUnderNewID(t *testing.T) {
	// P1: a retained UNRESOLVED (signed) record must block a fresh publication
	// of the identical intent reissued under a NEW request id in the same
	// session — otherwise a second block could be signed for the same transfer.
	tx := newTestTxService(t)
	req, _ := validWalletConnectRequest(t)
	req.Topic = "sess"
	req.RequestID = 100
	journalSignedRecord(t, tx, "sess", 100, req, wcStateSigned, "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc")

	newReq := req
	newReq.RequestID = 101 // same intent, new id
	res, err := tx.LookupWalletConnectPublication(newReq)
	if err != nil {
		t.Fatal(err)
	}
	if res.Outcome == "none" {
		t.Fatal("a new id with a retained matching intent resolved to none; a duplicate block could be built")
	}
	if res.Outcome != "unknown" {
		t.Fatalf("outcome = %q, want unknown (unresolved match)", res.Outcome)
	}
	if res.JournalTopic != "sess" || res.JournalRequestID != 100 {
		t.Fatalf("result must carry the ORIGINAL journal key for reconcile/ack, got %s#%d", res.JournalTopic, res.JournalRequestID)
	}

	// Prepare must not build a fresh hold for the duplicate — the intent-matched
	// replay resolves before any wallet/node gate, so no setup is needed.
	out, err := tx.PrepareWalletConnectSend(newReq)
	if err != nil {
		t.Fatalf("prepare of a matched-intent new id must return the replay, not error: %v", err)
	}
	if out.Outcome != "unknown" {
		t.Fatalf("prepare outcome = %q, want unknown replay", out.Outcome)
	}
	if tx.pending != nil {
		t.Fatal("a duplicate-intent new id must NEVER create a fresh hold")
	}
}

func TestLookupMatchesRetainedPublishedIntentUnderNewID(t *testing.T) {
	tx := newTestTxService(t)
	req, _ := validWalletConnectRequest(t)
	req.Topic = "sess"
	req.RequestID = 200
	built := journalSignedRecord(t, tx, "sess", 200, req, wcStatePublished, "dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd")

	newReq := req
	newReq.RequestID = 201
	res, err := tx.LookupWalletConnectPublication(newReq)
	if err != nil {
		t.Fatal(err)
	}
	if res.Outcome != "published" || res.Published["hash"] != built.Hash.String() {
		t.Fatalf("published intent match = %+v", res)
	}
	if res.JournalTopic != "sess" || res.JournalRequestID != 200 {
		t.Fatalf("published match must carry the original key, got %s#%d", res.JournalTopic, res.JournalRequestID)
	}
}

func TestLookupDoesNotMatchDifferentIntentOrTopic(t *testing.T) {
	tx := newTestTxService(t)
	req, _ := validWalletConnectRequest(t)
	req.Topic = "sess"
	req.RequestID = 300
	journalSignedRecord(t, tx, "sess", 300, req, wcStateSigned, "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee")

	// Different amount under a new id → no match (genuinely fresh).
	diff := req
	diff.RequestID = 301
	diff.AccountBlock.Amount = "200000000"
	if res, err := tx.LookupWalletConnectPublication(diff); err != nil || res.Outcome != "none" {
		t.Fatalf("different intent = %+v, %v; want none", res, err)
	}
	// Same intent under a DIFFERENT topic (session) → a blocking "duplicate"
	// outcome (fail closed: not auto-replayed to a possibly-unrelated dapp, but
	// still not a fresh publication).
	otherTopic := req
	otherTopic.Topic = "other-sess"
	otherTopic.RequestID = 302
	res, err := tx.LookupWalletConnectPublication(otherTopic)
	if err != nil {
		t.Fatal(err)
	}
	if res.Outcome != "duplicate" {
		t.Fatalf("cross-topic same intent outcome = %q, want duplicate", res.Outcome)
	}
	if res.JournalTopic != "sess" || res.JournalRequestID != 300 {
		t.Fatalf("duplicate must point at the retained record %s#%d", res.JournalTopic, res.JournalRequestID)
	}
	// It must never build a fresh hold.
	if _, err := tx.PrepareWalletConnectSend(otherTopic); err != nil {
		t.Fatalf("prepare of a cross-topic duplicate must return the blocking outcome, not error: %v", err)
	}
	if tx.pending != nil {
		t.Fatal("a cross-topic duplicate must NEVER create a fresh hold")
	}
}

func TestLookupBackfillsOwnershipForLegacyRecords(t *testing.T) {
	// P1: records written before the topic/requestId fields existed must still
	// participate in intent matching (their ownership is derived from the key),
	// or the new-id duplicate bypass persists for records already on disk.
	tx := newTestTxService(t)
	req, _ := validWalletConnectRequest(t)
	req.Topic = "legacy-sess"
	req.RequestID = 400
	from := types.ParseAddressPanic(req.FromAddress)
	template, _, _, err := walletConnectValidateIntent(req, from)
	if err != nil {
		t.Fatal(err)
	}
	built := stubBuilt(template, "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff")
	blockJSON, _ := json.Marshal(built)

	// Write a LEGACY-shaped record: no topic/requestId fields.
	dir, err := tx.config.dataDir()
	if err != nil {
		t.Fatal(err)
	}
	legacy := map[string]wcPublicationRecord{
		wcJournalKey("legacy-sess", 400): {
			IntentHash: walletConnectIntentHash(template),
			State:      wcStateSigned,
			BlockJSON:  blockJSON,
			Hash:       built.Hash.String(),
			CreatedAt:  1,
		},
	}
	raw, _ := json.MarshalIndent(legacy, "", "  ")
	if err := os.WriteFile(filepath.Join(dir, wcJournalFile), raw, 0o600); err != nil {
		t.Fatal(err)
	}

	// Same intent, NEW id → must match the legacy record (not none).
	newReq := req
	newReq.RequestID = 401
	res, err := tx.LookupWalletConnectPublication(newReq)
	if err != nil {
		t.Fatal(err)
	}
	if res.Outcome != "unknown" || res.JournalRequestID != 400 {
		t.Fatalf("legacy-record intent match = %+v; want unknown owned by 400", res)
	}
}
