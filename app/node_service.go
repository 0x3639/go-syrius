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
	chainID := n.chainID
	n.mu.RUnlock()
	if mode == "" {
		mode = "remote"
	}
	return NodeStatus{Mode: mode, Connected: connected, Syncing: false, Height: height, Peers: 0, ChainID: chainID}
}

func (n *NodeService) emitStatus(connected bool) {
	n.mu.RLock()
	ctx := n.ctx
	clientNil := n.client == nil
	height := n.height
	mode := n.mode
	chainID := n.chainID
	n.mu.RUnlock()

	if ctx == nil {
		return
	}
	if mode == "" {
		mode = "remote"
	}
	st := NodeStatus{Mode: mode, Connected: connected && !clientNil, Height: height, ChainID: chainID}
	runtime.EventsEmit(ctx, EventNodeStatus, st)
}

// SetNodeMode persists the node mode and connects to that mode's URL. The mode
// is persisted before connecting, so an unreachable node leaves the chosen mode
// in effect (the UI shows disconnected + Retry).
func (n *NodeService) SetNodeMode(mode string) error {
	if mode != "remote" && mode != "local" && mode != "embedded" {
		return fmt.Errorf("unknown node mode %q", mode)
	}
	if err := n.config.updateSettings(func(s *Settings) error {
		s.NodeMode = mode
		return nil
	}); err != nil {
		return err
	}
	s, err := n.config.GetSettings()
	if err != nil {
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
	// Switching to embedded supersedes any current connection; drop it first so a
	// failed embedded start cannot leave the old client emitting as "embedded".
	n.mu.Lock()
	n.disconnectLocked()
	n.mu.Unlock()
	n.emitStatus(false)

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
	activeMode := ""
	if err := n.config.updateSettings(func(s *Settings) error {
		if mode == "local" {
			s.LocalNodeURL = url
		} else {
			s.RemoteNodeURL = url
		}
		activeMode = s.NodeMode
		return nil
	}); err != nil {
		return err
	}
	if mode == activeMode {
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

// GetTransactions returns one page of the active address's account blocks plus
// whether a further page exists (for the history's pager).
func (n *NodeService) GetTransactions(page, count int) (TxPage, error) {
	if page < 0 || count < 0 {
		return TxPage{}, errors.New("page and count must be non-negative")
	}
	n.mu.RLock()
	client := n.client
	n.mu.RUnlock()

	if client == nil {
		return TxPage{}, errors.New("not connected")
	}
	addr, ok := n.wallet.activeAddress()
	if !ok {
		return TxPage{}, errLocked
	}
	list, err := client.LedgerApi.GetAccountBlocksByPage(addr, uint32(page), uint32(count)) // #nosec G115 -- page/count validated non-negative above; pagination values are small
	if err != nil {
		return TxPage{}, err
	}
	dc := newDecimalsCache(clientTokenDecimals(client))
	out := []TxRecord{}
	for _, b := range list.List {
		out = append(out, blockToRecords(b, dc)...)
	}
	// Derive hasMore from the total account-block count: the node's `More` flag is
	// unreliable (observed false even with thousands of pages remaining).
	return TxPage{Records: out, HasMore: (page+1)*count < list.Count}, nil
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
		// Drain whatever is already pending (the subscription only delivers NEW
		// blocks). Then re-drain on every new-block signal.
		n.sweepUnreceived(client, addr, stop)
		for {
			select {
			case <-stop:
				return
			case <-ch:
				n.sweepUnreceived(client, addr, stop)
			}
		}
	}()
	return nil
}

// sweepUnreceived receives every currently-unreceived block for addr, re-fetching
// and retrying until the queue drains or stalls. Each receive shifts the account
// frontier, and the node needs a moment to fold it in before the next receive's
// template is valid — so a single pass (the old behaviour) could receive the
// first block and leave later ones stuck. Emits a receiving:active event around
// the work so the UI can show progress (PoW/plasma generation takes seconds).
func (n *NodeService) sweepUnreceived(client *rpc_client.RpcClient, addr types.Address, stop <-chan struct{}) {
	if n.receiveFn == nil {
		return
	}
	first, err := client.LedgerApi.GetUnreceivedBlocksByAddress(addr, 0, 50)
	if err != nil || first == nil || len(first.List) == 0 {
		return
	}
	n.emitReceiving(true)
	defer n.emitReceiving(false)

	pending := first
	stale := 0
	for stale < 3 {
		select {
		case <-stop:
			return
		default:
		}
		before := len(pending.List)
		for _, b := range pending.List {
			select {
			case <-stop:
				return
			default:
				// Surface failures: a swallowed error here means auto-receive
				// silently stalls (it retries a few times, then gives up). Emit so
				// the UI can flag it instead of the block just never arriving.
				if _, err := n.receiveFn(b.Hash.String()); err != nil && n.ctx != nil {
					runtime.EventsEmit(n.ctx, EventAutoReceiveError, map[string]string{
						"hash":  b.Hash.String(),
						"error": err.Error(),
					})
				}
			}
		}
		// Let the node fold the just-published receives into the account frontier
		// before re-checking, so any retried block builds on the right prev-hash.
		select {
		case <-stop:
			return
		case <-time.After(1500 * time.Millisecond):
		}
		next, err := client.LedgerApi.GetUnreceivedBlocksByAddress(addr, 0, 50)
		if err != nil || next == nil || len(next.List) == 0 {
			return
		}
		if len(next.List) >= before {
			stale++ // no progress; give the frontier a few more tries, then give up
		} else {
			stale = 0
		}
		pending = next
	}
}

// emitReceiving notifies the frontend that auto-receive is (or is no longer)
// actively receiving — used to show a "Receiving…" / generating-plasma indicator.
func (n *NodeService) emitReceiving(active bool) {
	if n.ctx != nil {
		runtime.EventsEmit(n.ctx, EventAutoReceiving, active)
	}
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
	dc := newDecimalsCache(clientTokenDecimals(client))
	out := []UnreceivedBlock{}
	for _, b := range list.List {
		out = append(out, toUnreceivedBlock(b, dc.get(b.TokenStandard.String())))
	}
	return out, nil
}

func toUnreceivedBlock(b *api.AccountBlock, decimals int) UnreceivedBlock {
	u := UnreceivedBlock{FromHash: b.Hash.String(), FromAddress: b.Address.String(), Amount: "0", Token: b.TokenStandard.String(), Decimals: decimals}
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

// recordFor builds one TxRecord from a single account block's own fields.
func recordFor(b *api.AccountBlock, direction, counterparty, method string, dc *decimalsCache) TxRecord {
	rec := TxRecord{
		Hash:         b.Hash.String(),
		Direction:    direction,
		Method:       method,
		Counterparty: counterparty,
		Token:        b.TokenStandard.String(),
		Amount:       "0",
		Decimals:     dc.get(b.TokenStandard.String()),
	}
	if b.Amount != nil {
		rec.Amount = b.Amount.String()
	}
	if b.TokenInfo != nil {
		rec.Token = b.TokenInfo.TokenSymbol
	}
	if b.TokenStandard == types.ZeroTokenStandard {
		rec.Token = "" // no token (claim/contract-ack block) — the UI renders a dash
	}
	if b.ConfirmationDetail != nil {
		rec.Confirmed = true
		rec.MomentumHeight = b.ConfirmationDetail.MomentumHeight
	}
	return rec
}

// blockToRecords expands one account block into history rows, mirroring nomscan:
//   - a SEND becomes one OUT row (with the embedded method it calls, if any);
//   - a RECEIVE becomes an IN row for the value (the paired send: real amount,
//     token, and sender) plus a PAIR row for the zero-amount claim block itself.
func blockToRecords(b *api.AccountBlock, dc *decimalsCache) []TxRecord {
	if nom.IsSendBlock(b.BlockType) {
		return []TxRecord{recordFor(b, "out", b.ToAddress.String(), decodeMethod(b.ToAddress, b.Data), dc)}
	}
	out := make([]TxRecord, 0, 2)
	if p := b.PairedAccountBlock; p != nil {
		out = append(out, recordFor(p, "in", p.Address.String(), "", dc))
	}
	pairWith := b.Address.String()
	if b.PairedAccountBlock != nil {
		pairWith = b.PairedAccountBlock.Address.String()
	}
	out = append(out, recordFor(b, "pair", pairWith, "", dc))
	return out
}
