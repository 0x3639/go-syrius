//go:build bendrules

package app

import (
	"encoding/json"
	"fmt"
	"math/big"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/0x3639/znn-sdk-go/rpc_client"
	sdkwallet "github.com/0x3639/znn-sdk-go/wallet"
	"github.com/zenon-network/go-zenon/chain/nom"
	"github.com/zenon-network/go-zenon/common/types"
)

// HTTP JSON-RPC keeps the test on the real SDK parsing/signing/publication
// path. All traffic is loopback, with health checks and reconnects disabled.
func bendRPC(t *testing.T, handle func(string, []json.RawMessage) (any, error)) *rpc_client.RpcClient {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ID     json.RawMessage   `json:"id"`
			Method string            `json:"method"`
			Params []json.RawMessage `json:"params"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("RPC fixture decode: %v", err)
			http.Error(w, "bad request", 400)
			return
		}
		value, err := handle(req.Method, req.Params)
		response := map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": value}
		if err != nil {
			delete(response, "result")
			response["error"] = map[string]any{"code": -32000, "message": err.Error()}
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			t.Errorf("RPC fixture encode: %v", err)
		}
	}))
	t.Cleanup(server.Close)
	options := rpc_client.DefaultClientOptions()
	options.AutoReconnect = false
	options.HealthCheckInterval = 0
	client, err := rpc_client.NewRpcClientWithOptions(server.URL, options)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(client.Stop)
	return client
}

func bendRandom(t *testing.T) (*rand.Rand, int, int64) {
	t.Helper()
	seed := int64(1)
	count := 40
	if raw := os.Getenv("BEND_SEED"); raw != "" {
		v, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || v < 1 || strconv.FormatInt(v, 10) != raw {
			t.Fatal("BEND_SEED must be a positive signed 64-bit integer")
		}
		seed = v
	}
	if raw := os.Getenv("BEND_RANDOM"); raw != "" {
		v, err := strconv.Atoi(raw)
		if err != nil || v < 1 || v > 1000 || strconv.Itoa(v) != raw {
			t.Fatal("BEND_RANDOM must be 1..1000 without leading zeros")
		}
		count = v
	}
	return rand.New(rand.NewSource(seed)), count, seed
}

func TestBendLifecycle(t *testing.T) {
	names := []string{"Lock", "Unlock", "Switch", "Revoke", "Grant", "IdleTick"}
	histories := [][]int{{}, {0}, {3}, {0, 1}, {2}, {3, 4}, {2, 2}, {1}, {5}}
	rng, count, seed := bendRandom(t)
	for i := 0; i < count; i++ {
		seq := make([]int, 1+rng.Intn(24))
		for j := range seq {
			seq[j] = rng.Intn(len(names))
		}
		histories = append(histories, seq)
	}
	var observations []bendObservation
	for _, kind := range []string{"send", "receive"} {
		for caseIndex, history := range histories {
			tx := newTestTxService(t)
			unlockTestWallet(t, tx.wallet)
			if err := tx.config.SetAllowMainnetSend(true); err != nil {
				t.Fatal(err)
			}
			from, _ := tx.wallet.activeAddress()
			var broadcasts atomic.Int32
			var acted atomic.Bool
			// Execute the requested history after the SDK captured its signing key,
			// while the last pre-signing RPC is in flight. No sleeps or scheduler luck.
			act := func() {
				if !acted.CompareAndSwap(false, true) {
					return
				}
				for _, event := range history {
					switch event {
					case 0:
						if err := tx.wallet.Lock(); err != nil {
							t.Error(err)
						}
					case 1: // An already-unlocked wallet is unchanged; a new unlock is a new session.
						if _, ok := tx.wallet.activeAddress(); !ok {
							unlockTestWallet(t, tx.wallet)
						}
					case 2: // Match an account-selection generation change, without filesystem selection setup.
						tx.wallet.mu.Lock()
						tx.wallet.gen++
						tx.wallet.mu.Unlock()
					case 3:
						if err := tx.config.SetAllowMainnetSend(false); err != nil {
							t.Error(err)
						}
					case 4:
						if err := tx.config.SetAllowMainnetSend(true); err != nil {
							t.Error(err)
						}
					}
				}
			}
			source := types.HexToHashPanic(strings.Repeat("a", 64))
			client := bendRPC(t, func(method string, params []json.RawMessage) (any, error) {
				switch method {
				case "ledger.getFrontierAccountBlock":
					return nil, nil
				case "ledger.getFrontierMomentum":
					return map[string]any{"height": 1, "chainIdentifier": 1, "hash": strings.Repeat("b", 64)}, nil
				case "ledger.getAccountBlockByHash":
					return map[string]any{"hash": source.String(), "toAddress": from.String(), "amount": "1", "tokenStandard": types.ZnnTokenStandard.String()}, nil
				case "embedded.plasma.getRequiredPoWForAccountBlock":
					act()
					return map[string]any{"basePlasma": 21000, "availablePlasma": 21000, "requiredDifficulty": 0}, nil
				case "ledger.publishRawTransaction":
					broadcasts.Add(1)
					if len(params) != 1 {
						return nil, fmt.Errorf("invalid publication fixture")
					}
					var b nom.AccountBlock
					if err := json.Unmarshal(params[0], &b); err != nil {
						return nil, err
					}
					if b.Address != from || len(b.Signature) == 0 {
						t.Error("BEND_MISMATCH publication lost its signing identity")
					}
					return nil, nil
				default:
					return nil, fmt.Errorf("unexpected method %s", method)
				}
			})
			tx.node.client = client
			tx.node.chainID = 1
			var err error
			if kind == "receive" {
				_, err = tx.Receive(source.String())
			} else {
				b := client.LedgerApi.SendTemplate(types.BridgeContract, types.ZnnTokenStandard, big.NewInt(1), nil)
				b.Address = from
				b.ChainIdentifier = 1
				id := mustHoldPending(t, tx, b, callExpect{from: from, to: b.ToAddress, zts: b.TokenStandard, amount: big.NewInt(1)}, tx.wallet.sessionGen())
				_, err = tx.ConfirmPublish(id)
			}
			if !acted.Load() {
				t.Fatalf("lifecycle fixture did not reach checkpoint: %s: %v", kind, err)
			}
			if broadcasts.Load() > 1 || (broadcasts.Load() > 0 && err != nil) {
				t.Errorf("BEND_MISMATCH %s returned error after publication: %v", kind, err)
			}
			ops := make([]string, len(history))
			for i, event := range history {
				ops[i] = "L." + names[event] + "{}"
			}
			label := fmt.Sprintf("%s seed=%d case=%d history=%s", kind, seed, caseIndex, strings.Join(ops, ","))
			observations = append(observations, bendObservation{label, "P.mark(L.permit(L.run([" + strings.Join(ops, ",") + "], L.State{True{}, True{}, True{}})))", bendBit(broadcasts.Load() == 1 && err == nil)})
		}
	}
	compareBend(t, observations)
}

func TestBendMetadataIdentity(t *testing.T) {
	var observations []bendObservation
	zts := types.ParseZTSPanic(customZts)
	for _, returned := range []string{customZts, types.ZnnTokenStandard.String(), types.QsrTokenStandard.String(), types.ZeroTokenStandard.String(), "missing"} {
		for _, decimals := range []int{0, 6, 8, 18, 19, 255} {
			client := bendRPC(t, func(method string, _ []json.RawMessage) (any, error) {
				if method != "embedded.token.getByZts" {
					return nil, fmt.Errorf("unexpected method %s", method)
				}
				if returned == "missing" {
					return nil, nil
				}
				return map[string]any{"tokenStandard": returned, "decimals": decimals, "totalSupply": "1", "maxSupply": "1"}, nil
			})
			_, err := clientTokenDecimals(client)(zts)
			exists := returned != "missing" && returned != types.ZeroTokenStandard.String()
			observations = append(observations, bendObservation{fmt.Sprintf("metadata-%s-%d", returned, decimals), "P.mark(I.metadata(" + bendArgs(exists, returned == customZts, decimals <= 18) + "))", bendBit(err == nil)})
		}
	}
	compareBend(t, observations)
}

func TestBendCorruptJournal(t *testing.T) {
	valid := `{"topic":"topic","requestId":1,"intentHash":"intent","hash":"hash","state":"signed","blockJson":{"hash":"hash"}}`
	entry := `"topic#1":` + valid
	cases := []struct {
		name, body string
		valid      bool
	}{
		{"missing", "", true}, {"empty-object", "{}", true},
		{"null-document", "null", false}, {"truncated", "{", false},
		{"null-record", `{"topic#1":null}`, false},
		{"empty-record", `{"topic#1":{}}`, false},
		{"unknown-state", `{"topic#1":{"topic":"topic","requestId":1,"intentHash":"intent","hash":"hash","state":"maybe"}}`, false},
		{"unknown-state-with-payload", "{" + strings.Replace(entry, `"state":"signed"`, `"state":"maybe"`, 1) + "}", false},
		{"valid-record", "{" + entry + "}", true},
		{"legacy-ownership", `{"topic#1":` + strings.ReplaceAll(strings.ReplaceAll(valid, `"topic":"topic",`, ""), `"requestId":1,`, "") + "}", true},
		{"duplicate-key", "{" + entry + "," + entry + "}", false},
		{"foreign-topic", "{" + strings.Replace(entry, `"topic":"topic"`, `"topic":"other"`, 1) + "}", false},
		{"foreign-request", "{" + strings.Replace(entry, `"requestId":1`, `"requestId":2`, 1) + "}", false},
		{"noncanonical-key", "{" + strings.Replace(entry, `topic#1`, `topic#01`, 1) + "}", false},
		{"missing-payload", "{" + strings.Replace(entry, `,"blockJson":{"hash":"hash"}`, "", 1) + "}", false},
		{"different-payload-hash", "{" + strings.Replace(entry, `"blockJson":{"hash":"hash"}`, `"blockJson":{"hash":"other"}`, 1) + "}", false},
		{"trailing-document", "{" + entry + "}{}", false},
	}
	var observations []bendObservation
	for _, tc := range cases {
		dir := t.TempDir()
		if tc.body != "" {
			if err := os.WriteFile(filepath.Join(dir, wcJournalFile), []byte(tc.body), 0600); err != nil {
				t.Fatal(err)
			}
		}
		j := newWCJournal(func() (string, error) { return dir, nil })
		_, err := j.load()
		state := "Corrupt"
		if tc.valid {
			state = "Valid"
		}
		if tc.name == "missing" {
			state = "Missing"
		}
		observations = append(observations, bendObservation{tc.name, "P.mark(I.usable(I." + state + "{}))", bendBit(err == nil)})
		if !tc.valid {
			var rec wcPublicationRecord
			if err := json.Unmarshal([]byte(valid), &rec); err != nil {
				t.Fatal(err)
			}
			// A read error must not be followed by erasure or fresh publication.
			if j.put("topic#1", rec) == nil || j.deletePublished("topic#1") == nil {
				t.Errorf("BEND_MISMATCH %s: write/ack accepted corrupted storage", tc.name)
			}
			raw, err := os.ReadFile(filepath.Join(dir, wcJournalFile))
			if err != nil || string(raw) != tc.body {
				t.Errorf("BEND_MISMATCH %s: corrupt storage was overwritten: %v", tc.name, err)
			}
		}
	}
	compareBend(t, observations)
}

// These traces drive public publication/recovery methods and the real disk
// journal. Only signing and node responses use existing injected seams.
func TestBendRecovery(t *testing.T) {
	names := []string{"QueryError", "Found", "RetryAccepted", "RetryLost", "FoundSaveError", "RetrySaveError", "Ack", "Restart", "Revoke", "Grant"}
	histories := [][]int{{0, 6, 7, 3, 6}, {8, 2, 1, 6}, {5, 7, 6, 1, 6}, {4, 7, 6, 1}, {2, 2, 7, 6, 6}, {3, 3, 8, 3, 9, 2}}
	rng, count, seed := bendRandom(t)
	for i := 0; i < count; i++ {
		seq := make([]int, 1+rng.Intn(24))
		for j := range seq {
			seq[j] = rng.Intn(len(names))
		}
		histories = append(histories, seq)
	}
	var observations []bendObservation
	for caseIndex, history := range histories {
		tx := newTestTxService(t)
		id, template := wcTestHold(t, tx, 1, "topic", 1)
		if err := tx.config.SetAllowMainnetSend(true); err != nil {
			t.Fatal(err)
		}
		built := stubBuilt(template, strings.Repeat("7", 64))
		wantResult, err := walletConnectBlockJSON(built)
		if err != nil {
			t.Fatal(err)
		}
		tx.prepareBlockFn = func(_ *rpc_client.RpcClient, _ *nom.AccountBlock, _ *sdkwallet.KeyPair) (*nom.AccountBlock, error) {
			return built, nil
		}
		tx.publishFn = func(_ *rpc_client.RpcClient, _ *nom.AccountBlock) error { return fmt.Errorf("lost initial reply") }
		if _, err := tx.ConfirmWalletConnectPublish(id); err == nil {
			t.Fatal("BEND_MISMATCH initial lost reply was reported as success")
		}
		req, _ := validWalletConnectRequest(t)
		req.Topic, req.RequestID, req.FromAddress = "topic", 1, template.Address.String()
		original, found, err := tx.wcJournal.get("topic#1")
		if err != nil || !found || original.State != wcStateSigned {
			t.Fatalf("BEND_MISMATCH lost reply not retained: %v", err)
		}
		var actual strings.Builder
		ops := make([]string, len(history))
		for i, op := range history {
			ops[i] = "R." + names[op] + "{}"
			sent := 0
			tx.prepareBlockFn = func(_ *rpc_client.RpcClient, _ *nom.AccountBlock, _ *sdkwallet.KeyPair) (*nom.AccountBlock, error) {
				t.Error("BEND_MISMATCH recovery tried to build/sign another block")
				return nil, fmt.Errorf("recovery must reuse retained block")
			}
			tx.blockByHashFn = func(_ *rpc_client.RpcClient, hash types.Hash) (bool, error) {
				if hash != built.Hash {
					t.Error("BEND_MISMATCH recovery queried another hash")
				}
				if op == 0 {
					return false, fmt.Errorf("node unavailable")
				}
				return op == 1 || op == 4, nil
			}
			tx.publishFn = func(_ *rpc_client.RpcClient, b *nom.AccountBlock) error {
				sent++
				got, err := walletConnectBlockJSON(b)
				if err != nil || !reflect.DeepEqual(got, wantResult) {
					t.Error("BEND_MISMATCH recovery changed the retained block")
				}
				if op == 3 {
					return fmt.Errorf("lost rebroadcast reply")
				}
				return nil
			}
			originalDir := tx.wcJournal.dir
			if op == 4 || op == 5 {
				calls := 0
				tx.wcJournal.dir = func() (string, error) {
					calls++
					// Reconcile's get and markPublished's read work; saving fails.
					if calls == 3 {
						return "", fmt.Errorf("journal save unavailable")
					}
					return originalDir()
				}
			}
			var result map[string]interface{}
			var opErr error
			switch op {
			case 6:
				opErr = tx.AckWalletConnectResult("topic", 1)
			case 7:
				tx = newTxService(tx.config, tx.wallet, tx.node)
			case 8, 9:
				opErr = tx.config.SetAllowMainnetSend(op == 9)
			default:
				result, opErr = tx.ReconcileWalletConnectPublication("topic", 1)
			}
			tx.wcJournal.dir = originalDir
			if result != nil && !reflect.DeepEqual(result, wantResult) {
				t.Error("BEND_MISMATCH recovery returned a different result")
			}
			rec, exists, err := tx.wcJournal.get("topic#1")
			if err != nil {
				t.Fatal(err)
			}
			stage := "none"
			if exists {
				stage = "unknown"
				if rec.State == wcStatePublished {
					stage = "published"
				}
				// All retained fields except the explicitly changing state agree.
				copyRec := rec
				copyRec.State = original.State
				if !reflect.DeepEqual(copyRec, original) {
					t.Error("BEND_MISMATCH recovery changed retained journal content")
				}
			}
			replay, err := tx.LookupWalletConnectPublication(req)
			if err != nil || replay.Outcome != stage || (exists && (replay.Preview == nil || replay.Preview.HoldID != 0 || replay.PublishedHash != original.Hash || replay.JournalTopic != "topic" || replay.JournalRequestID != 1)) {
				t.Errorf("BEND_MISMATCH lookup disagrees with retained outcome: %+v %v", replay, err)
			}
			if exists {
				stage += ":1"
			}
			fmt.Fprintf(&actual, "%s:%d:%s;", bendBit(opErr == nil), sent, stage)
		}
		label := fmt.Sprintf("recovery seed=%d case=%d history=%s", seed, caseIndex, strings.Join(ops, ","))
		observations = append(observations, bendObservation{label, "R.trace([" + strings.Join(ops, ",") + "], R.State{R.Uncertain{1n}, True{}})", actual.String()})
	}
	compareBend(t, observations)
}
