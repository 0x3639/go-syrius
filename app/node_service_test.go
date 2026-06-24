package app

import (
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"

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
	rec := toTxRecord(send)
	if rec.Direction != "send" {
		t.Fatalf("direction = %s, want send", rec.Direction)
	}
	if rec.Amount != "100000000" || rec.Confirmed {
		t.Fatalf("rec = %+v", rec)
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
	got := toUnreceivedBlock(b)
	if got.FromAddress != b.Address.String() || got.Amount != "150000000" {
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
	err := n.SetNode("ws://127.0.0.1:1")
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
	s, err := n.config.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	s.NodeMode = "embedded"
	if err := n.config.SetSettings(s); err != nil {
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
