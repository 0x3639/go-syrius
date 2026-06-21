// Package app holds the Wails-bound services that form the binding boundary
// between the Svelte frontend and the Go backend. It is one package so App can
// wire unexported context and dependencies into the services directly.
package app

import "context"

// App owns the service instances and the Wails runtime context.
type App struct {
	ctx    context.Context
	Config *ConfigService
	Wallet *WalletService
	Node   *NodeService
	Tx     *TxService
}

// New constructs the App and its services (not yet started).
func New() *App {
	cfg := newConfigService()
	w := newWalletService(cfg)
	n := newNodeService(cfg, w)
	t := newTxService(cfg, w, n)
	n.setReceiveFunc(t.Receive)
	w.setOnLock(t.clearPending)
	return &App{Config: cfg, Wallet: w, Node: n, Tx: t}
}

// OnStartup receives the Wails runtime context and distributes it.
func (a *App) OnStartup(ctx context.Context) {
	a.ctx = ctx
	a.Config.ctx = ctx
	a.Wallet.ctx = ctx
	a.Node.ctx = ctx
	a.Tx.ctx = ctx
}

// OnShutdown locks the wallet and disconnects the node on exit.
func (a *App) OnShutdown(ctx context.Context) {
	a.Node.StopAutoReceive()
	a.Node.stopEmbedded()
	_ = a.Wallet.Lock()
	_ = a.Node.Disconnect()
}

// Bindings is the list of structs whose exported methods Wails exposes to JS.
func (a *App) Bindings() []interface{} {
	return []interface{}{a.Config, a.Wallet, a.Node, a.Tx}
}
