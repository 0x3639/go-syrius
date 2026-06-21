package app

import (
	"context"
	"errors"
	"fmt"
	neturl "net/url"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/0x3639/go-syrius/internal/embeddednode"
	"github.com/0x3639/znn-sdk-go/rpc_client"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"github.com/zenon-network/go-zenon/chain/nom"
	"github.com/zenon-network/go-zenon/common/types"
	api "github.com/zenon-network/go-zenon/rpc/api"
)

// embeddedHandle abstracts a running embedded node (real or test stub).
type embeddedHandle interface {
	WSURL() string
	DataDir() string
	Stop() error
}

// NodeService owns the remote RPC connection and surfaces reads + status events.
type NodeService struct {
	ctx    context.Context
	config *ConfigService
	wallet *WalletService

	mu      sync.RWMutex
	mode    string
	client  *rpc_client.RpcClient
	url     string
	height  uint64
	chainID uint64
	stop    chan struct{}

	receiveFn func(fromHash string) (string, error)
	autoStop  chan struct{}

	embedded      embeddedHandle
	embeddedStart func(dataDir string) (embeddedHandle, error)
	syncStop      chan struct{}
}

func newNodeService(c *ConfigService, w *WalletService) *NodeService {
	n := &NodeService{config: c, wallet: w}
	n.embeddedStart = func(dataDir string) (embeddedHandle, error) {
		return embeddednode.Start(dataDir)
	}
	return n
}

// SetNode connects to url, verifies reachability, and starts the momentum
// subscription that drives status/height events. URL persistence belongs to
// SetNodeMode/SetNodeURL; SetNode is a pure connect.
func (n *NodeService) SetNode(url string) error {
	n.mu.Lock()
	n.disconnectLocked()
	n.mu.Unlock()

	client, err := rpc_client.NewRpcClient(url)
	if err != nil {
		n.emitStatus(false)
		return fmt.Errorf("connect: %w", err)
	}
	m, err := client.LedgerApi.GetFrontierMomentum()
	if err != nil {
		client.Stop()
		n.emitStatus(false)
		return fmt.Errorf("node unreachable: %w", err)
	}

	n.mu.Lock()
	n.client = client
	n.url = url
	n.height = m.Height
	n.chainID = m.ChainIdentifier
	n.mu.Unlock()

	n.emitStatus(true)

	if err := n.startMomentumLoop(); err != nil {
		// Subscription failed: disconnect so we don't appear connected when no
		// ticks will ever fire.
		n.mu.Lock()
		n.disconnectLocked()
		n.mu.Unlock()
		n.emitStatus(false)
		return fmt.Errorf("subscribe to momentums: %w", err)
	}
	return nil
}

// startMomentumLoop starts the subscription goroutine and returns an error if
// the subscription cannot be established. The caller must NOT hold n.mu.
func (n *NodeService) startMomentumLoop() error {
	n.mu.RLock()
	client := n.client
	subCtx := n.ctx
	n.mu.RUnlock()

	if subCtx == nil {
		subCtx = context.Background()
	}

	sub, ch, err := client.SubscriberApi.ToMomentums(subCtx)
	if err != nil {
		return err
	}

	n.mu.Lock()
	n.stop = make(chan struct{})
	stop := n.stop
	n.mu.Unlock()

	go func() {
		defer sub.Unsubscribe()
		for {
			select {
			case <-stop:
				return
			case ms := <-ch:
				n.mu.Lock()
				for _, m := range ms {
					if m.Height > n.height {
						n.height = m.Height
					}
				}
				height := n.height
				ctx := n.ctx
				n.mu.Unlock()

				if ctx != nil {
					runtime.EventsEmit(ctx, EventMomentumTick, height)
				}
				n.emitStatus(true)
			}
		}
	}()
	return nil
}

// disconnectLocked tears down client and stop channel.
// Callers MUST hold n.mu (write lock) before calling this.
func (n *NodeService) disconnectLocked() {
	if n.stop != nil {
		close(n.stop)
		n.stop = nil
	}
	if n.autoStop != nil {
		close(n.autoStop)
		n.autoStop = nil
	}
	if n.client != nil {
		n.client.Stop()
		n.client = nil
	}
	// Reset the cached chain identifier so a stale value (e.g. testnet) can't be
	// read by currentChainID() after disconnect and bypass the mainnet guard.
	n.chainID = 0
	n.height = 0
}

// Disconnect closes the connection and stops the subscription.
func (n *NodeService) Disconnect() error {
	n.mu.Lock()
	n.disconnectLocked()
	n.mu.Unlock()
	n.emitStatus(false)
	return nil
}

// NodeStatus returns the current connection snapshot.
func (n *NodeService) NodeStatus() NodeStatus {
	n.mu.RLock()
	connected := n.client != nil
	height := n.height
	mode := n.mode
	n.mu.RUnlock()
	if mode == "" {
		mode = "remote"
	}
	return NodeStatus{Mode: mode, Connected: connected, Syncing: false, Height: height, Peers: 0}
}

func (n *NodeService) emitStatus(connected bool) {
	n.mu.RLock()
	ctx := n.ctx
	clientNil := n.client == nil
	height := n.height
	mode := n.mode
	n.mu.RUnlock()

	if ctx == nil {
		return
	}
	if mode == "" {
		mode = "remote"
	}
	st := NodeStatus{Mode: mode, Connected: connected && !clientNil, Height: height}
	runtime.EventsEmit(ctx, EventNodeStatus, st)
}

// SetNodeMode persists the node mode and connects to that mode's URL. The mode
// is persisted before connecting, so an unreachable node leaves the chosen mode
// in effect (the UI shows disconnected + Retry).
func (n *NodeService) SetNodeMode(mode string) error {
	if mode != "remote" && mode != "local" && mode != "embedded" {
		return fmt.Errorf("unknown node mode %q", mode)
	}
	s, err := n.config.GetSettings()
	if err != nil {
		return err
	}
	s.NodeMode = mode
	if err := n.config.SetSettings(s); err != nil {
		return err
	}

	// Tear down any running embedded node when leaving embedded mode.
	if mode != "embedded" {
		n.stopEmbedded()
	}
	n.mu.Lock()
	n.mode = mode
	n.mu.Unlock()

	if mode == "embedded" {
		return n.startEmbedded()
	}
	return n.SetNode(s.ActiveNodeURL())
}

// startEmbedded starts the embedded node, connects to it, and starts the sync
// poller. The caller has already persisted/marked mode == "embedded". Mutex
// discipline: mu is never held across embeddedStart/SetNode/startSyncPoller.
func (n *NodeService) startEmbedded() error {
	dir, err := n.config.dataDir()
	if err != nil {
		return err
	}
	h, serr := n.embeddedStart(dir)
	if serr != nil {
		n.emitStatus(false)
		return fmt.Errorf("start embedded node: %w", serr)
	}
	n.mu.Lock()
	n.embedded = h
	n.mu.Unlock()
	if cerr := n.SetNode(h.WSURL()); cerr != nil {
		n.stopEmbedded() // tear down the just-started node so Retry can start fresh
		return cerr
	}
	n.startSyncPoller()
	return nil
}

// SetNodeURL persists a mode's URL (validated) and reconnects if it is active.
func (n *NodeService) SetNodeURL(mode, url string) error {
	if mode == "embedded" {
		return fmt.Errorf("embedded node url is fixed and cannot be changed")
	}
	if mode != "remote" && mode != "local" {
		return fmt.Errorf("unknown node mode %q", mode)
	}
	u, perr := neturl.Parse(url)
	if perr != nil || (u.Scheme != "ws" && u.Scheme != "wss") || u.Host == "" {
		return fmt.Errorf("node url must be a ws:// or wss:// URL with a host")
	}
	s, err := n.config.GetSettings()
	if err != nil {
		return err
	}
	if mode == "local" {
		s.LocalNodeURL = url
	} else {
		s.RemoteNodeURL = url
	}
	if err := n.config.SetSettings(s); err != nil {
		return err
	}
	if mode == s.NodeMode {
		return n.SetNode(url)
	}
	return nil
}

// stopEmbedded halts the embedded node + sync poller if running.
func (n *NodeService) stopEmbedded() {
	n.mu.Lock()
	if n.syncStop != nil {
		close(n.syncStop)
		n.syncStop = nil
	}
	h := n.embedded
	n.embedded = nil
	n.mu.Unlock()
	if h != nil {
		_ = h.Stop()
	}
}

// startSyncPoller polls StatsApi sync info every 2s and emits node:sync.
func (n *NodeService) startSyncPoller() {
	n.mu.Lock()
	if n.syncStop != nil {
		close(n.syncStop)
	}
	stop := make(chan struct{})
	n.syncStop = stop
	client := n.client
	ctx := n.ctx
	n.mu.Unlock()
	if client == nil || ctx == nil {
		return
	}
	go func() {
		var samples []heightSample
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case now := <-ticker.C:
				info, err := client.StatsApi.SyncInfo()
				if err != nil {
					continue
				}
				peers := 0
				if ni, nerr := client.StatsApi.NetworkInfo(); nerr == nil {
					peers = ni.NumPeers
				}
				samples = append(samples, heightSample{T: now, Height: info.CurrentHeight})
				if len(samples) > 10 {
					samples = samples[len(samples)-10:]
				}
				st := computeSync(samples, info.CurrentHeight, info.TargetHeight, peers, mapSyncState(info.State))
				runtime.EventsEmit(ctx, EventNodeSync, st)
			}
		}
	}()
}

// GetEmbeddedInfo reports whether the embedded node is running and its data size.
func (n *NodeService) GetEmbeddedInfo() (EmbeddedInfo, error) {
	dir, err := n.config.dataDir()
	if err != nil {
		return EmbeddedInfo{}, err
	}
	emb := filepath.Join(dir, "embedded")
	n.mu.RLock()
	running := n.embedded != nil
	n.mu.RUnlock()
	return EmbeddedInfo{Running: running, DataDir: emb, SizeBytes: dirSize(emb)}, nil
}

// DeleteEmbeddedData removes the embedded chain DB. Refuses while running.
func (n *NodeService) DeleteEmbeddedData() error {
	n.mu.RLock()
	running := n.embedded != nil
	n.mu.RUnlock()
	if running {
		return errors.New("stop the embedded node first")
	}
	dir, err := n.config.dataDir()
	if err != nil {
		return err
	}
	return os.RemoveAll(filepath.Join(dir, "embedded"))
}

func dirSize(path string) int64 {
	var total int64
	_ = filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err == nil && info != nil && !info.IsDir() {
			total += info.Size()
		}
		return nil
	})
	return total
}

// Connect connects to the active mode's URL using persisted settings.
func (n *NodeService) Connect() error {
	s, err := n.config.GetSettings()
	if err != nil {
		return err
	}
	n.mu.Lock()
	n.mode = s.NodeMode
	n.mu.Unlock()
	if s.NodeMode == "embedded" {
		return n.startEmbedded()
	}
	return n.SetNode(s.ActiveNodeURL())
}

// GetNodeConfig returns the node mode and per-mode URLs for the settings UI.
func (n *NodeService) GetNodeConfig() (NodeConfig, error) {
	s, err := n.config.GetSettings()
	if err != nil {
		return NodeConfig{}, err
	}
	return NodeConfig{Mode: s.NodeMode, RemoteURL: s.RemoteNodeURL, LocalURL: s.LocalNodeURL}, nil
}

// GetBalances returns the active address's balances.
func (n *NodeService) GetBalances() ([]TokenBalance, error) {
	n.mu.RLock()
	client := n.client
	n.mu.RUnlock()

	if client == nil {
		return nil, errors.New("not connected")
	}
	addr, ok := n.wallet.activeAddress()
	if !ok {
		return nil, errLocked
	}
	info, err := client.LedgerApi.GetAccountInfoByAddress(addr)
	if err != nil {
		return nil, err
	}
	out := []TokenBalance{}
	for zts, bi := range info.BalanceInfoMap {
		out = append(out, toTokenBalance(zts, bi))
	}
	return out, nil
}

// GetTransactions returns one page of the active address's account blocks.
func (n *NodeService) GetTransactions(page, count int) ([]TxRecord, error) {
	n.mu.RLock()
	client := n.client
	n.mu.RUnlock()

	if client == nil {
		return nil, errors.New("not connected")
	}
	addr, ok := n.wallet.activeAddress()
	if !ok {
		return nil, errLocked
	}
	list, err := client.LedgerApi.GetAccountBlocksByPage(addr, uint32(page), uint32(count))
	if err != nil {
		return nil, err
	}
	out := []TxRecord{}
	for _, b := range list.List {
		out = append(out, toTxRecord(b))
	}
	return out, nil
}

// currentClient returns the connected client or nil, under the read lock.
func (n *NodeService) currentClient() *rpc_client.RpcClient {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.client
}

// currentChainID returns the connected node's chain identifier (0 if unknown).
func (n *NodeService) currentChainID() uint64 {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.chainID
}

// setReceiveFunc wires the callback used by auto-receive to receive each block.
func (n *NodeService) setReceiveFunc(fn func(string) (string, error)) { n.receiveFn = fn }

// StartAutoReceive subscribes to unreceived blocks for the active address and
// receives each via receiveFn. Idempotent; StopAutoReceive stops it.
func (n *NodeService) StartAutoReceive() error {
	// Claim the running slot under the lock before subscribing so a concurrent
	// or repeated call returns early instead of orphaning a goroutine.
	n.mu.Lock()
	if n.autoStop != nil {
		n.mu.Unlock()
		return nil // already running
	}
	stop := make(chan struct{})
	n.autoStop = stop
	n.mu.Unlock()

	// releaseSlot clears the reserved slot iff it's still ours, so a failed
	// start never leaves a phantom "running" state.
	releaseSlot := func() {
		n.mu.Lock()
		if n.autoStop == stop {
			n.autoStop = nil
		}
		n.mu.Unlock()
	}

	client := n.currentClient()
	if client == nil {
		releaseSlot()
		return errors.New("not connected")
	}
	addr, ok := n.wallet.activeAddress()
	if !ok {
		releaseSlot()
		return errLocked
	}
	ctx := n.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	sub, ch, err := client.SubscriberApi.ToUnreceivedAccountBlocksByAddress(ctx, addr)
	if err != nil {
		releaseSlot()
		return err
	}
	go func() {
		defer sub.Unsubscribe()
		for {
			select {
			case <-stop:
				return
			case blocks := <-ch:
				if n.receiveFn == nil {
					continue
				}
				for _, b := range blocks {
					_, _ = n.receiveFn(b.Hash.String())
				}
			}
		}
	}()
	return nil
}

// StopAutoReceive stops the auto-receive subscription if running.
func (n *NodeService) StopAutoReceive() {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.autoStop != nil {
		close(n.autoStop)
		n.autoStop = nil
	}
}

// GetUnreceived lists inbound blocks not yet received by the active address.
func (n *NodeService) GetUnreceived() ([]UnreceivedBlock, error) {
	client := n.currentClient()
	if client == nil {
		return nil, errors.New("not connected")
	}
	addr, ok := n.wallet.activeAddress()
	if !ok {
		return nil, errLocked
	}
	list, err := client.LedgerApi.GetUnreceivedBlocksByAddress(addr, 0, 50)
	if err != nil {
		return nil, err
	}
	out := []UnreceivedBlock{}
	for _, b := range list.List {
		out = append(out, toUnreceivedBlock(b))
	}
	return out, nil
}

func toUnreceivedBlock(b *api.AccountBlock) UnreceivedBlock {
	u := UnreceivedBlock{FromHash: b.Hash.String(), FromAddress: b.Address.String(), Amount: "0", Token: b.TokenStandard.String()}
	if b.Amount != nil {
		u.Amount = b.Amount.String()
	}
	if b.TokenInfo != nil {
		u.Token = b.TokenInfo.TokenSymbol
	}
	return u
}

func toTokenBalance(zts types.ZenonTokenStandard, bi *api.BalanceInfo) TokenBalance {
	tb := TokenBalance{Zts: zts.String(), Amount: "0"}
	if bi.Balance != nil {
		tb.Amount = bi.Balance.String()
	}
	if bi.TokenInfo != nil {
		tb.Symbol = bi.TokenInfo.TokenSymbol
		tb.Decimals = int(bi.TokenInfo.Decimals)
	}
	return tb
}

func toTxRecord(b *api.AccountBlock) TxRecord {
	rec := TxRecord{
		Hash:      b.Hash.String(),
		Token:     b.TokenStandard.String(),
		Amount:    "0",
		Direction: "receive",
	}
	if b.Amount != nil {
		rec.Amount = b.Amount.String()
	}
	if nom.IsSendBlock(b.BlockType) {
		rec.Direction = "send"
		rec.Counterparty = b.ToAddress.String()
	} else {
		rec.Counterparty = b.Address.String()
	}
	if b.TokenInfo != nil {
		rec.Token = b.TokenInfo.TokenSymbol
	}
	if b.ConfirmationDetail != nil {
		rec.Confirmed = true
		rec.MomentumHeight = b.ConfirmationDetail.MomentumHeight
	}
	return rec
}
