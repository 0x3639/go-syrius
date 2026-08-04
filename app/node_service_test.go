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

// wedgedHandle simulates the 2026-08-03 incident: an embedded node whose
// Stop() never returns (spec §2/§4).
type wedgedHandle struct {
	stopCalled chan struct{}
	release    chan struct{}
}

func (w *wedgedHandle) WSURL() string   { return "ws://127.0.0.1:35998" }
func (w *wedgedHandle) DataDir() string { return "" }
func (w *wedgedHandle) Stop() error {
	close(w.stopCalled)
	<-w.release
	return nil
}

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

func TestSetNodeModeRejectsLocal(t *testing.T) {
	n := newTestNode(t)
	if err := n.SetNodeMode("local"); err == nil {
		t.Fatal("SetNodeMode(local) succeeded, want error")
	}
	// Rejection must happen BEFORE any persistence: a removed mode is never user
	// intent, so nothing may reach disk. Asserting the persisted mode via
	// GetNodeConfig would be vacuous — it runs migrateSettings on read, which
	// normalizes a persisted "local" back to "remote". The honest gate is that
	// settings.json was never written at all (newTestConfig starts empty).
	assertSettingsNotWritten(t, n)
}

func TestSetNodeURLRejectsLocal(t *testing.T) {
	n := newTestNode(t)
	if err := n.SetNodeURL("local", "ws://127.0.0.1:35998"); err == nil {
		t.Fatal("SetNodeURL(local) succeeded, want error")
	}
	assertSettingsNotWritten(t, n)
}

// assertSettingsNotWritten fails if settings.json exists — the reject-before-
// persist gate. It relies on newTestConfig's fresh empty data dir, so absence
// of the file is proof no write happened.
func assertSettingsNotWritten(t *testing.T, n *NodeService) {
	t.Helper()
	d, err := n.config.dataDir()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(d, "settings.json")); !os.IsNotExist(err) {
		t.Fatal("rejected mode must not persist: settings.json was written")
	}
}

// seedNodeMode persists a node mode WITHOUT connecting. It lets a test make
// "remote" the non-active mode so SetNodeURL exercises its persist-only path
// (no dial, hence no network dependence). The in-memory n.mode is set to match
// so NodeStatus() reports the seeded mode — NodeStatus reports "remote" for an
// empty mode, which would make a later "is it remote?" assertion vacuous.
func seedNodeMode(t *testing.T, n *NodeService, mode string) {
	t.Helper()
	if err := n.config.updateSettings(func(s *Settings) error {
		s.NodeMode = mode
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	n.mu.Lock()
	n.mode = mode
	n.mu.Unlock()
}

func TestSetNodeModePersistsEvenIfUnreachable(t *testing.T) {
	n := newTestNode(t)
	// Start from embedded so the transition below is a real mode change, and
	// point remote at an unreachable node. The connect attempt fails, but the
	// chosen mode must still be persisted (user intent) and reflected by
	// GetNodeConfig.
	seedNodeMode(t, n, "embedded")
	if err := n.config.updateSettings(func(s *Settings) error {
		s.RemoteNodeURL = "ws://127.0.0.1:1"
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	_ = n.SetNodeMode("remote") // connect error expected and ignored here
	cfg, err := n.GetNodeConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Mode != "remote" {
		t.Fatalf("mode should persist as remote, got %q", cfg.Mode)
	}
	if n.NodeStatus().Mode != "remote" {
		t.Fatalf("NodeStatus().Mode should be remote, got %q", n.NodeStatus().Mode)
	}
}

// A wedged embedded Stop() must not hold a mode transition hostage: the switch
// completes on a bound, the status flips honestly, and embedded is then refused
// in-process until the app restarts (the abandoned node still owns the WS port).
func TestSetNodeModeBoundedTeardownOnWedgedStop(t *testing.T) {
	old := embeddedStopTimeout
	embeddedStopTimeout = 100 * time.Millisecond
	t.Cleanup(func() { embeddedStopTimeout = old })

	n := newTestNode(t)
	// Unreachable remote so the post-teardown dial fails fast instead of
	// touching the network (same trick as TestSetNodeModePersistsEvenIfUnreachable).
	if err := n.config.updateSettings(func(s *Settings) error {
		s.RemoteNodeURL = "ws://127.0.0.1:1"
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	seedNodeMode(t, n, "embedded")
	// No real node may ever boot from this test: if a wedged guard regresses,
	// fail loudly instead of starting a mainnet node in CI.
	n.embeddedStart = func(string) (embeddedHandle, error) {
		return nil, errors.New("test: embedded start must not be reached")
	}
	w := &wedgedHandle{stopCalled: make(chan struct{}), release: make(chan struct{})}
	t.Cleanup(func() { close(w.release) }) // let the background waiter exit
	n.mu.Lock()
	n.embedded = w
	// A live embedded connection, so the mid-teardown status assertion below is
	// about the transition dropping it — not about a service that never had one.
	n.client = &rpc_client.RpcClient{}
	n.chainID = 3
	n.mu.Unlock()

	done := make(chan error, 1)
	go func() { done <- n.SetNodeMode("remote") }()

	// Stop must have been attempted…
	select {
	case <-w.stopCalled:
	case <-time.After(2 * time.Second):
		t.Fatal("Stop was never called")
	}
	// Honest transition status (spec §5): the mode must ALREADY read "remote"
	// here — mid-teardown, with Stop still blocked and nothing dialed yet.
	// This is deterministic, not a race: SetNodeMode writes n.mode before
	// calling stopEmbedded, which happens-before close(w.stopCalled) inside
	// Stop(). Pins the reorder; without it the flip could move back after
	// teardown and no test would notice.
	if got := n.NodeStatus().Mode; got != "remote" {
		t.Fatalf("mode at transition start = %q, want remote (flip must precede teardown)", got)
	}
	// …and the status must be HONEST about it: the outgoing embedded client is
	// dropped in the same critical section as the mode flip, so a NodeStatus()
	// pull here cannot report the old connection (height/chain id included)
	// under the new mode for the length of the teardown.
	if st := n.NodeStatus(); st.Connected || st.ChainID != 0 {
		t.Fatalf("status mid-teardown = %+v, want disconnected with chainID 0 (old client must be dropped at transition start)", st)
	}
	// …and the transition must complete despite Stop never returning.
	// (The remote dial fails fast against the unreachable test URL — an error
	// return is fine; a hang is the bug.)
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("SetNodeMode hung on wedged embedded Stop")
	}

	// Honest status: the mode flipped even though Stop is still blocked.
	if got := n.NodeStatus().Mode; got != "remote" {
		t.Fatalf("mode = %q, want remote", got)
	}
	// The wedged service refuses to restart embedded in-process.
	if err := n.SetNodeMode("embedded"); err == nil ||
		!strings.Contains(err.Error(), "restart go-syrius") {
		t.Fatalf("SetNodeMode(embedded) after wedge: err = %v, want restart-required error", err)
	}
	// Reject-before-persist: the refusal must not have written the mode it
	// refused. settings.json EXISTS here (the remote transition above wrote it),
	// so the honest gate is the persisted VALUE, which must still agree with
	// in-memory mode and NodeStatus.
	cfg, err := n.GetNodeConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Mode != "remote" {
		t.Fatalf("refused mode must not persist: persisted mode = %q, want remote", cfg.Mode)
	}
	if got := n.NodeStatus().Mode; got != "remote" {
		t.Fatalf("in-memory mode after refusal = %q, want remote", got)
	}
}

// OnShutdown calls stopEmbedded WITHOUT opMu, so a user-driven Connect() can
// arrive while that teardown is still in its bounded wait. In that window the
// handle is already nil and the wedge is not yet latched, so an unguarded
// Connect would sail into embeddednode.Start() and block forever on the package
// mutex the in-flight Stop() holds — and OnShutdown's own Disconnect() would
// then deadlock behind it on opMu. The stop must therefore be visible for the
// whole wait, and Connect must refuse promptly instead of starting anything.
func TestConnectDuringShutdownStopDoesNotDeadlock(t *testing.T) {
	old := embeddedStopTimeout
	// Registered FIRST so it runs LAST (cleanups are LIFO): the stop goroutine
	// below reads this var, and must be joined before it is restored.
	t.Cleanup(func() { embeddedStopTimeout = old })
	// Long enough that the Connect below lands inside the stopping window rather
	// than after the wedge latches — both refusals are accepted, but the
	// transient one is the case under test.
	embeddedStopTimeout = 3 * time.Second

	n := newTestNode(t)
	seedNodeMode(t, n, "embedded")
	n.embeddedStart = func(string) (embeddedHandle, error) {
		t.Errorf("embeddedStart must not be reached while a stop is in flight")
		return nil, errors.New("test: embedded start must not be reached")
	}
	w := &wedgedHandle{stopCalled: make(chan struct{}), release: make(chan struct{})}
	stopDone := make(chan struct{})
	t.Cleanup(func() {
		close(w.release)
		<-stopDone
	})
	n.mu.Lock()
	n.embedded = w
	n.mu.Unlock()

	// No opMu — exactly how OnShutdown calls it.
	go func() {
		n.stopEmbedded()
		close(stopDone)
	}()
	select {
	case <-w.stopCalled:
	case <-time.After(2 * time.Second):
		t.Fatal("Stop was never called")
	}

	done := make(chan error, 1)
	go func() { done <- n.Connect() }()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("Connect during an in-flight embedded stop must refuse, got nil error")
		}
		if !strings.Contains(err.Error(), "stopping") && !strings.Contains(err.Error(), "restart go-syrius") {
			t.Fatalf("Connect mid-stop: err = %v, want a stopping (or already-wedged) refusal", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Connect deadlocked during an in-flight embedded stop")
	}
}

// Connect() reaches startEmbedded with the mode already persisted, so the
// SetNodeMode-level guard cannot cover it. It MUST still refuse: embeddednode's
// package mutex is held across the wedged node.Stop(), so embeddednode.Start()
// would block on mu.Lock() forever while Connect holds opMu — the original
// incident, recreated. Hence the backstop guard inside startEmbedded.
func TestConnectRefusesEmbeddedWhileWedged(t *testing.T) {
	n := newTestNode(t)
	seedNodeMode(t, n, "embedded")
	n.embeddedStart = func(string) (embeddedHandle, error) {
		return nil, errors.New("test: embedded start must not be reached")
	}
	n.mu.Lock()
	n.embeddedWedged = true
	n.mu.Unlock()

	done := make(chan error, 1)
	go func() { done <- n.Connect() }()
	select {
	case err := <-done:
		if err == nil || !strings.Contains(err.Error(), "restart go-syrius") {
			t.Fatalf("Connect while wedged: err = %v, want restart-required error", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Connect hung on a wedged embedded node")
	}
}

// A wedged embedded node may still be writing to its data dir, so deleting it
// out from under the abandoned process risks corrupting whatever RemoveAll does
// not manage to remove. Refuse until the app restarts.
func TestDeleteEmbeddedDataRefusedWhileWedged(t *testing.T) {
	n := newTestNode(t)
	n.mu.Lock()
	n.embeddedWedged = true
	n.mu.Unlock()

	err := n.DeleteEmbeddedData()
	if err == nil || !strings.Contains(err.Error(), "restart go-syrius") {
		t.Fatalf("DeleteEmbeddedData while wedged: err = %v, want restart-required error", err)
	}
}

// Same hazard as the wedged case, transient cause: while stopEmbedded is in its
// bounded wait the handle is already nil — so the "stop the embedded node first"
// check passes — but the node is very much alive and still writing to the data
// dir. RemoveAll under it risks corrupting whatever it fails to remove.
func TestDeleteEmbeddedDataRefusedWhileStopping(t *testing.T) {
	n := newTestNode(t)
	n.mu.Lock()
	n.embeddedStopping = true
	n.mu.Unlock()

	err := n.DeleteEmbeddedData()
	if err == nil || !strings.Contains(err.Error(), "stopping") {
		t.Fatalf("DeleteEmbeddedData mid-stop: err = %v, want a stopping refusal", err)
	}
}

func TestSetNodeURLValidatesAndPersists(t *testing.T) {
	n := newTestNode(t)
	if err := n.SetNodeURL("bogus", "ws://x"); err == nil {
		t.Fatal("expected unknown mode to error")
	}
	if err := n.SetNodeURL("remote", "http://x"); err == nil {
		t.Fatal("expected non-ws scheme to error")
	}
	// Setting the non-active mode's URL persists without a reconnect (no error).
	seedNodeMode(t, n, "embedded")
	if err := n.SetNodeURL("remote", "ws://127.0.0.1:9"); err != nil {
		t.Fatalf("SetNodeURL(remote): %v", err)
	}
	cfg, _ := n.GetNodeConfig()
	if cfg.RemoteURL != "ws://127.0.0.1:9" {
		t.Fatalf("RemoteURL not persisted: %q", cfg.RemoteURL)
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
	// Success while remote is the non-active mode, so it persists without
	// connecting.
	seedNodeMode(t, n, "embedded")
	if err := n.SetNodeURL("remote", "wss://host.example:35998"); err != nil {
		t.Fatalf("SetNodeURL(remote, valid): %v", err)
	}
	cfg, err := n.GetNodeConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.RemoteURL != "wss://host.example:35998" {
		t.Fatalf("RemoteURL not persisted: %q", cfg.RemoteURL)
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
	if cfg.Mode != "remote" || cfg.RemoteURL != defaultNodeURL {
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

// The embedded sync poller must feed the live ledger height into the status
// height: during bulk sync the momentum subscription delivers only every few
// minutes, so without this bump the UI height pill lags the real sync progress
// by whole epochs (observed live: status height 408k vs sync height 538k).
func TestNoteSyncHeight_BumpsStatusMonotonically(t *testing.T) {
	n := newTestNode(t)
	n.mu.Lock()
	n.height = 100
	n.mu.Unlock()

	n.noteSyncHeight(150)
	if got := n.NodeStatus().Height; got != 150 {
		t.Fatalf("sync sample must raise status height: got %d want 150", got)
	}
	n.noteSyncHeight(120) // a lower/stale sample must never regress the height
	if got := n.NodeStatus().Height; got != 150 {
		t.Fatalf("height must be monotonic: got %d want 150", got)
	}
	n.noteSyncHeight(0) // a not-ready sample is ignored
	if got := n.NodeStatus().Height; got != 150 {
		t.Fatalf("zero sample must be ignored: got %d want 150", got)
	}
}

// A dead connection (e.g. after sleep/wake) tears down via disconnect, and the
// sync poller must go down with it: its captured client is stopped, so a
// surviving poller would error silently forever while looking alive.
func TestDisconnectStopsSyncPoller(t *testing.T) {
	n := newTestNode(t)
	stop := make(chan struct{})
	n.mu.Lock()
	n.syncStop = stop
	n.mu.Unlock()

	if err := n.Disconnect(); err != nil {
		t.Fatalf("Disconnect: %v", err)
	}
	select {
	case <-stop: // closed — the poller goroutine has been released
	default:
		t.Fatal("disconnect must close the sync poller's stop channel")
	}
	n.mu.RLock()
	defer n.mu.RUnlock()
	if n.syncStop != nil {
		t.Fatal("disconnect must clear syncStop so a later stop can't double-close")
	}
}

// After a wallet-side connection death (degradeConnection) the embedded node
// keeps running. A reconnect (Retry/Connect) must dial the RUNNING node again —
// starting a second one would hit the single-instance guard and leave the user
// unable to reconnect without an app restart.
func TestConnectReusesRunningEmbeddedNode(t *testing.T) {
	n := newTestNode(t)
	if err := n.config.updateSettings(func(s *Settings) error {
		s.NodeMode = "embedded"
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	starts := 0
	n.embeddedStart = func(dataDir string) (embeddedHandle, error) {
		starts++
		return stubHandle{url: "ws://127.0.0.1:1", dir: dataDir}, nil
	}
	// Simulate the post-degrade state: node running, wallet connection gone.
	n.mu.Lock()
	n.embedded = stubHandle{url: "ws://127.0.0.1:1", dir: "d"}
	n.mode = "embedded"
	n.mu.Unlock()

	_ = n.Connect() // dial fails (unreachable stub URL); ignore the error
	if starts != 0 {
		t.Fatalf("Connect must reuse the running embedded node, started %d new node(s)", starts)
	}
}

// The embedded node must hold an App Nap prevention assertion for exactly as
// long as it runs: begun when the node starts, released when it stops —
// including the teardown after a failed wallet connect.
func TestEmbeddedTogglesAppNapPrevention(t *testing.T) {
	n := newTestNode(t)
	var begins, ends int
	n.appNapBegin = func(string) { begins++ }
	n.appNapEnd = func() { ends++ }
	n.embeddedStart = func(dataDir string) (embeddedHandle, error) {
		return stubHandle{url: "ws://127.0.0.1:1", dir: dataDir}, nil
	}
	_ = n.SetNodeMode("embedded") // node starts; connect fails; node torn down
	if begins != 1 {
		t.Fatalf("App Nap prevention must begin once with the node, got %d", begins)
	}
	if ends != 1 {
		t.Fatalf("teardown must release the App Nap assertion, got %d end(s)", ends)
	}
}

// Switching away from embedded mode stops the node and must release the App
// Nap assertion with it.
func TestModeSwitchReleasesAppNap(t *testing.T) {
	n := newTestNode(t)
	var ends int
	n.appNapBegin = func(string) {}
	n.appNapEnd = func() { ends++ }
	if err := n.config.updateSettings(func(s *Settings) error {
		s.RemoteNodeURL = "ws://127.0.0.1:1"
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	n.mu.Lock()
	n.embedded = stubHandle{url: "ws://127.0.0.1:1", dir: "d"}
	n.mode = "embedded"
	n.mu.Unlock()

	_ = n.SetNodeMode("remote") // dial fails; the embedded teardown still runs
	if ends != 1 {
		t.Fatalf("leaving embedded mode must release the App Nap assertion, got %d end(s)", ends)
	}
}

// GS-11: node URLs must not persist query/fragment; userinfo stays allowed
// (legitimate basic-auth to the user's own node) but is scrubbed from errors.
func TestSetNodeURL_RejectsQueryAndFragment(t *testing.T) {
	n := newTestNode(t)
	for _, bad := range []string{"wss://h:35998?apikey=x", "wss://h:35998#frag", "ws://h:35998/path?a=b"} {
		if err := n.SetNodeURL("remote", bad); err == nil {
			t.Fatalf("url %q must be rejected", bad)
		}
	}
	// Userinfo must pass validation. Make embedded the active mode so remote
	// persists without dialing — dialing would otherwise make this a
	// network-dependent (flaky) test of the validation path.
	seedNodeMode(t, n, "embedded")
	if err := n.SetNodeURL("remote", "wss://user:pass@h:35998"); err != nil {
		t.Fatalf("basic-auth userinfo must remain allowed: %v", err)
	}
}

func TestRedactURLUserinfo(t *testing.T) {
	got := redactURLUserinfo("dial ws://user:pass@h:1/ failed", "ws://user:pass@h:1")
	if strings.Contains(got, "pass") {
		t.Fatalf("credentials leaked: %q", got)
	}
	if !strings.Contains(got, "***@") {
		t.Fatalf("redaction marker missing: %q", got)
	}
	// URLs without userinfo pass through untouched.
	if msg := redactURLUserinfo("connect refused", "wss://h:1"); msg != "connect refused" {
		t.Fatalf("no-userinfo message altered: %q", msg)
	}
}
