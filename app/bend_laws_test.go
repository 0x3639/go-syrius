//go:build bendrules

package app

// These are differential tests, not a second Go implementation of the model.
// Expected values come from executing the checked-in Bend code. Fixtures use
// only public test keys and temporary directories; no live node or funds.
import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/0x3639/znn-sdk-go/rpc_client"
	sdkwallet "github.com/0x3639/znn-sdk-go/wallet"
	"github.com/zenon-network/go-zenon/chain/nom"
	"github.com/zenon-network/go-zenon/common/types"
)

type bendObservation struct{ name, expression, actual string }

func bendBool(b bool) string {
	if b {
		return "True{}"
	}
	return "False{}"
}
func bendBit(b bool) string {
	if b {
		return "1"
	}
	return "0"
}
func bendArgs(bs ...bool) string {
	s := make([]string, len(bs))
	for i, b := range bs {
		s[i] = bendBool(b)
	}
	return strings.Join(s, ", ")
}

func compareBend(t *testing.T, observations []bendObservation) {
	t.Helper()
	if len(observations) == 0 {
		t.Fatal("Bend harness: empty comparison")
	}
	root, err := filepath.Abs("../test/bend")
	if err != nil {
		t.Fatal(err)
	}
	scratch := filepath.Join(root, ".scratch")
	if err = os.MkdirAll(scratch, 0700); err != nil {
		t.Fatal(err)
	}
	dir, err := os.MkdirTemp(scratch, "trace-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	var src strings.Builder
	src.WriteString("import Base\nimport ../../policy/model.bend as P\nimport ../../pending/model.bend as H\nimport ../../journal/model.bend as J\nimport ../../lifecycle/model.bend as L\nimport ../../input/model.bend as I\nimport ../../recovery/model.bend as R\n\n")
	// Split expressions into defs to avoid deeply nested source terms.
	for i, o := range observations {
		fmt.Fprintf(&src, "def case_%d() -> String:\n  %s\n\n", i, o.expression)
	}
	src.WriteString("def main() -> +List<String>:\n  [")
	for i := range observations {
		if i > 0 {
			src.WriteString(", ")
		}
		fmt.Fprintf(&src, "case_%d()", i)
	}
	src.WriteString("]\n")
	if err = os.WriteFile(filepath.Join(dir, "main.bend"), []byte(src.String()), 0600); err != nil {
		t.Fatal(err)
	}
	image := os.Getenv("BEND_IMAGE")
	if image == "" {
		image = "go-syrius-bend:2.0.40"
	}
	name := fmt.Sprintf("syrius-bend-%d-%s", os.Getpid(), filepath.Base(dir))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "docker", "run", "--rm", "--name", name, "--network", "none", "--read-only", "--cap-drop", "ALL", "--security-opt", "no-new-privileges", "--pids-limit", "256", "--memory", "2g", "--tmpfs", "/tmp:rw,nosuid,size=256m", "-v", root+":/work:ro", image, ".scratch/"+filepath.Base(dir)+"/main.bend")
	raw, err := cmd.CombinedOutput()
	if err != nil {
		clean, stop := context.WithTimeout(context.Background(), 10*time.Second)
		defer stop()
		_ = exec.CommandContext(clean, "docker", "rm", "-f", name).Run()
		t.Fatalf("Bend harness execution error: %v\n%s", err, raw)
	}
	var expected []string
	if err = json.Unmarshal(raw, &expected); err != nil {
		t.Fatalf("Bend harness output error: %v\n%s", err, raw)
	}
	if len(expected) != len(observations) {
		t.Fatalf("Bend harness: got %d results, need %d", len(expected), len(observations))
	}
	for i, o := range observations {
		if expected[i] != o.actual {
			t.Errorf("BEND_MISMATCH %s: wallet=%q model=%q", o.name, o.actual, expected[i])
		}
	}
	t.Logf("compared %d observations with Bend", len(observations))
}

func TestBendEffect(t *testing.T) {
	var obs []bendObservation
	a := types.ParseAddressPanic("z1qzal6c5s9rjnnxd2z7dvdhjxpmmj4fmw56a0mz")
	other := types.ParseAddressPanic("z1qqjnwjjpnue8xmmpanz6csze6tcmtzzdtfsww7")
	// All combinations of sender, recipient, token, amount presence/value, data.
	// The amount exceeds Number.MAX_SAFE_INTEGER by many orders of magnitude.
	amount, _ := new(big.Int).SetString("123456789012345678901234567890", 10)
	for mask := 0; mask < 64; mask++ {
		bs := make([]bool, 6)
		for i := range bs {
			bs[i] = mask&(1<<i) != 0
		}
		b := &nom.AccountBlock{Address: a, ToAddress: other, TokenStandard: types.ZnnTokenStandard, Amount: new(big.Int).Set(amount), Data: []byte{1, 2, 3}}
		e := callExpect{from: a, to: other, zts: types.ZnnTokenStandard, amount: new(big.Int).Set(amount), data: []byte{1, 2, 3}}
		if !bs[0] {
			b.Address = other
		}
		if !bs[1] {
			b.ToAddress = a
		}
		if !bs[2] {
			b.TokenStandard = types.QsrTokenStandard
		}
		if !bs[4] {
			b.Amount.Add(b.Amount, big.NewInt(1))
		}
		if !bs[3] {
			b.Amount = nil
		}
		if !bs[5] {
			b.Data = []byte{1, 2, 4}
		}
		obs = append(obs, bendObservation{fmt.Sprintf("effect-%d", mask), "P.mark(P.effect(" + bendArgs(bs...) + "))", bendBit(assertMatches(b, e) == nil)})
	}
	b := &nom.AccountBlock{Amount: big.NewInt(1)}
	obs = append(obs, bendObservation{"nil-expected-amount", "P.mark(P.effect(True{}, True{}, True{}, False{}, True{}, True{}))", bendBit(assertMatches(b, callExpect{}) == nil)})
	compareBend(t, obs)
}

func TestBendPolicy(t *testing.T) {
	tx := newTestTxService(t)
	var obs []bendObservation
	for _, allow := range []bool{false, true} {
		if err := tx.config.SetAllowMainnetSend(allow); err != nil {
			t.Fatal(err)
		}
		for _, chain := range []uint64{0, 1, 3, 73404} {
			obs = append(obs, bendObservation{fmt.Sprintf("chain-%d-consent-%v", chain, allow), "P.mark(P.mainnet(" + bendArgs(chain == 1, allow) + "))", bendBit(tx.guardChain(chain) == nil)})
		}
	}
	for d := -2; d <= 25; d++ {
		_, err := boundTokenDecimals(d, types.ZnnTokenStandard)
		n := d
		if n < 0 {
			n = -n
		}
		obs = append(obs, bendObservation{fmt.Sprintf("decimals-%d", d), fmt.Sprintf("P.mark(P.decimals(%s, %dn))", bendBool(d < 0), n), bendBit(err == nil)})
	}
	now := time.Unix(100000, 0)
	for _, minutes := range []int{0, 1, 5, 15, 30} {
		for _, elapsed := range []int{0, 59, 60, 61, 299, 300, 301, 899, 900, 901, 1799, 1800, 1801, 3600} {
			tx.wallet.autoLockMins = minutes
			tx.wallet.lastActivity = now.Add(-time.Duration(elapsed) * time.Second)
			obs = append(obs, bendObservation{fmt.Sprintf("expiry-%d-%d", minutes, elapsed), fmt.Sprintf("P.mark(P.expired(%dn, %dn))", minutes*60, elapsed), bendBit(tx.wallet.autoLockExpired(now))})
		}
	}
	for _, method := range []string{"Wrap", "Redeem"} {
		for _, amount := range []int64{-1, 0, 1, 100000000} {
			for _, znn := range []bool{false, true} {
				req, active := validWalletConnectRequest(t)
				if method == "Redeem" {
					req, active = canonicalRedeemRequest(t)
				}
				req.AccountBlock.Amount = strconv.FormatInt(amount, 10)
				if !znn {
					req.AccountBlock.TokenStandard = types.QsrTokenStandard.String()
				}
				_, _, _, err := walletConnectBridgeTemplate(req, active)
				obs = append(obs, bendObservation{fmt.Sprintf("funding-%s-%d-%v", method, amount, znn), "P.mark(P.funding(P." + method + "{}, " + bendArgs(amount > 0, amount == 0, znn) + "))", bendBit(err == nil)})
			}
		}
	}
	compareBend(t, obs)
}

// Every prefix is visited, so the count is sum(alphabet^i, i=1..depth).
func bendSequences(t *testing.T, alphabet int, visit func([]int)) {
	t.Helper()
	depth := 3
	if s := os.Getenv("BEND_DEPTH"); s != "" {
		n, err := strconv.Atoi(s)
		if err != nil || n < 1 || n > 4 || strconv.Itoa(n) != s {
			t.Fatal("BEND_DEPTH must be 1..4 without leading zeros")
		}
		depth = n
	}
	var walk func([]int)
	walk = func(prefix []int) {
		if len(prefix) == depth {
			return
		}
		for i := 0; i < alphabet; i++ {
			seq := append(append([]int(nil), prefix...), i)
			visit(seq)
			walk(seq)
		}
	}
	walk(nil)
}

func TestBendPending(t *testing.T) {
	var obs []bendObservation
	names := []string{"Place{}", "CancelCurrent{}", "CancelStale{}", "CancelAll{}", "ClearCurrent{}", "ClearStale{}"}
	bendSequences(t, len(names), func(seq []int) {
		tx := &TxService{}
		var ops []string
		var actual strings.Builder
		for _, op := range seq {
			ops = append(ops, "H."+names[op])
			var err error
			switch op {
			case 0:
				_, err = tx.holdPending(&nom.AccountBlock{}, callExpect{}, 7)
			case 1:
				err = tx.CancelPending(tx.pendingHoldID)
			case 2:
				err = tx.CancelPending(tx.holdCounter + 1)
			case 3:
				err = tx.CancelPending(0)
			case 4:
				tx.clearPendingIf(tx.pendingHoldID)
			case 5:
				tx.clearPendingIf(tx.holdCounter + 1)
			}
			fmt.Fprintf(&actual, "%s:%d:%s:0;", bendBit(err == nil), tx.holdCounter, bendBit(tx.pending != nil))
			if tx.pending != nil && (tx.pendingHoldID == 0 || tx.pendingHoldID != tx.holdCounter) {
				t.Fatal("BEND_MISMATCH pending identity was lost")
			}
			if tx.pending == nil && (tx.pendingHoldID != 0 || tx.pendingWC != nil || tx.pendingGen != 0) {
				t.Fatal("BEND_MISMATCH cleanup left stale ownership")
			}
		}
		obs = append(obs, bendObservation{fmt.Sprint(seq), "H.trace([" + strings.Join(ops, ", ") + "], H.State{0n, False{}, False{}})", actual.String()})
	})
	// Exercise the actual cancellation gate with a publishing latch, independently
	// of the sequence alphabet. Post-sign interleavings are also checked below.
	for _, id := range []uint64{0, 1, 2} {
		tx := &TxService{pending: &nom.AccountBlock{}, pendingHoldID: 1, holdCounter: 1, publishingHold: 1}
		err := tx.CancelPending(id)
		obs = append(obs, bendObservation{fmt.Sprintf("busy-cancel-%d", id), "H.show_reply(H.cancel(H.State{1n, True{}, True{}}, " + bendArgs(id == 0, id == 1) + "))", fmt.Sprintf("%s:1:%s:1", bendBit(err == nil), bendBit(tx.pending != nil))})
	}
	compareBend(t, obs)
}

func TestBendJournal(t *testing.T) {
	var obs []bendObservation
	dir := t.TempDir()
	key := wcJournalKey("bend-topic", 1)
	names := []string{"Write{1n}", "Write{2n}", "Publish{}", "Ack{}", "Restart{}"}
	records := map[string]wcPublicationRecord{}
	for i := 1; i <= 2; i++ {
		hash := fmt.Sprintf("block-%d", i)
		records[hash] = wcPublicationRecord{Topic: "bend-topic", RequestID: 1, IntentHash: fmt.Sprintf("intent-%d", i), State: wcStateSigned, BlockJSON: json.RawMessage(fmt.Sprintf(`{"hash":"block-%d","amount":"%d","token":"ZNN","from":"sender-%d"}`, i, i, i)), Hash: hash, CreatedAt: int64(i)}
	}
	bendSequences(t, len(names), func(seq []int) {
		p := filepath.Join(dir, wcJournalFile)
		if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
		journal := newWCJournal(func() (string, error) { return dir, nil })
		var ops []string
		var actual strings.Builder
		for _, op := range seq {
			ops = append(ops, "J."+names[op])
			var err error
			switch op {
			case 0, 1:
				err = journal.put(key, records[fmt.Sprintf("block-%d", op+1)])
			case 2:
				err = journal.markPublished(key)
			case 3:
				err = journal.deletePublished(key)
			case 4:
				journal = newWCJournal(func() (string, error) { return dir, nil })
			}
			rec, found, readErr := journal.get(key)
			if readErr != nil {
				t.Fatal(readErr)
			}
			state := "absent"
			if found {
				want, ok := records[rec.Hash]
				if !ok {
					t.Fatal("BEND_MISMATCH journal changed the block identity")
				}
				want.State = rec.State
				// Persistence formats raw JSON. Compare semantic content separately.
				var gotJSON, wantJSON any
				if json.Unmarshal(rec.BlockJSON, &gotJSON) != nil || json.Unmarshal(want.BlockJSON, &wantJSON) != nil {
					t.Fatal("journal fixture JSON error")
				}
				if !reflect.DeepEqual(gotJSON, wantJSON) {
					t.Fatal("BEND_MISMATCH journal changed signed block bytes/content")
				}
				rec.BlockJSON = nil
				want.BlockJSON = nil
				if !reflect.DeepEqual(rec, want) {
					t.Fatal("BEND_MISMATCH journal changed payload or ownership")
				}
				state = string(rec.State) + ":" + strings.TrimPrefix(rec.Hash, "block-")
			}
			actual.WriteString(bendBit(err == nil) + ":" + state + ";")
		}
		obs = append(obs, bendObservation{fmt.Sprint(seq), "J.trace([" + strings.Join(ops, ", ") + "], J.Absent{})", actual.String()})
	})
	compareBend(t, obs)
}

func TestBendPublication(t *testing.T) {
	var obs []bendObservation
	cases := []string{"valid", "wrong-hold", "zero-hold", "session", "sender", "destination", "token", "amount", "data", "chain", "consent", "prepare-error", "journal-error", "cancel-in-flight", "concurrent-confirm", "repeat-confirm"}
	for _, name := range cases {
		tx := newTestTxService(t)
		id, _ := wcTestHold(t, tx, 1, "bend-topic", 1)
		if err := tx.config.SetAllowMainnetSend(true); err != nil {
			t.Fatal(err)
		}
		// identity, session, effect, chain, consent, preparation, journal durability.
		gates := []bool{true, true, true, true, true, true, true}
		confirmed := id
		if name == "wrong-hold" {
			confirmed = id + 1
			gates[0] = false
		}
		if name == "zero-hold" {
			confirmed = 0
			gates[0] = false
		}
		broadcasts := 0
		tx.prepareBlockFn = func(_ *rpc_client.RpcClient, tpl *nom.AccountBlock, _ *sdkwallet.KeyPair) (*nom.AccountBlock, error) {
			b := stubBuilt(tpl, strings.Repeat("6", 64))
			switch name {
			case "session":
				tx.wallet.gen++
				gates[1] = false
			case "sender":
				b.Address = types.ZeroAddress
				gates[2] = false
			case "destination":
				b.ToAddress = types.StakeContract
				gates[2] = false
			case "token":
				b.TokenStandard = types.QsrTokenStandard
				gates[2] = false
			case "amount":
				b.Amount = big.NewInt(1)
				gates[2] = false
			case "data":
				b.Data = []byte{1}
				gates[2] = false
			case "chain":
				b.ChainIdentifier = 3
				gates[3] = false
			case "consent":
				if err := tx.config.SetAllowMainnetSend(false); err != nil {
					t.Fatal(err)
				}
				gates[4] = false
			case "prepare-error":
				gates[5] = false
				return nil, fmt.Errorf("injected prepare failure")
			case "journal-error":
				gates[6] = false
				tx.wcJournal = newWCJournal(func() (string, error) { return "", fmt.Errorf("injected persistence failure") })
			case "cancel-in-flight":
				if tx.CancelPending(id) == nil || tx.CancelPending(0) == nil {
					t.Error("BEND_MISMATCH cancellation succeeded during publication")
				}
			case "concurrent-confirm":
				if _, err := tx.ConfirmPublish(id); err == nil {
					t.Error("BEND_MISMATCH concurrent confirmation accepted")
				}
			}
			return b, nil
		}
		tx.publishFn = func(_ *rpc_client.RpcClient, b *nom.AccountBlock) error {
			broadcasts++
			rec, found, err := tx.wcJournal.get(wcJournalKey("bend-topic", 1))
			if err != nil || !found || rec.State != wcStateSigned || rec.Hash != b.Hash.String() {
				t.Error("BEND_MISMATCH broadcast preceded durable signed record")
			}
			return nil
		}
		_, err := tx.ConfirmPublish(confirmed)
		obs = append(obs, bendObservation{name, "P.mark(P.publish(" + bendArgs(gates...) + "))", bendBit(broadcasts == 1 && err == nil)})
		// Rejection means no broadcast at all, not merely an error returned later.
		if err != nil && broadcasts != 0 {
			t.Errorf("BEND_MISMATCH %s broadcast despite refusing", name)
		}
		if name == "repeat-confirm" {
			if _, err := tx.ConfirmPublish(id); err == nil || broadcasts != 1 {
				t.Error("BEND_MISMATCH completed hold published twice")
			}
		}
	}
	compareBend(t, obs)
}

func TestBendReconnect(t *testing.T) {
	var obs []bendObservation
	// A syntactically invalid URL makes eligible attempts fail immediately,
	// without external networking. A retry=true proves the dial path was entered.
	for mask := 0; mask < 64; mask++ {
		bs := make([]bool, 6)
		for i := range bs {
			bs[i] = mask&(1<<i) != 0
		}
		n := newTestNode(t)
		n.mode = "embedded"
		n.connGen = 4
		h := &reconnectHandle{url: "://invalid"}
		n.embedded = h
		if !bs[0] {
			n.connGen = 5
		}
		if !bs[1] {
			n.mode = "remote"
		}
		if !bs[2] {
			n.embedded = nil
		}
		if !bs[3] {
			n.client = &rpc_client.RpcClient{}
		}
		n.embeddedStopping = !bs[4]
		n.embeddedWedged = !bs[5]
		_, retry := n.retryEmbeddedConnection(4)
		obs = append(obs, bendObservation{fmt.Sprintf("eligibility-%d", mask), "P.mark(P.reconnect(" + bendArgs(bs...) + "))", bendBit(retry)})
		if h.stops.Load() != 0 {
			t.Fatal("BEND_MISMATCH reconnect stopped embedded node")
		}
	}
	compareBend(t, obs)
}
