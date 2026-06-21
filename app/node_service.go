package app

import (
	"context"
	"errors"
	"fmt"

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

	client *rpc_client.RpcClient
	url    string
	height uint64
	stop   chan struct{}
}

func newNodeService(c *ConfigService, w *WalletService) *NodeService {
	return &NodeService{config: c, wallet: w}
}

// SetNode connects to url, verifies reachability, persists it, and starts the
// momentum subscription that drives status/height events.
func (n *NodeService) SetNode(url string) error {
	n.disconnectLocked()
	client, err := rpc_client.NewRpcClient(url)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	m, err := client.LedgerApi.GetFrontierMomentum()
	if err != nil {
		client.Stop()
		return fmt.Errorf("node unreachable: %w", err)
	}
	n.client = client
	n.url = url
	n.height = m.Height

	if s, err := n.config.GetSettings(); err == nil {
		s.NodeURL = url
		_ = n.config.SetSettings(s)
	}
	n.emitStatus(true)
	n.startMomentumLoop()
	return nil
}

func (n *NodeService) startMomentumLoop() {
	n.stop = make(chan struct{})
	// Use the Wails context when available; fall back to Background so the
	// subscription can be established even before Wails calls startup.
	subCtx := n.ctx
	if subCtx == nil {
		subCtx = context.Background()
	}
	sub, ch, err := n.client.SubscriberApi.ToMomentums(subCtx)
	if err != nil {
		return
	}
	go func() {
		defer sub.Unsubscribe()
		for {
			select {
			case <-n.stop:
				return
			case ms := <-ch:
				for _, m := range ms {
					if m.Height > n.height {
						n.height = m.Height
					}
				}
				if n.ctx != nil {
					runtime.EventsEmit(n.ctx, EventMomentumTick, n.height)
				}
				n.emitStatus(true)
			}
		}
	}()
}

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
	n.disconnectLocked()
	n.emitStatus(false)
	return nil
}

// NodeStatus returns the current connection snapshot.
func (n *NodeService) NodeStatus() NodeStatus {
	return NodeStatus{Mode: "remote", Connected: n.client != nil, Syncing: false, Height: n.height, Peers: 0}
}

func (n *NodeService) emitStatus(connected bool) {
	if n.ctx == nil {
		return
	}
	st := NodeStatus{Mode: "remote", Connected: connected && n.client != nil, Height: n.height}
	runtime.EventsEmit(n.ctx, EventNodeStatus, st)
}

// GetBalances returns the active address's balances.
func (n *NodeService) GetBalances() ([]TokenBalance, error) {
	if n.client == nil {
		return nil, errors.New("not connected")
	}
	addr, ok := n.wallet.activeAddress()
	if !ok {
		return nil, errLocked
	}
	info, err := n.client.LedgerApi.GetAccountInfoByAddress(addr)
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
	if n.client == nil {
		return nil, errors.New("not connected")
	}
	addr, ok := n.wallet.activeAddress()
	if !ok {
		return nil, errLocked
	}
	list, err := n.client.LedgerApi.GetAccountBlocksByPage(addr, uint32(page), uint32(count))
	if err != nil {
		return nil, err
	}
	out := []TxRecord{}
	for _, b := range list.List {
		out = append(out, toTxRecord(b))
	}
	return out, nil
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
