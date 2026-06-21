package app

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/0x3639/znn-sdk-go/rpc_client"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"github.com/zenon-network/go-zenon/chain/nom"
	"github.com/zenon-network/go-zenon/common/types"
	api "github.com/zenon-network/go-zenon/rpc/api"
)

// NodeService owns the remote RPC connection and surfaces reads + status events.
type NodeService struct {
	ctx    context.Context
	config *ConfigService
	wallet *WalletService

	mu      sync.RWMutex
	client  *rpc_client.RpcClient
	url     string
	height  uint64
	chainID uint64
	stop    chan struct{}
}

func newNodeService(c *ConfigService, w *WalletService) *NodeService {
	return &NodeService{config: c, wallet: w}
}

// SetNode connects to url, verifies reachability, persists it, and starts the
// momentum subscription that drives status/height events.
func (n *NodeService) SetNode(url string) error {
	n.mu.Lock()
	n.disconnectLocked()
	n.mu.Unlock()

	client, err := rpc_client.NewRpcClient(url)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	m, err := client.LedgerApi.GetFrontierMomentum()
	if err != nil {
		client.Stop()
		return fmt.Errorf("node unreachable: %w", err)
	}

	n.mu.Lock()
	n.client = client
	n.url = url
	n.height = m.Height
	n.chainID = m.ChainIdentifier
	n.mu.Unlock()

	if s, err := n.config.GetSettings(); err == nil {
		s.NodeURL = url
		_ = n.config.SetSettings(s)
	}
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
	if n.client != nil {
		n.client.Stop()
		n.client = nil
	}
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
	n.mu.RUnlock()
	return NodeStatus{Mode: "remote", Connected: connected, Syncing: false, Height: height, Peers: 0}
}

func (n *NodeService) emitStatus(connected bool) {
	n.mu.RLock()
	ctx := n.ctx
	clientNil := n.client == nil
	height := n.height
	n.mu.RUnlock()

	if ctx == nil {
		return
	}
	st := NodeStatus{Mode: "remote", Connected: connected && !clientNil, Height: height}
	runtime.EventsEmit(ctx, EventNodeStatus, st)
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
