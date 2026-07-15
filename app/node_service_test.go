package app

import (
	"errors"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/0x3639/go-syrius/internal/governance"
	"github.com/0x3639/znn-sdk-go/rpc_client"
	"github.com/zenon-network/go-zenon/chain/nom"
	"github.com/zenon-network/go-zenon/common/types"
	api "github.com/zenon-network/go-zenon/rpc/api"
)

type stubHandle struct{ url, dir string }

func (s stubHandle) WSURL() string   { return s.url }
func (s stubHandle) DataDir() string { return s.dir }
func (s stubHandle) Stop() error     { return nil }

func TestToTokenBalance(t *testing.T) {
	bi := &api.BalanceInfo{
		TokenInfo: &api.Token{ZenonTokenStandard: types.ZnnTokenStandard, TokenSymbol: "ZNN", Decimals: 8},
		Balance:   big.NewInt(5000000000000),
	}
	got := toTokenBalance(types.ZnnTokenStandard, bi)
	if got.Symbol != "ZNN" || got.Decimals != 8 || got.Amount != "5000000000000" {
		t.Fatalf("toTokenBalance = %+v", got)
	}
	if got.Zts != types.ZnnTokenStandard.String() {
		t.Fatalf("zts = %s", got.Zts)
	}
}

func TestToTxRecordDirection(t *testing.T) {
	send := &api.AccountBlock{}
	send.AccountBlock = nom.AccountBlock{
		Hash:          types.HexToHashPanic("0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20"),
		BlockType:     nom.BlockTypeUserSend,
		ToAddress:     types.ParseAddressPanic("z1qqjnwjjpnue8xmmpanz6csze6tcmtzzdtfsww7"),
		Amount:        big.NewInt(100000000),
		TokenStandard: types.ZnnTokenStandard,
	}
	recs := blockToRecords(send, newDecimalsCache(nil))
	if len(recs) != 1 || recs[0].Direction != "out" {
		t.Fatalf("send -> %+v, want one out record", recs)
	}
	if recs[0].Amount != "100000000" || recs[0].Confirmed || recs[0].Decimals != 8 {
		t.Fatalf("rec = %+v", recs[0])
	}
}

func TestBlockToRecordsReceiveEmitsInAndPair(t *testing.T) {
	const sender = "z1qzal6c5s9rjnnxd2z7dvdhjxpmmj4fmw56a0mz"
	const me = "z1qqjnwjjpnue8xmmpanz6csze6tcmtzzdtfsww7"
	// A receive block carries amount 0 / the zero ZTS; the value lives in its pair.
	recv := &api.AccountBlock{}
	recv.AccountBlock = nom.AccountBlock{
		Hash:      types.HexToHashPanic("0202030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20"),
		BlockType: nom.BlockTypeUserReceive,
		Address:   types.ParseAddressPanic(me),
	}
	paired := &api.AccountBlock{}
	paired.AccountBlock = nom.AccountBlock{
		Hash:          types.HexToHashPanic("0303030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20"),
		Address:       types.ParseAddressPanic(sender),
		ToAddress:     types.ParseAddressPanic(me),
		Amount:        big.NewInt(500000000),
		TokenStandard: types.ZnnTokenStandard,
	}
	recv.PairedAccountBlock = paired

	recs := blockToRecords(recv, newDecimalsCache(nil))
	if len(recs) != 2 {
		t.Fatalf("receive -> %d records, want 2 (in + pair)", len(recs))
	}
	in, pair := recs[0], recs[1]
	if in.Direction != "in" || in.Amount != "500000000" || in.Token != types.ZnnTokenStandard.String() || in.Counterparty != sender {
		t.Fatalf("in row = %+v", in)
	}
	if pair.Direction != "pair" || pair.Amount != "0" || pair.Token != "" {
		t.Fatalf("pair row = %+v (want pair/0/empty-token)", pair)
	}
}

func TestToUnreceivedBlock(t *testing.T) {
	b := &api.AccountBlock{}
	b.AccountBlock = nom.AccountBlock{
		Hash:          types.HexToHashPanic("0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20"),
		Address:       types.ParseAddressPanic("z1qqjnwjjpnue8xmmpanz6csze6tcmtzzdtfsww7"),
		Amount:        big.NewInt(150000000),
		TokenStandard: types.ZnnTokenStandard,
	}
	got := toUnreceivedBlock(b, 6)
	if got.FromAddress != b.Address.String() || got.Amount != "150000000" || got.Decimals != 6 {
		t.Fatalf("toUnreceivedBlock = %+v", got)
	}
	if got.FromHash != b.Hash.String() {
		t.Fatalf("fromHash = %s", got.FromHash)
	}
}

func TestStatusDefaults(t *testing.T) {
	n := newNodeService(newConfigService(), newWalletService(newConfigService()))
	s := n.NodeStatus()
	if s.Connected || s.Mode != "remote" {
		t.Fatalf("status = %+v", s)
	}
}

func TestNodeStatusReportsChainID(t *testing.T) {
	n := newTestNode(t)
	n.mu.Lock()
	n.chainID = 42
	n.mu.Unlock()
	if got := n.NodeStatus().ChainID; got != 42 {
		t.Fatalf("NodeStatus().ChainID = %d, want 42", got)
	}
}

func newTestNode(t *testing.T) *NodeService {
	t.Helper()
	return newNodeService(newTestConfig(t), nil)
}

func TestSetNodeModeRejectsUnknown(t *testing.T) {
	n := newTestNode(t)
	if err := n.SetNodeMode("bogus"); err == nil {
		t.Fatal("expected unknown mode to error")
	}
}

func TestSetNodeModePersistsEvenIfUnreachable(t *testing.T) {
	n := newTestNode(t)
	// No local node is running; the connect attempt fails, but the chosen mode
	// must still be persisted (user intent), and reflected by GetNodeConfig.
	_ = n.SetNodeMode("local") // connect error expected and ignored here
	cfg, err := n.GetNodeConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Mode != "local" {
		t.Fatalf("mode should persist as local, got %q", cfg.Mode)
	}
	if n.NodeStatus().Mode != "local" {
		t.Fatalf("NodeStatus().Mode should be local, got %q", n.NodeStatus().Mode)
	}
}

func TestSetNodeURLValidatesAndPersists(t *testing.T) {
	n := newTestNode(t)
	if err := n.SetNodeURL("bogus", "ws://x"); err == nil {
		t.Fatal("expected unknown mode to error")
	}
	if err := n.SetNodeURL("local", "http://x"); err == nil {
		t.Fatal("expected non-ws scheme to error")
	}
	// Setting the non-active mode's URL persists without a reconnect (no error).
	if err := n.SetNodeURL("local", "ws://127.0.0.1:9"); err != nil {
		t.Fatalf("SetNodeURL(local): %v", err)
	}
	cfg, _ := n.GetNodeConfig()
	if cfg.LocalURL != "ws://127.0.0.1:9" {
		t.Fatalf("LocalURL not persisted: %q", cfg.LocalURL)
	}
}

func TestSetNodeFailedConnectLeavesCleanStatus(t *testing.T) {
	n := newTestNode(t)
	// Unreachable address: the connect/reachability check fails. After a failed
	// connect the status must be a clean disconnected state (not stale).
	err := n.setNode("ws://127.0.0.1:1")
	if err == nil {
		t.Fatal("expected SetNode to fail against an unreachable node")
	}
	st := n.NodeStatus()
	if st.Connected {
		t.Fatalf("status should be disconnected after failed connect, got %+v", st)
	}
	if st.Height != 0 {
		t.Fatalf("height should be 0 after failed connect, got %d", st.Height)
	}
}

func TestSetNodeURLStrictValidation(t *testing.T) {
	n := newTestNode(t)
	if err := n.SetNodeURL("remote", "ws://"); err == nil {
		t.Fatal("expected ws:// with no host to error")
	}
	if err := n.SetNodeURL("remote", "wss:// "); err == nil {
		t.Fatal("expected wss:// with trailing space to error")
	}
	if err := n.SetNodeURL("remote", "not-a-url"); err == nil {
		t.Fatal("expected non-url to error")
	}
	// Success on the non-active mode (local) so it persists without connecting.
	if err := n.SetNodeURL("local", "wss://host.example:35998"); err != nil {
		t.Fatalf("SetNodeURL(local, valid): %v", err)
	}
	cfg, err := n.GetNodeConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.LocalURL != "wss://host.example:35998" {
		t.Fatalf("LocalURL not persisted: %q", cfg.LocalURL)
	}
}

func TestSetNodeURLRejectsEmbedded(t *testing.T) {
	n := newTestNode(t)
	if err := n.SetNodeURL("embedded", "ws://127.0.0.1:35998"); err == nil {
		t.Fatal("embedded URL is fixed; SetNodeURL must reject mode embedded")
	}
}

func TestSetNodeModeEmbeddedPersistsAndStarts(t *testing.T) {
	n := newTestNode(t)
	started := false
	// Stub the starter so no real node is spun up; return a handle whose URL is
	// unreachable so the subsequent connect fails — mode must still persist.
	n.embeddedStart = func(dataDir string) (embeddedHandle, error) {
		started = true
		return stubHandle{url: "ws://127.0.0.1:1", dir: dataDir}, nil
	}
	_ = n.SetNodeMode("embedded") // connect will fail (unreachable); ignore
	if !started {
		t.Fatal("embedded starter not invoked")
	}
	cfg, _ := n.GetNodeConfig()
	if cfg.Mode != "embedded" {
		t.Fatalf("mode should persist embedded, got %q", cfg.Mode)
	}
}

func TestStartEmbeddedTearsDownOnConnectFailure(t *testing.T) {
	n := newTestNode(t)
	// Stub the starter to return an unreachable handle so SetNode fails.
	n.embeddedStart = func(dataDir string) (embeddedHandle, error) {
		return stubHandle{url: "ws://127.0.0.1:1", dir: dataDir}, nil
	}
	if err := n.SetNodeMode("embedded"); err == nil {
		t.Fatal("expected SetNodeMode to return connect error")
	}
	// Teardown must have cleared n.embedded so a Retry can start fresh.
	info, err := n.GetEmbeddedInfo()
	if err != nil {
		t.Fatal(err)
	}
	if info.Running {
		t.Fatal("embedded node should have been torn down after connect failure")
	}
}

func TestConnectStartsEmbeddedWhenModePersisted(t *testing.T) {
	n := newTestNode(t)
	// Persist embedded mode as if a prior session selected it.
	if err := n.config.updateSettings(func(s *Settings) error {
		s.NodeMode = "embedded"
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	started := false
	// Stub starter returns an unreachable handle so the connect fails; we only
	// assert that Connect() started the embedded node.
	n.embeddedStart = func(dataDir string) (embeddedHandle, error) {
		started = true
		return stubHandle{url: "ws://127.0.0.1:1", dir: dataDir}, nil
	}
	_ = n.Connect() // connect will fail (unreachable); ignore
	if !started {
		t.Fatal("Connect() did not start embedded node when embedded mode persisted")
	}
}

func TestDeleteEmbeddedData(t *testing.T) {
	n := newTestNode(t)
	dir, _ := n.config.dataDir()
	emb := filepath.Join(dir, "embedded")
	if err := os.MkdirAll(emb, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(emb, "x"), []byte("data"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := n.DeleteEmbeddedData(); err != nil {
		t.Fatalf("DeleteEmbeddedData: %v", err)
	}
	if _, err := os.Stat(emb); !os.IsNotExist(err) {
		t.Fatal("embedded dir should be gone")
	}
	// absent dir is fine
	if err := n.DeleteEmbeddedData(); err != nil {
		t.Fatalf("delete absent: %v", err)
	}
}

func TestGetEmbeddedInfoSize(t *testing.T) {
	n := newTestNode(t)
	dir, _ := n.config.dataDir()
	emb := filepath.Join(dir, "embedded")
	os.MkdirAll(emb, 0o700)
	os.WriteFile(filepath.Join(emb, "x"), make([]byte, 1234), 0o600)
	info, err := n.GetEmbeddedInfo()
	if err != nil {
		t.Fatal(err)
	}
	if info.SizeBytes < 1234 {
		t.Fatalf("size = %d", info.SizeBytes)
	}
	if info.Running {
		t.Fatal("not running")
	}
}

func TestGetNodeConfigDefaults(t *testing.T) {
	n := newTestNode(t)
	cfg, err := n.GetNodeConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Mode != "remote" || cfg.RemoteURL != defaultNodeURL || cfg.LocalURL != defaultLocalNodeURL {
		t.Fatalf("unexpected node config: %+v", cfg)
	}
}

func TestGetTransactionsRejectsNegativePaging(t *testing.T) {
	n := newTestNode(t) // existing helper: newNodeService(newTestConfig(t), nil)
	if _, err := n.GetTransactions(-1, 10); err == nil {
		t.Fatal("negative page must be rejected")
	} else if !strings.Contains(err.Error(), "non-negative") {
		t.Fatalf("negative page must be rejected with a non-negative message, got %v", err)
	}
	if _, err := n.GetTransactions(0, -5); err == nil {
		t.Fatal("negative count must be rejected")
	} else if !strings.Contains(err.Error(), "non-negative") {
		t.Fatalf("negative count must be rejected with a non-negative message, got %v", err)
	}
}

func TestSupersededDialDoesNotInstall(t *testing.T) {
	n := newTestNode(t)
	// First connect intent captures its generation (as SetNode does).
	n.mu.Lock()
	n.disconnectLocked()
	gen := n.connGen
	n.mu.Unlock()

	// A newer intent (SetNode/Disconnect) arrives while the first dial is slow.
	n.mu.Lock()
	n.disconnectLocked()
	cur := n.connGen
	n.mu.Unlock()

	if n.installConnection(&rpc_client.RpcClient{}, nil, nil, "ws://stale", 1, 3, gen) {
		t.Fatal("a superseded dial must not install its client over the newer one")
	}
	if n.currentClient() != nil {
		t.Fatal("the stale install must leave no client behind")
	}

	// The latest intent still installs normally.
	fresh := &rpc_client.RpcClient{}
	governanceAPI := governance.NewAPI(nil)
	if !n.installConnection(fresh, nil, governanceAPI, "ws://fresh", 9, 3, cur) {
		t.Fatal("the current dial must install")
	}
	if n.currentClient() != fresh {
		t.Fatal("expected the fresh client to be installed")
	}
	if n.currentChainID() != 3 {
		t.Fatalf("chainID not installed, got %d", n.currentChainID())
	}
	if n.currentGovernance() != governanceAPI {
		t.Fatal("expected the matching governance adapter to be installed")
	}
}

func TestDisconnectInvalidatesInFlightDial(t *testing.T) {
	n := newTestNode(t)
	n.mu.Lock()
	n.disconnectLocked()
	gen := n.connGen
	n.mu.Unlock()

	if err := n.Disconnect(); err != nil {
		t.Fatalf("Disconnect: %v", err)
	}
	if n.installConnection(&rpc_client.RpcClient{}, nil, nil, "ws://late", 1, 3, gen) {
		t.Fatal("a dial that loses to an explicit Disconnect must not install")
	}
}

func TestStartMomentumLoopSupersededIsNoop(t *testing.T) {
	n := newTestNode(t)
	n.mu.Lock()
	n.disconnectLocked()
	gen := n.connGen
	n.disconnectLocked() // superseded before the loop starts
	n.mu.Unlock()
	if err := n.startMomentumLoop(gen); err != nil {
		t.Fatalf("a superseded loop start must be a silent no-op, got %v", err)
	}
	n.mu.RLock()
	defer n.mu.RUnlock()
	if n.stop != nil {
		t.Fatal("a superseded loop must not install a stop channel")
	}
}

// --- PR-03/PR-04: generation-safe teardown + degradation ---

func TestDegradeConnectionTearsDownCurrentGen(t *testing.T) {
	n := newTestNode(t)
	n.mu.Lock()
	n.disconnectLocked()
	gen := n.connGen
	n.mu.Unlock()
	if !n.installConnection(&rpc_client.RpcClient{}, nil, governance.NewAPI(nil), "ws://x", 42, 3, gen) {
		t.Fatal("install should succeed")
	}
	if !n.degradeConnection(gen) {
		t.Fatal("degrade of the current generation must tear down")
	}
	st := n.NodeStatus()
	if st.Connected || st.Height != 0 || st.ChainID != 0 {
		t.Fatalf("after degradation the status must be disconnected with cleared height/chain, got %+v", st)
	}
	if n.currentClient() != nil {
		t.Fatal("no client may remain installed")
	}
	if n.currentGovernance() != nil {
		t.Fatal("no governance adapter may remain installed")
	}
	// Repeated closure/degradation of the same (now superseded) gen is a no-op —
	// no double-close, no second teardown.
	if n.degradeConnection(gen) {
		t.Fatal("a second degrade of the same generation must be a no-op")
	}
}

func TestDegradeConnectionStaleGenLeavesNewerConnection(t *testing.T) {
	n := newTestNode(t)
	n.mu.Lock()
	n.disconnectLocked()
	oldGen := n.connGen
	n.mu.Unlock()

	// A newer connection wins the slot…
	n.mu.Lock()
	n.disconnectLocked()
	newGen := n.connGen
	n.mu.Unlock()
	fresh := &rpc_client.RpcClient{}
	if !n.installConnection(fresh, nil, nil, "ws://fresh", 99, 3, newGen) {
		t.Fatal("newer install should succeed")
	}

	// …then the OLD subscription fails/closes. It must not touch the new one.
	if n.degradeConnection(oldGen) {
		t.Fatal("a stale generation must not degrade the newer connection")
	}
	if n.currentClient() != fresh {
		t.Fatal("the newer connection must remain installed")
	}
	st := n.NodeStatus()
	if !st.Connected || st.Height != 99 || st.ChainID != 3 {
		t.Fatalf("the newer connection's status must be untouched, got %+v", st)
	}
}

// --- PR-05: mode transitions are one ordered operation ---

func TestSetNodeModeSerialized(t *testing.T) {
	n := newTestNode(t)
	// Point remote at an unreachable local port so the second transition fails
	// fast instead of dialing a real network endpoint.
	if err := n.config.updateSettings(func(s *Settings) error {
		s.RemoteNodeURL = "ws://127.0.0.1:1"
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	entered := make(chan struct{})
	release := make(chan struct{})
	n.embeddedStart = func(dataDir string) (embeddedHandle, error) {
		close(entered)
		<-release
		return nil, errors.New("test: embedded start aborted")
	}

	firstDone := make(chan error, 1)
	go func() { firstDone <- n.SetNodeMode("embedded") }()
	<-entered // transition 1 is mid-flight inside the embedded start

	secondDone := make(chan error, 1)
	go func() { secondDone <- n.SetNodeMode("remote") }()
	select {
	case <-secondDone:
		t.Fatal("a second mode transition ran while the first was mid-operation")
	case <-time.After(100 * time.Millisecond):
	}

	close(release)
	<-firstDone  // embedded start fails; error expected
	<-secondDone // remote dial fails (unreachable); mode state must still be consistent

	// The LAST transition owns persisted mode, in-memory mode, and embedded state.
	s, err := n.config.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if s.NodeMode != "remote" {
		t.Fatalf("persisted mode = %q, want remote (the last transition)", s.NodeMode)
	}
	if st := n.NodeStatus(); st.Mode != "remote" {
		t.Fatalf("in-memory mode = %q, want remote", st.Mode)
	}
	n.mu.RLock()
	emb := n.embedded
	n.mu.RUnlock()
	if emb != nil {
		t.Fatal("no embedded handle may survive a superseding non-embedded transition")
	}
}

func TestSupersededEmbeddedStartCannotInstall(t *testing.T) {
	n := newTestNode(t)
	if err := n.config.updateSettings(func(s *Settings) error {
		s.RemoteNodeURL = "ws://127.0.0.1:1"
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	entered := make(chan struct{})
	release := make(chan struct{})
	n.embeddedStart = func(dataDir string) (embeddedHandle, error) {
		close(entered)
		<-release
		// The embedded node "starts" successfully, but by now a newer remote
		// transition is queued behind this one.
		return stubHandle{url: "ws://127.0.0.1:1", dir: dataDir}, nil
	}

	firstDone := make(chan error, 1)
	go func() { firstDone <- n.SetNodeMode("embedded") }()
	<-entered
	secondDone := make(chan error, 1)
	go func() { secondDone <- n.SetNodeMode("remote") }()
	close(release)
	<-firstDone
	<-secondDone

	// The remote transition ran strictly AFTER embedded finished, so it owns the
	// final state: mode remote, embedded handle stopped and gone.
	s, _ := n.config.GetSettings()
	if s.NodeMode != "remote" {
		t.Fatalf("persisted mode = %q, want remote", s.NodeMode)
	}
	n.mu.RLock()
	emb := n.embedded
	n.mu.RUnlock()
	if emb != nil {
		t.Fatal("the superseded embedded transition's node must have been stopped by the remote transition")
	}
}
