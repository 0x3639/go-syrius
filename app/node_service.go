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
	"github.com/0x3639/go-syrius/internal/governance"
	"github.com/0x3639/znn-sdk-go/rpc_client"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"github.com/zenon-network/go-zenon/chain/nom"
	"github.com/zenon-network/go-zenon/common/types"
	api "github.com/zenon-network/go-zenon/rpc/api"
	"github.com/zenon-network/go-zenon/rpc/server"
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

	mu               sync.RWMutex
	mode             string
	client           *rpc_client.RpcClient
	governance       *governance.API
	governanceClient *server.Client
	url              string
	height           uint64
	chainID          uint64
	stop             chan struct{}
	// connGen identifies the latest connection intent. Every disconnect (and thus
	// every SetNode/Connect/Disconnect) bumps it; a dial that finishes late must
	// find its captured gen still current or it may not install — otherwise two
	// overlapping connects could leave the wallet on whichever endpoint finished
	// LAST rather than the one selected last, leaking the loser's client.
	connGen uint64

	// opMu serializes whole node-mode transitions (SetNodeMode, Connect,
	// SetNodeURL, Disconnect, DeleteEmbeddedData). A transition is a multi-step
	// sequence — persist mode, stop/start the embedded node, set n.mode, dial —
	// and overlapping transitions could otherwise interleave those steps so the
	// persisted mode, in-memory mode, embedded lifecycle, and connected URL
	// belong to different requests. Lock order: opMu before mu, never inverse.
	opMu sync.Mutex

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

// setNode connects to url, verifies reachability, and starts the momentum
// subscription that drives status/height events. URL persistence belongs to
// SetNodeMode/SetNodeURL; setNode is the raw connector. It is UNEXPORTED on
// purpose: it must only run inside opMu-protected transitions — a direct
// WebView call could otherwise win the connection generation and pair the
// persisted/displayed mode with an arbitrary endpoint.
func (n *NodeService) setNode(url string) error {
	n.mu.Lock()
	n.disconnectLocked()
	gen := n.connGen
	n.mu.Unlock()

	client, err := rpc_client.NewRpcClient(url)
	if err != nil {
		// A dial can take seconds; only emit if this call still owns the
		// connection intent — never over a newer connection's status.
		n.emitDisconnectedIfCurrent(gen)
		return fmt.Errorf("connect: %w", err)
	}
	m, err := client.LedgerApi.GetFrontierMomentum()
	if err != nil {
		client.Stop()
		n.emitDisconnectedIfCurrent(gen)
		return fmt.Errorf("node unreachable: %w", err)
	}
	// Governance is not exposed by the stable v0.2 SDK surface.
	// Keep the shipped testnet feature through the app-local adapter, using its
	// own raw caller because RpcClient does not expose its underlying transport.
	governanceClient, err := server.Dial(url)
	if err != nil {
		client.Stop()
		n.emitDisconnectedIfCurrent(gen)
		return fmt.Errorf("connect governance transport: %w", err)
	}
	governanceAPI := governance.NewAPI(governanceClient)

	if !n.installConnection(client, governanceClient, governanceAPI, url, m.Height, m.ChainIdentifier, gen) {
		governanceClient.Close()
		client.Stop()
		return errors.New("connection attempt superseded by a newer request")
	}

	if err := n.startMomentumLoop(gen); err != nil {
		// Subscription failed: the connection cannot deliver ticks, so it must
		// not stay installed OR keep reporting connected. degradeConnection
		// tears down and emits disconnected atomically iff gen still owns the
		// connection — a stale failure never touches a newer connection's state.
		n.degradeConnection(gen)
		return fmt.Errorf("subscribe to momentums: %w", err)
	}
	return nil
}

// degradeConnection tears down the connection installed under gen and emits a
// disconnected status — as ONE critical section, so the emitted snapshot can
// never interleave with (or override) a newer connection's status. Returns
// false without touching anything when gen has been superseded.
func (n *NodeService) degradeConnection(gen uint64) bool {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.connGen != gen {
		return false
	}
	n.disconnectLocked()
	n.emitStatusLocked(false)
	return true
}

// installConnection publishes a successfully dialed client as the current
// connection iff gen is still the latest connection intent, and emits the
// connected status inside the same critical section (so a concurrent teardown
// can never emit BETWEEN install and emit and be overridden by our stale
// snapshot). Returns false when superseded — the caller must Stop the orphaned
// client.
func (n *NodeService) installConnection(client *rpc_client.RpcClient, governanceClient *server.Client, governanceAPI *governance.API, url string, height, chainID, gen uint64) bool {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.connGen != gen {
		return false
	}
	n.client = client
	n.governanceClient = governanceClient
	n.governance = governanceAPI
	n.url = url
	n.height = height
	n.chainID = chainID
	n.emitStatusLocked(true)
	return true
}

// emitStatusLocked emits a status snapshot composed from the CURRENT fields.
// Callers must hold n.mu (write lock); emitting under the lock is what makes
// status transitions atomic with the state change they describe. (No Go-side
// event handlers exist, so EventsEmit cannot re-enter NodeService.)
func (n *NodeService) emitStatusLocked(connected bool) {
	if n.ctx == nil {
		return
	}
	mode := n.mode
	if mode == "" {
		mode = "remote"
	}
	st := NodeStatus{Mode: mode, Connected: connected && n.client != nil, Height: n.height, ChainID: n.chainID}
	runtime.EventsEmit(n.ctx, EventNodeStatus, st)
}

// emitDisconnectedIfCurrent emits a disconnected status only if gen is still
// the current connection intent — a failed dial from a superseded attempt must
// not paint a newer successful connection as disconnected.
func (n *NodeService) emitDisconnectedIfCurrent(gen uint64) {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.connGen != gen {
		return
	}
	n.emitStatusLocked(false)
}

// startMomentumLoop starts the subscription goroutine and returns an error if
// the subscription cannot be established. gen must be the connection intent the
// caller installed under; a superseded loop unsubscribes itself instead of
// racing the newer connection's loop. The caller must NOT hold n.mu.
func (n *NodeService) startMomentumLoop(gen uint64) error {
	n.mu.RLock()
	client := n.client
	subCtx := n.ctx
	current := n.connGen == gen
	n.mu.RUnlock()

	if !current || client == nil {
		return nil // superseded; the newer connection owns the loop
	}
	if subCtx == nil {
		subCtx = context.Background()
	}

	sub, ch, err := client.SubscriberApi.ToMomentums(subCtx)
	if err != nil {
		return err
	}

	n.mu.Lock()
	if n.connGen != gen {
		n.mu.Unlock()
		sub.Unsubscribe()
		return nil // superseded while subscribing
	}
	n.stop = make(chan struct{})
	stop := n.stop
	n.mu.Unlock()

	go func() {
		defer sub.Unsubscribe()
		for {
			select {
			case <-stop:
				return
			case ms, ok := <-ch:
				if !ok {
					// Unexpected channel closure: the connection is dead, not just
					// quiet. If this loop still owns the current generation, tear the
					// connection down and emit disconnected — otherwise the UI shows
					// a healthy node whose height never advances. A superseded loop
					// exits silently; the replacement connection owns status.
					n.degradeConnection(gen)
					return
				}
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
	// Any teardown is a new connection intent: an in-flight dial captured under
	// the previous gen must not install its client afterwards.
	n.connGen++
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
	if n.governanceClient != nil {
		n.governanceClient.Close()
		n.governanceClient = nil
	}
	n.governance = nil
	// Reset the cached chain identifier so a stale value (e.g. testnet) can't be
	// read by currentChainID() after disconnect and bypass the mainnet guard.
	n.chainID = 0
	n.height = 0
}

// Disconnect closes the connection and stops the subscription.
func (n *NodeService) Disconnect() error {
	n.opMu.Lock()
	defer n.opMu.Unlock()
	n.mu.Lock()
	n.disconnectLocked()
	n.emitStatusLocked(false)
	n.mu.Unlock()
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
	// One transition at a time: persist-mode, embedded stop/start, n.mode, and
	// the connect must all belong to THIS request, never interleaved with a
	// concurrent Apply/Connect.
	n.opMu.Lock()
	defer n.opMu.Unlock()

	// One settings mutation captures both the persisted mode and the URL the
	// transition will dial — no separate re-read another writer could slip into.
	var target string
	if err := n.config.updateSettings(func(s *Settings) error {
		s.NodeMode = mode
		target = s.ActiveNodeURL()
		return nil
	}); err != nil {
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
	return n.setNode(target)
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
	if cerr := n.setNode(h.WSURL()); cerr != nil {
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
	// Serialized with mode transitions: persisting a URL and reconnecting to it
	// is itself a transition step and must not interleave with SetNodeMode.
	n.opMu.Lock()
	defer n.opMu.Unlock()

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
		return n.setNode(url)
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

// noteSyncHeight folds a sync-poller ledger-height sample into the status
// height (monotonic, zero samples ignored). During embedded bulk sync the
// momentum subscription delivers only sparsely, so without this the status
// height — the sidebar pill — lags the live sync progress by whole epochs.
// Emits node:status only when the height actually advances. The monotonic
// guard also neutralizes a straggler sample from a dying poller after a mode
// switch: it can never drag a fresher connection's height backwards.
func (n *NodeService) noteSyncHeight(h uint64) {
	if h == 0 {
		return
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	if h <= n.height {
		return
	}
	n.height = h
	n.emitStatusLocked(true)
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
				n.noteSyncHeight(info.CurrentHeight)
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
	// Serialized with mode transitions so the delete can't race an embedded
	// start between the running-check and the removal.
	n.opMu.Lock()
	defer n.opMu.Unlock()
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
	n.opMu.Lock()
	defer n.opMu.Unlock()

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
	return n.setNode(s.ActiveNodeURL())
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

// currentGovernance returns the app-local governance adapter installed for the
// active connection. It is nil whenever the node is disconnected.
func (n *NodeService) currentGovernance() *governance.API {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.governance
}

// currentChainID returns the connected node's chain identifier (0 if unknown).
func (n *NodeService) currentChainID() uint64 {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.chainID
}

// connectionSnapshot returns the client and its chain identifier read together
// under one lock, so a node transition cannot pair an old client with a new
// chain id (or vice-versa) between two separate accessor calls.
func (n *NodeService) connectionSnapshot() (*rpc_client.RpcClient, uint64) {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.client, n.chainID
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
		// Release the running slot on ANY exit — including an unexpected channel
		// closure — so a later StartAutoReceive can restart instead of seeing a
		// phantom "already running". No-op when StopAutoReceive already cleared it.
		defer releaseSlot()
		// Drain whatever is already pending (the subscription only delivers NEW
		// blocks). Then re-drain on every new-block signal.
		n.sweepUnreceived(client, addr, stop)
		for {
			select {
			case <-stop:
				return
			case _, ok := <-ch:
				if !ok {
					// Unexpected channel closure (connection died): exit instead of
					// spinning on a closed channel.
					return
				}
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
