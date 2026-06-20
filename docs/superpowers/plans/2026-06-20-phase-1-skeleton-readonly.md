# Phase 1 — Wails Skeleton + Read-Only Wallet Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A Wails v2 desktop app that imports/unlocks an existing keystore and shows balances, address (copy + QR), transaction history, and live connection/sync status — strictly read-only.

**Architecture:** Go backend exposes three Wails-bound services (`ConfigService`, `WalletService`, `NodeService`) under `app/` (one Go package, so `App` sets unexported `ctx`/dependencies directly). Keystore read/derive uses **go-zenon's `wallet`** package; node reads use **znn-sdk-go** `rpc_client`/`LedgerApi`. The Svelte+TS+Tailwind frontend calls bindings (pull: balances/history) and subscribes to Wails events (push: status/momentum). No secret ever crosses the boundary.

**Tech Stack:** Go 1.24+, Wails v2, `github.com/0x3639/znn-sdk-go v0.1.16`, `github.com/zenon-network/go-zenon`, Svelte + TypeScript + Vite + Tailwind, Vitest, `qrcode`.

## Global Constraints

- Module path `github.com/0x3639/go-syrius`; `znn-sdk-go v0.1.16` pinned, **unmodified**; `go-zenon` is a direct dep used for all keystore work.
- **No secrets across the binding boundary:** no method returns a private key, seed, mnemonic, or decrypted keystore. Never log secrets.
- **Keystore reads use go-zenon**, never znn-sdk-go's wallet (it cannot read syrius keystores — `docs/compatibility-notes.md`).
- Default node URL `wss://my.hc1node.com:35998` (mainnet), user-editable.
- Data dir `os.UserConfigDir()/go-syrius`; wallets under `<dataDir>/wallets/`; settings at `<dataDir>/settings.json`.
- Amounts: 8-decimals base units; DTOs carry amounts as **base-unit decimal strings** (never float).
- Tests: offline suite (`go test ./...`) network-free and deterministic; live/connect tests behind `//go:build integration`; keystore tests read the gitignored `secrets/` and skip if absent. No secrets committed.
- Scope: read-only + keystore import only. No send, no wallet creation, no mnemonic import.

## File structure

```
main.go                       # Wails bootstrap; embeds frontend/dist; binds services
app/app.go                    # App struct; OnStartup wires ctx + deps into services
app/dto.go                    # DTOs (secret-free) + event-name constants
app/config_service.go         # data dir + settings persistence
app/wallet_service.go         # list/import/unlock/lock/accounts (go-zenon wallet)
app/node_service.go           # remote connection, status events, reads
app/*_test.go                 # Go unit tests
frontend/src/lib/stores/*.ts  # wallet, node, balances, txs
frontend/src/lib/components/*.svelte
frontend/src/routes/*.svelte  # Unlock, Dashboard
frontend/src/lib/format.ts    # amount/address formatting + its test
```

---

## Task 1: Scaffold the Wails app and merge into the module

**Files:**
- Create: `main.go`, `wails.json`, `frontend/` (Svelte-TS template), `build/`
- Modify: `go.mod`, `go.sum`
- Create: `app/app.go`

**Interfaces:**
- Produces: a buildable Wails app; `app.App` struct with `OnStartup(ctx context.Context)` storing `ctx`.

- [ ] **Step 1: Install the Wails CLI**

```bash
go install github.com/wailsapp/wails/v2/cmd/wails@latest
"$(go env GOPATH)/bin/wails" version   # expect v2.x
```

- [ ] **Step 2: Scaffold into a temp dir and merge**

```bash
cd /tmp && rm -rf syrius-scaffold
"$(go env GOPATH)/bin/wails" init -n syrius -t svelte-ts -d /tmp/syrius-scaffold
cd /Users/dfriestedt/Documents/go-syrius
cp -R /tmp/syrius-scaffold/frontend ./frontend
cp -R /tmp/syrius-scaffold/build ./build
cp /tmp/syrius-scaffold/wails.json ./wails.json
# Take main.go from scaffold but we will overwrite app.go with our own (Step 4).
cp /tmp/syrius-scaffold/main.go ./main.go
```

- [ ] **Step 3: Reconcile go.mod (keep our module path + deps, add Wails)**

```bash
go get github.com/wailsapp/wails/v2@v2.10.1
go mod tidy
```
Edit `wails.json` so `"name": "syrius"` and the frontend install/build commands use pnpm:
```json
{ "frontend:install": "pnpm install", "frontend:build": "pnpm run build" }
```

- [ ] **Step 4: Replace the scaffold App with our package-based App**

Overwrite `main.go`:
```go
package main

import (
	"embed"

	"github.com/0x3639/go-syrius/app"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	a := app.New()
	if err := wails.Run(&options.App{
		Title:  "syrius",
		Width:  1100,
		Height: 720,
		AssetServer: &assetserver.Options{Assets: assets},
		OnStartup:  a.OnStartup,
		OnShutdown: a.OnShutdown,
		Bind:       a.Bindings(),
	}); err != nil {
		panic(err)
	}
}
```

Create `app/app.go`:
```go
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
}

// New constructs the App and its services (not yet started).
func New() *App {
	cfg := newConfigService()
	w := newWalletService(cfg)
	n := newNodeService(cfg, w)
	return &App{Config: cfg, Wallet: w, Node: n}
}

// OnStartup receives the Wails runtime context and distributes it.
func (a *App) OnStartup(ctx context.Context) {
	a.ctx = ctx
	a.Config.ctx = ctx
	a.Wallet.ctx = ctx
	a.Node.ctx = ctx
}

// OnShutdown locks the wallet and disconnects the node on exit.
func (a *App) OnShutdown(ctx context.Context) {
	_ = a.Wallet.Lock()
	_ = a.Node.Disconnect()
}

// Bindings is the list of structs whose exported methods Wails exposes to JS.
func (a *App) Bindings() []interface{} {
	return []interface{}{a.Config, a.Wallet, a.Node}
}
```

> Note: `newConfigService`, `newWalletService`, `newNodeService`, and the `ctx`/`Lock`/`Disconnect` members are defined in Tasks 2–4. Until those exist this file will not compile; implement Task 1 through Step 3 (buildable scaffold), then return to verify the full build at the end of Task 4. For Step 5 below, temporarily stub `app.New()` returning `&App{}` with no services if you want an early window — but the recommended path is to do Tasks 2–4 before the first full `wails build`.

- [ ] **Step 5: Verify the toolchain builds the frontend**

```bash
cd frontend && pnpm install && pnpm run build && cd ..
```
Expected: `frontend/dist` produced, no errors.

- [ ] **Step 6: Commit**

```bash
git add main.go wails.json frontend build go.mod go.sum app/app.go
git commit -m "chore: scaffold Wails svelte-ts app and merge into module"
```

---

## Task 2: ConfigService (data dir + settings)

**Files:**
- Create: `app/config_service.go`
- Create: `app/dto.go` (Settings DTO + event names)
- Test: `app/config_service_test.go`

**Interfaces:**
- Produces: `newConfigService() *ConfigService`; `(*ConfigService) GetSettings() (Settings, error)`; `SetSettings(Settings) error`; unexported `dataDir() (string, error)`, `walletsDir() (string, error)`, field `ctx context.Context`.
- `Settings{ NodeURL string; Theme string; LastWallet string; ActiveAccount int }`.

- [ ] **Step 1: Write DTOs + event names**

`app/dto.go`:
```go
package app

// Event names emitted to the frontend.
const (
	EventNodeStatus   = "node:status"
	EventMomentumTick = "momentum:tick"
	EventWalletLocked = "wallet:locked"
)

// Settings is the persisted user configuration.
type Settings struct {
	NodeURL       string `json:"nodeUrl"`
	Theme         string `json:"theme"`
	LastWallet    string `json:"lastWallet"`
	ActiveAccount int    `json:"activeAccount"`
}

// WalletMeta identifies a keystore without exposing secrets.
type WalletMeta struct {
	Name        string `json:"name"`
	BaseAddress string `json:"baseAddress"`
}

// AccountInfo is one derived account.
type AccountInfo struct {
	Index   int    `json:"index"`
	Address string `json:"address"`
}

// NodeStatus is the connection/sync snapshot pushed via EventNodeStatus.
type NodeStatus struct {
	Mode      string `json:"mode"`
	Connected bool   `json:"connected"`
	Syncing   bool   `json:"syncing"`
	Height    uint64 `json:"height"`
	Peers     int    `json:"peers"`
}

// TokenBalance is one token's balance for the active address.
type TokenBalance struct {
	Zts      string `json:"zts"`
	Symbol   string `json:"symbol"`
	Decimals int    `json:"decimals"`
	Amount   string `json:"amount"` // base-unit decimal string
}

// TxRecord is one account block in history.
type TxRecord struct {
	Hash           string `json:"hash"`
	Direction      string `json:"direction"` // "send" | "receive"
	Counterparty   string `json:"counterparty"`
	Token          string `json:"token"`
	Amount         string `json:"amount"`
	MomentumHeight uint64 `json:"momentumHeight"`
	Confirmed      bool   `json:"confirmed"`
	Timestamp      int64  `json:"timestamp"`
}

const defaultNodeURL = "wss://my.hc1node.com:35998"
```

- [ ] **Step 2: Write the failing test**

`app/config_service_test.go`:
```go
package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSettingsRoundTripAndDefaults(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GO_SYRIUS_DATA_DIR", dir)

	c := newConfigService()

	// Defaults on first read (no file yet).
	got, err := c.GetSettings()
	if err != nil {
		t.Fatalf("GetSettings: %v", err)
	}
	if got.NodeURL != defaultNodeURL || got.Theme != "dark" {
		t.Fatalf("defaults wrong: %+v", got)
	}

	// Round-trip.
	got.NodeURL = "ws://127.0.0.1:35998"
	got.ActiveAccount = 3
	if err := c.SetSettings(got); err != nil {
		t.Fatalf("SetSettings: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "settings.json")); err != nil {
		t.Fatalf("settings.json not written: %v", err)
	}
	again, err := c.GetSettings()
	if err != nil {
		t.Fatalf("GetSettings 2: %v", err)
	}
	if again.NodeURL != "ws://127.0.0.1:35998" || again.ActiveAccount != 3 {
		t.Fatalf("round-trip mismatch: %+v", again)
	}
}

func TestWalletsDirCreated(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GO_SYRIUS_DATA_DIR", dir)
	c := newConfigService()
	wd, err := c.walletsDir()
	if err != nil {
		t.Fatalf("walletsDir: %v", err)
	}
	if _, err := os.Stat(wd); err != nil {
		t.Fatalf("wallets dir not created: %v", err)
	}
}
```

- [ ] **Step 3: Run to verify failure**

Run: `go test ./app/ -run 'TestSettings|TestWalletsDir' -v`
Expected: FAIL — `undefined: newConfigService`.

- [ ] **Step 4: Implement ConfigService**

`app/config_service.go`:
```go
package app

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
)

// ConfigService resolves the data directory and persists user settings.
type ConfigService struct {
	ctx context.Context
}

func newConfigService() *ConfigService { return &ConfigService{} }

// dataDir is the app data directory; GO_SYRIUS_DATA_DIR overrides it (tests).
func (c *ConfigService) dataDir() (string, error) {
	if d := os.Getenv("GO_SYRIUS_DATA_DIR"); d != "" {
		if err := os.MkdirAll(d, 0o700); err != nil {
			return "", err
		}
		return d, nil
	}
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	d := filepath.Join(base, "go-syrius")
	if err := os.MkdirAll(d, 0o700); err != nil {
		return "", err
	}
	return d, nil
}

func (c *ConfigService) walletsDir() (string, error) {
	d, err := c.dataDir()
	if err != nil {
		return "", err
	}
	wd := filepath.Join(d, "wallets")
	if err := os.MkdirAll(wd, 0o700); err != nil {
		return "", err
	}
	return wd, nil
}

func defaultSettings() Settings {
	return Settings{NodeURL: defaultNodeURL, Theme: "dark", ActiveAccount: 0}
}

// GetSettings returns persisted settings, or defaults if none exist.
func (c *ConfigService) GetSettings() (Settings, error) {
	d, err := c.dataDir()
	if err != nil {
		return Settings{}, err
	}
	raw, err := os.ReadFile(filepath.Join(d, "settings.json"))
	if os.IsNotExist(err) {
		return defaultSettings(), nil
	}
	if err != nil {
		return Settings{}, err
	}
	s := defaultSettings()
	if err := json.Unmarshal(raw, &s); err != nil {
		return Settings{}, err
	}
	return s, nil
}

// SetSettings persists settings as JSON.
func (c *ConfigService) SetSettings(s Settings) error {
	d, err := c.dataDir()
	if err != nil {
		return err
	}
	raw, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(d, "settings.json"), raw, 0o600)
}
```

- [ ] **Step 5: Run to verify pass**

Run: `go test ./app/ -run 'TestSettings|TestWalletsDir' -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add app/config_service.go app/dto.go app/config_service_test.go
git commit -m "feat(app): ConfigService with data dir and settings persistence"
```

---

## Task 3: WalletService (go-zenon wallet)

**Files:**
- Create: `app/wallet_service.go`
- Test: `app/wallet_service_test.go`

**Interfaces:**
- Consumes: `*ConfigService` (`walletsDir`), `EventWalletLocked`, DTOs `WalletMeta`/`AccountInfo`.
- Produces: `newWalletService(*ConfigService) *WalletService`; `ListWallets() ([]WalletMeta, error)`; `ImportKeystore(srcPath string) (WalletMeta, error)`; `Unlock(name, password string) error`; `Lock() error`; `CurrentAccounts() ([]AccountInfo, error)`; `SelectAccount(index int) error`; unexported `activeAddress() (types.Address, bool)`, field `ctx context.Context`. Holds the decrypted `*wallet.KeyStore`.
- Account range derived: indices 0–9.

- [ ] **Step 1: Write the failing test**

`app/wallet_service_test.go`:
```go
package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// locateSecretsKeystore returns the gitignored real keystore + password, or skips.
func locateSecretsKeystore(t *testing.T) (path, password string) {
	t.Helper()
	ks := "../secrets/pillar.json"
	if _, err := os.Stat(ks); err != nil {
		t.Skip("no secrets/pillar.json; skipping wallet integration-ish test")
	}
	raw, err := os.ReadFile("../secrets/pillar-password.txt")
	if err != nil {
		t.Skip("no secrets/pillar-password.txt")
	}
	return ks, strings.TrimSpace(string(raw))
}

func newTestWalletService(t *testing.T) *WalletService {
	t.Helper()
	t.Setenv("GO_SYRIUS_DATA_DIR", t.TempDir())
	return newWalletService(newConfigService())
}

func TestImportListUnlockLock(t *testing.T) {
	ksPath, pw := locateSecretsKeystore(t)
	w := newTestWalletService(t)

	meta, err := w.ImportKeystore(ksPath)
	if err != nil {
		t.Fatalf("ImportKeystore: %v", err)
	}
	if !strings.HasPrefix(meta.BaseAddress, "z1") {
		t.Fatalf("bad baseAddress: %q", meta.BaseAddress)
	}

	list, err := w.ListWallets()
	if err != nil || len(list) != 1 {
		t.Fatalf("ListWallets = %v, %v", list, err)
	}

	if err := w.Unlock(meta.Name, "wrong-password"); err == nil {
		t.Fatal("expected unlock to fail with wrong password")
	}
	if err := w.Unlock(meta.Name, pw); err != nil {
		t.Fatalf("Unlock: %v", err)
	}

	accts, err := w.CurrentAccounts()
	if err != nil || len(accts) != 10 {
		t.Fatalf("CurrentAccounts = %v (len %d), %v", accts, len(accts), err)
	}
	if accts[0].Address != meta.BaseAddress {
		t.Fatalf("index-0 %s != baseAddress %s", accts[0].Address, meta.BaseAddress)
	}

	if err := w.Lock(); err != nil {
		t.Fatalf("Lock: %v", err)
	}
	if _, err := w.CurrentAccounts(); err == nil {
		t.Fatal("expected CurrentAccounts to fail after Lock")
	}
}

func TestImportRejectsNonKeystore(t *testing.T) {
	w := newTestWalletService(t)
	bad := filepath.Join(t.TempDir(), "notakeystore.json")
	if err := os.WriteFile(bad, []byte(`{"hello":"world"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := w.ImportKeystore(bad); err == nil {
		t.Fatal("expected ImportKeystore to reject a non-keystore file")
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./app/ -run TestImport -v`
Expected: FAIL — `undefined: newWalletService`.

- [ ] **Step 3: Implement WalletService**

`app/wallet_service.go`:
```go
package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	"github.com/zenon-network/go-zenon/common/types"
	"github.com/zenon-network/go-zenon/wallet"
)

const accountRange = 10

var errLocked = errors.New("wallet is locked")

// WalletService imports, unlocks, and derives accounts from syrius keystores
// using go-zenon's canonical wallet implementation.
type WalletService struct {
	ctx    context.Context
	config *ConfigService

	keystore *wallet.KeyStore // nil when locked
	active   int
}

func newWalletService(c *ConfigService) *WalletService {
	return &WalletService{config: c}
}

// ListWallets returns metadata for each keystore file, without decrypting.
func (w *WalletService) ListWallets() ([]WalletMeta, error) {
	dir, err := w.config.walletsDir()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	out := []WalletMeta{}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		kf, err := wallet.ReadKeyFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue // not a valid keystore; skip
		}
		out = append(out, WalletMeta{Name: e.Name(), BaseAddress: kf.BaseAddress.String()})
	}
	return out, nil
}

// ImportKeystore validates a keystore file and copies it into the wallets dir.
func (w *WalletService) ImportKeystore(srcPath string) (WalletMeta, error) {
	kf, err := wallet.ReadKeyFile(srcPath)
	if err != nil {
		return WalletMeta{}, fmt.Errorf("not a valid syrius keystore: %w", err)
	}
	dir, err := w.config.walletsDir()
	if err != nil {
		return WalletMeta{}, err
	}
	name := filepath.Base(srcPath)
	dst := filepath.Join(dir, name)
	if _, err := os.Stat(dst); err == nil {
		return WalletMeta{}, fmt.Errorf("a wallet named %q already exists", name)
	}
	if err := copyFile(srcPath, dst); err != nil {
		return WalletMeta{}, err
	}
	return WalletMeta{Name: name, BaseAddress: kf.BaseAddress.String()}, nil
}

// Unlock decrypts the named keystore and holds it in memory.
func (w *WalletService) Unlock(name, password string) error {
	dir, err := w.config.walletsDir()
	if err != nil {
		return err
	}
	kf, err := wallet.ReadKeyFile(filepath.Join(dir, name))
	if err != nil {
		return fmt.Errorf("cannot read keystore: %w", err)
	}
	ks, err := kf.Decrypt(password)
	if err != nil {
		return errors.New("incorrect password")
	}
	w.keystore = ks
	w.active = 0
	return nil
}

// Lock zeroes and drops the decrypted keystore.
func (w *WalletService) Lock() error {
	if w.keystore != nil {
		w.keystore.Zero()
		w.keystore = nil
	}
	if w.ctx != nil {
		runtime.EventsEmit(w.ctx, EventWalletLocked)
	}
	return nil
}

// CurrentAccounts derives indices 0..accountRange-1 from the unlocked keystore.
func (w *WalletService) CurrentAccounts() ([]AccountInfo, error) {
	if w.keystore == nil {
		return nil, errLocked
	}
	out := make([]AccountInfo, 0, accountRange)
	for i := 0; i < accountRange; i++ {
		_, kp, err := w.keystore.DeriveForIndexPath(uint32(i))
		if err != nil {
			return nil, err
		}
		out = append(out, AccountInfo{Index: i, Address: kp.Address.String()})
	}
	return out, nil
}

// SelectAccount sets the active account index and persists it.
func (w *WalletService) SelectAccount(index int) error {
	if w.keystore == nil {
		return errLocked
	}
	if index < 0 || index >= accountRange {
		return fmt.Errorf("account index %d out of range", index)
	}
	w.active = index
	s, err := w.config.GetSettings()
	if err == nil {
		s.ActiveAccount = index
		_ = w.config.SetSettings(s)
	}
	return nil
}

// activeAddress returns the active account's address, false if locked.
func (w *WalletService) activeAddress() (types.Address, bool) {
	if w.keystore == nil {
		return types.Address{}, false
	}
	_, kp, err := w.keystore.DeriveForIndexPath(uint32(w.active))
	if err != nil {
		return types.Address{}, false
	}
	return kp.Address, true
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}
```

- [ ] **Step 4: Run to verify pass**

Run: `go test ./app/ -run TestImport -v`
Expected: PASS (the unlock test runs only if `secrets/` is present; otherwise it skips, and `TestImportRejectsNonKeystore` still passes).

- [ ] **Step 5: Commit**

```bash
git add app/wallet_service.go app/wallet_service_test.go
git commit -m "feat(app): WalletService (import/unlock/lock/accounts via go-zenon)"
```

---

## Task 4: NodeService (connection, events, reads)

**Files:**
- Create: `app/node_service.go`
- Test: `app/node_service_test.go`

**Interfaces:**
- Consumes: `*ConfigService`, `*WalletService` (`activeAddress`), DTOs `NodeStatus`/`TokenBalance`/`TxRecord`, events `EventNodeStatus`/`EventMomentumTick`.
- Produces: `newNodeService(*ConfigService, *WalletService) *NodeService`; `SetNode(url string) error`; `Disconnect() error`; `NodeStatus() NodeStatus`; `GetBalances() ([]TokenBalance, error)`; `GetTransactions(page, count int) ([]TxRecord, error)`; unexported `toTokenBalance`/`toTxRecord` mappers; field `ctx context.Context`.

- [ ] **Step 1: Write the failing test (pure DTO mappers + status; no network)**

`app/node_service_test.go`:
```go
package app

import (
	"math/big"
	"testing"

	"github.com/zenon-network/go-zenon/chain/nom"
	"github.com/zenon-network/go-zenon/common/types"
	api "github.com/zenon-network/go-zenon/rpc/api"
)

func TestToTokenBalance(t *testing.T) {
	bi := &api.BalanceInfo{
		TokenInfo: &api.Token{TokenStandard: types.ZnnTokenStandard, TokenSymbol: "ZNN", Decimals: 8},
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

func TestStatusDefaults(t *testing.T) {
	n := newNodeService(newConfigService(), newWalletService(newConfigService()))
	s := n.NodeStatus()
	if s.Connected || s.Mode != "remote" {
		t.Fatalf("status = %+v", s)
	}
}
```

> Verify the embedded-field access in your environment: `api.AccountBlock` embeds `nom.AccountBlock`. If the embedded field name differs, adjust the literal (`send.AccountBlock = nom.AccountBlock{...}`) — confirm with `go doc github.com/zenon-network/go-zenon/rpc/api.AccountBlock`.

- [ ] **Step 2: Run to verify failure**

Run: `go test ./app/ -run 'TestToToken|TestToTx|TestStatus' -v`
Expected: FAIL — `undefined: newNodeService`/`toTokenBalance`.

- [ ] **Step 3: Implement NodeService**

`app/node_service.go`:
```go
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
	sub, ch, err := n.client.SubscriberApi.ToMomentums(n.ctx)
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
```

> `nom.IsSendBlock(blockType uint64) bool` exists in go-zenon `chain/nom`. Confirm with `go doc github.com/zenon-network/go-zenon/chain/nom.IsSendBlock`; if it takes a different type, cast `b.BlockType` accordingly.

- [ ] **Step 4: Run to verify pass + full module build**

Run: `go test ./app/ -run 'TestToToken|TestToTx|TestStatus' -v && go build ./...`
Expected: PASS and clean build (this completes Task 1's deferred full build — `app.New()` now compiles).

- [ ] **Step 5: Commit**

```bash
git add app/node_service.go app/node_service_test.go
git commit -m "feat(app): NodeService (remote connect, status events, balance/tx reads)"
```

---

## Task 5: Generate bindings and verify the app boots

**Files:**
- Modify: `frontend/wailsjs/` (generated)

**Interfaces:**
- Produces: TypeScript bindings under `frontend/wailsjs/go/app/*` for ConfigService/WalletService/NodeService, consumed by the frontend.

- [ ] **Step 1: Generate bindings**

```bash
"$(go env GOPATH)/bin/wails" generate module
ls frontend/wailsjs/go/app   # expect ConfigService.* WalletService.* NodeService.*
```

- [ ] **Step 2: Verify a dev build boots (manual)**

```bash
"$(go env GOPATH)/bin/wails" dev
```
Expected: a window opens (default template UI for now). Close it. (If running headless, `wails build` instead and confirm it produces `build/bin/syrius`.)

- [ ] **Step 3: Commit**

```bash
git add frontend/wailsjs
git commit -m "chore: generate Wails TS bindings for app services"
```

---

## Task 6: Frontend foundation (Tailwind tokens + formatting util)

**Files:**
- Modify: `frontend/src/app.css`, `frontend/tailwind.config.js` (create if absent), `frontend/postcss.config.js`
- Create: `frontend/src/lib/format.ts`, `frontend/src/lib/format.test.ts`
- Modify: `frontend/package.json` (add `tailwindcss`, `qrcode`, `vitest`)

**Interfaces:**
- Produces: `formatAmount(base: string, decimals: number): string`, `shortAddress(addr: string): string`.

- [ ] **Step 1: Add deps**

```bash
cd frontend
pnpm add -D tailwindcss@^3 postcss autoprefixer vitest @testing-library/svelte jsdom
pnpm add qrcode
npx tailwindcss init -p
cd ..
```

- [ ] **Step 2: Configure Tailwind tokens**

`frontend/tailwind.config.js`:
```js
/** @type {import('tailwindcss').Config} */
export default {
  darkMode: 'class',
  content: ['./index.html', './src/**/*.{svelte,ts}'],
  theme: {
    extend: {
      colors: {
        bg: '#0e0f13', surface: '#1a1c22', text: '#e6e8ee', muted: '#9aa0ad',
        accent: '#4f8cff', success: '#3fb950', warn: '#d29922', error: '#f85149',
      },
      fontFamily: { mono: ['ui-monospace', 'SFMono-Regular', 'monospace'] },
    },
  },
  plugins: [],
}
```

`frontend/src/app.css` (replace contents):
```css
@tailwind base;
@tailwind components;
@tailwind utilities;

:root { color-scheme: dark; }
body { @apply bg-bg text-text; }
```

- [ ] **Step 3: Write the failing format test**

`frontend/src/lib/format.test.ts`:
```ts
import { describe, it, expect } from 'vitest'
import { formatAmount, shortAddress } from './format'

describe('format', () => {
  it('formats base units by decimals', () => {
    expect(formatAmount('5000000000000', 8)).toBe('50000')
    expect(formatAmount('150000000', 8)).toBe('1.5')
    expect(formatAmount('0', 8)).toBe('0')
  })
  it('shortens addresses', () => {
    expect(shortAddress('z1qrr0dvun0p0nrsx6h9ppnfrgl8e6r7a8wpcjmg')).toBe('z1qrr0…pcjmg')
  })
})
```

Add to `frontend/package.json` scripts: `"test": "vitest run"`.

- [ ] **Step 4: Run to verify failure**

Run: `cd frontend && pnpm test`
Expected: FAIL — cannot resolve `./format`.

- [ ] **Step 5: Implement format util**

`frontend/src/lib/format.ts`:
```ts
// formatAmount converts a base-unit integer string to a human decimal string.
export function formatAmount(base: string, decimals: number): string {
  const neg = base.startsWith('-')
  const digits = (neg ? base.slice(1) : base).padStart(decimals + 1, '0')
  const intPart = digits.slice(0, digits.length - decimals)
  let frac = digits.slice(digits.length - decimals).replace(/0+$/, '')
  const out = frac ? `${intPart}.${frac}` : intPart
  return neg ? `-${out}` : out
}

// shortAddress renders z1xxxx…xxxxx for compact display.
export function shortAddress(addr: string): string {
  if (addr.length <= 12) return addr
  return `${addr.slice(0, 6)}…${addr.slice(-5)}`
}
```

- [ ] **Step 6: Run to verify pass**

Run: `cd frontend && pnpm test`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add frontend/package.json frontend/pnpm-lock.yaml frontend/tailwind.config.js frontend/postcss.config.js frontend/src/app.css frontend/src/lib/format.ts frontend/src/lib/format.test.ts
git commit -m "feat(frontend): Tailwind tokens + amount/address formatting util"
```

---

## Task 7: Frontend stores (wallet, node, balances, txs)

**Files:**
- Create: `frontend/src/lib/stores/wallet.ts`, `node.ts`, `balances.ts`, `txs.ts`
- Create: `frontend/src/lib/stores/wallet.test.ts`

**Interfaces:**
- Consumes: generated bindings `wailsjs/go/app/{WalletService,NodeService,ConfigService}`, `wailsjs/runtime`.
- Produces: Svelte stores: `wallet` (`{locked, accounts, active, walletName}` + `unlock/lock/select/refreshAccounts`), `node` (`NodeStatus`, started by `initNodeEvents()`), `balances` (`load()`), `txs` (`load(page,count)`).

- [ ] **Step 1: Write the failing store test (mocked bindings)**

`frontend/src/lib/stores/wallet.test.ts`:
```ts
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { get } from 'svelte/store'

vi.mock('../../../wailsjs/go/app/WalletService', () => ({
  Unlock: vi.fn().mockResolvedValue(undefined),
  Lock: vi.fn().mockResolvedValue(undefined),
  CurrentAccounts: vi.fn().mockResolvedValue([{ index: 0, address: 'z1qrr0...' }]),
  SelectAccount: vi.fn().mockResolvedValue(undefined),
}))

import { wallet, unlock, lock } from './wallet'

describe('wallet store', () => {
  beforeEach(() => { lock() })
  it('unlock populates accounts and clears locked', async () => {
    await unlock('pillar.json', 'pw')
    const s = get(wallet)
    expect(s.locked).toBe(false)
    expect(s.accounts.length).toBe(1)
    expect(s.walletName).toBe('pillar.json')
  })
})
```

- [ ] **Step 2: Run to verify failure**

Run: `cd frontend && pnpm test src/lib/stores/wallet.test.ts`
Expected: FAIL — cannot resolve `./wallet`.

- [ ] **Step 3: Implement stores**

`frontend/src/lib/stores/wallet.ts`:
```ts
import { writable } from 'svelte/store'
import * as W from '../../../wailsjs/go/app/WalletService'

export type Account = { index: number; address: string }
export type WalletState = { locked: boolean; walletName: string; accounts: Account[]; active: number }

export const wallet = writable<WalletState>({ locked: true, walletName: '', accounts: [], active: 0 })

export async function unlock(name: string, password: string): Promise<void> {
  await W.Unlock(name, password)
  const accounts = (await W.CurrentAccounts()) as Account[]
  wallet.set({ locked: false, walletName: name, accounts, active: 0 })
}

export function lock(): void {
  W.Lock().catch(() => {})
  wallet.set({ locked: true, walletName: '', accounts: [], active: 0 })
}

export async function select(index: number): Promise<void> {
  await W.SelectAccount(index)
  wallet.update((s) => ({ ...s, active: index }))
}
```

`frontend/src/lib/stores/node.ts`:
```ts
import { writable } from 'svelte/store'
import { EventsOn } from '../../../wailsjs/runtime/runtime'

export type NodeStatus = { mode: string; connected: boolean; syncing: boolean; height: number; peers: number }
export const node = writable<NodeStatus>({ mode: 'remote', connected: false, syncing: false, height: 0, peers: 0 })

// initNodeEvents wires backend push events into the store. Returns nothing;
// onTick is invoked on each momentum so callers can refresh pulled data.
export function initNodeEvents(onTick: () => void): void {
  EventsOn('node:status', (s: NodeStatus) => node.set(s))
  EventsOn('momentum:tick', () => onTick())
}
```

`frontend/src/lib/stores/balances.ts`:
```ts
import { writable } from 'svelte/store'
import * as N from '../../../wailsjs/go/app/NodeService'

export type TokenBalance = { zts: string; symbol: string; decimals: number; amount: string }
export const balances = writable<TokenBalance[]>([])

export async function loadBalances(): Promise<void> {
  try { balances.set((await N.GetBalances()) as TokenBalance[]) } catch { balances.set([]) }
}
```

`frontend/src/lib/stores/txs.ts`:
```ts
import { writable } from 'svelte/store'
import * as N from '../../../wailsjs/go/app/NodeService'

export type TxRecord = {
  hash: string; direction: string; counterparty: string; token: string
  amount: string; momentumHeight: number; confirmed: boolean; timestamp: number
}
export const txs = writable<TxRecord[]>([])

export async function loadTxs(page = 0, count = 25): Promise<void> {
  try { txs.set((await N.GetTransactions(page, count)) as TxRecord[]) } catch { txs.set([]) }
}
```

- [ ] **Step 4: Run to verify pass**

Run: `cd frontend && pnpm test src/lib/stores/wallet.test.ts`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/lib/stores
git commit -m "feat(frontend): wallet/node/balances/txs stores with event wiring"
```

---

## Task 8: Unlock route (picker, password, import)

**Files:**
- Create: `frontend/src/lib/components/PasswordInput.svelte`, `WalletPicker.svelte`
- Create: `frontend/src/routes/Unlock.svelte`
- Create: `frontend/src/routes/Unlock.test.ts`

**Interfaces:**
- Consumes: `wallet`/`unlock` store, `WalletService.ListWallets`/`ImportKeystore`, runtime `OpenFileDialog`.
- Produces: `Unlock.svelte` emitting an `unlocked` event (or navigating) on success.

- [ ] **Step 1: Write the failing component test (mocked bindings)**

`frontend/src/routes/Unlock.test.ts`:
```ts
import { describe, it, expect, vi } from 'vitest'
import { render, fireEvent, screen } from '@testing-library/svelte'

vi.mock('../../wailsjs/go/app/WalletService', () => ({
  ListWallets: vi.fn().mockResolvedValue([{ name: 'pillar.json', baseAddress: 'z1qrr0dvun0p0nrsx6h9ppnfrgl8e6r7a8wpcjmg' }]),
  Unlock: vi.fn().mockResolvedValue(undefined),
  CurrentAccounts: vi.fn().mockResolvedValue([{ index: 0, address: 'z1qrr0dvun0p0nrsx6h9ppnfrgl8e6r7a8wpcjmg' }]),
  ImportKeystore: vi.fn(),
  Lock: vi.fn().mockResolvedValue(undefined),
}))
vi.mock('../../wailsjs/runtime/runtime', () => ({ EventsOn: vi.fn() }))

import Unlock from './Unlock.svelte'

describe('Unlock', () => {
  it('lists wallets and shows an unlock control', async () => {
    render(Unlock)
    expect(await screen.findByText(/pillar\.json/)).toBeTruthy()
  })
  it('shows an error on wrong password', async () => {
    const W = await import('../../wailsjs/go/app/WalletService')
    ;(W.Unlock as any).mockRejectedValueOnce(new Error('incorrect password'))
    render(Unlock)
    await screen.findByText(/pillar\.json/)
    await fireEvent.input(screen.getByLabelText(/password/i), { target: { value: 'x' } })
    await fireEvent.click(screen.getByRole('button', { name: /unlock/i }))
    expect(await screen.findByText(/incorrect password/i)).toBeTruthy()
  })
})
```

- [ ] **Step 2: Run to verify failure**

Run: `cd frontend && pnpm test src/routes/Unlock.test.ts`
Expected: FAIL — cannot resolve `./Unlock.svelte`.

- [ ] **Step 3: Implement components + route**

`frontend/src/lib/components/PasswordInput.svelte`:
```svelte
<script lang="ts">
  export let value = ''
  export let label = 'Password'
</script>
<label class="block text-sm text-muted">{label}
  <input type="password" bind:value aria-label={label}
    class="mt-1 w-full rounded bg-surface px-3 py-2 text-text outline-none focus:ring-2 focus:ring-accent" />
</label>
```

`frontend/src/lib/components/WalletPicker.svelte`:
```svelte
<script lang="ts">
  import type { WalletMeta } from '../../wailsjs/go/models'
  export let wallets: { name: string; baseAddress: string }[] = []
  export let selected = ''
</script>
<ul class="space-y-1">
  {#each wallets as w}
    <li>
      <button class="w-full rounded px-3 py-2 text-left {selected === w.name ? 'bg-accent/20' : 'bg-surface'}"
        on:click={() => (selected = w.name)}>
        <div class="text-text">{w.name}</div>
        <div class="font-mono text-xs text-muted">{w.baseAddress}</div>
      </button>
    </li>
  {/each}
</ul>
```

`frontend/src/routes/Unlock.svelte`:
```svelte
<script lang="ts">
  import { createEventDispatcher, onMount } from 'svelte'
  import * as W from '../../wailsjs/go/app/WalletService'
  import { OpenFileDialog } from '../../wailsjs/runtime/runtime'
  import { unlock } from '../lib/stores/wallet'
  import PasswordInput from '../lib/components/PasswordInput.svelte'
  import WalletPicker from '../lib/components/WalletPicker.svelte'

  const dispatch = createEventDispatcher()
  let wallets: { name: string; baseAddress: string }[] = []
  let selected = ''
  let password = ''
  let error = ''
  let busy = false

  async function refresh() { wallets = (await W.ListWallets()) ?? []; if (!selected && wallets[0]) selected = wallets[0].name }
  onMount(refresh)

  async function doUnlock() {
    error = ''; busy = true
    try { await unlock(selected, password); dispatch('unlocked') }
    catch (e: any) { error = e?.message ?? String(e) }
    finally { busy = false; password = '' }
  }

  async function doImport() {
    error = ''
    try {
      const path = await OpenFileDialog({ title: 'Import keystore' } as any)
      if (!path) return
      await W.ImportKeystore(path as unknown as string)
      await refresh()
    } catch (e: any) { error = e?.message ?? String(e) }
  }
</script>

<div class="mx-auto mt-16 w-[28rem] space-y-4">
  <h1 class="text-xl">Unlock wallet</h1>
  {#if wallets.length === 0}
    <p class="text-muted">No wallets yet. Import a keystore to begin.</p>
  {:else}
    <WalletPicker {wallets} bind:selected />
    <PasswordInput bind:value={password} />
    <button class="w-full rounded bg-accent py-2 text-bg disabled:opacity-50"
      disabled={busy || !selected} on:click={doUnlock} aria-label="Unlock">Unlock</button>
  {/if}
  <button class="w-full rounded border border-muted/40 py-2 text-muted" on:click={doImport}>Import keystore…</button>
  {#if error}<p class="text-error" role="alert">{error}</p>{/if}
</div>
```

- [ ] **Step 4: Run to verify pass**

Run: `cd frontend && pnpm test src/routes/Unlock.test.ts`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/lib/components/PasswordInput.svelte frontend/src/lib/components/WalletPicker.svelte frontend/src/routes/Unlock.svelte frontend/src/routes/Unlock.test.ts
git commit -m "feat(frontend): unlock route with wallet picker, password, import"
```

---

## Task 9: Dashboard route (address+QR, balances, history, status, accounts)

**Files:**
- Create: `frontend/src/lib/components/AddressDisplay.svelte`, `BalanceList.svelte`, `TxHistory.svelte`, `StatusBar.svelte`, `AccountSwitcher.svelte`
- Create: `frontend/src/routes/Dashboard.svelte`
- Create: `frontend/src/routes/Dashboard.test.ts`
- Modify: `frontend/src/App.svelte` (route between Unlock and Dashboard)

**Interfaces:**
- Consumes: `wallet`, `node`, `balances`, `txs` stores; `format` util; `qrcode`.
- Produces: `Dashboard.svelte`; `App.svelte` toggling on `wallet.locked`.

- [ ] **Step 1: Write the failing dashboard test (mocked stores/bindings)**

`frontend/src/routes/Dashboard.test.ts`:
```ts
import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/svelte'

vi.mock('../../wailsjs/go/app/NodeService', () => ({
  GetBalances: vi.fn().mockResolvedValue([{ zts: 'zts1znn...', symbol: 'ZNN', decimals: 8, amount: '5000000000000' }]),
  GetTransactions: vi.fn().mockResolvedValue([]),
}))
vi.mock('../../wailsjs/runtime/runtime', () => ({ EventsOn: vi.fn(), ClipboardSetText: vi.fn() }))

import Dashboard from './Dashboard.svelte'
import { wallet } from '../lib/stores/wallet'

describe('Dashboard', () => {
  it('renders the active address and balances', async () => {
    wallet.set({ locked: false, walletName: 'pillar.json', active: 0,
      accounts: [{ index: 0, address: 'z1qrr0dvun0p0nrsx6h9ppnfrgl8e6r7a8wpcjmg' }] })
    render(Dashboard)
    expect(await screen.findByText(/ZNN/)).toBeTruthy()
    expect(await screen.findByText(/50000/)).toBeTruthy()
  })
})
```

- [ ] **Step 2: Run to verify failure**

Run: `cd frontend && pnpm test src/routes/Dashboard.test.ts`
Expected: FAIL — cannot resolve `./Dashboard.svelte`.

- [ ] **Step 3: Implement components + route**

`frontend/src/lib/components/StatusBar.svelte`:
```svelte
<script lang="ts">
  import { node } from '../stores/node'
</script>
<div class="flex items-center gap-3 text-xs text-muted">
  <span class="inline-block h-2 w-2 rounded-full {$node.connected ? 'bg-success' : 'bg-error'}"></span>
  <span>{$node.connected ? 'Connected' : 'Disconnected'}</span>
  <span>height {$node.height}</span>
  {#if $node.peers}<span>{$node.peers} peers</span>{/if}
</div>
```

`frontend/src/lib/components/AddressDisplay.svelte`:
```svelte
<script lang="ts">
  import { onMount } from 'svelte'
  import QRCode from 'qrcode'
  import { ClipboardSetText } from '../../wailsjs/runtime/runtime'
  export let address = ''
  let dataUrl = ''
  let copied = false
  onMount(async () => { if (address) dataUrl = await QRCode.toDataURL(address, { margin: 1, width: 160 }) })
  async function copy() { await ClipboardSetText(address); copied = true; setTimeout(() => (copied = false), 1200) }
</script>
<div class="flex items-center gap-4 rounded bg-surface p-4">
  {#if dataUrl}<img src={dataUrl} alt="address QR" class="h-32 w-32 rounded bg-white p-1" />{/if}
  <div class="min-w-0">
    <div class="break-all font-mono text-sm text-text">{address}</div>
    <button class="mt-2 rounded bg-accent/20 px-2 py-1 text-xs text-accent" on:click={copy}>{copied ? 'Copied' : 'Copy'}</button>
  </div>
</div>
```

`frontend/src/lib/components/BalanceList.svelte`:
```svelte
<script lang="ts">
  import { balances } from '../stores/balances'
  import { formatAmount } from '../format'
</script>
<div class="rounded bg-surface p-4">
  <h2 class="mb-2 text-sm text-muted">Balances</h2>
  {#each $balances as b}
    <div class="flex justify-between py-1">
      <span>{b.symbol || b.zts}</span>
      <span class="font-mono">{formatAmount(b.amount, b.decimals || 8)}</span>
    </div>
  {/each}
</div>
```

`frontend/src/lib/components/TxHistory.svelte`:
```svelte
<script lang="ts">
  import { txs } from '../stores/txs'
  import { formatAmount, shortAddress } from '../format'
</script>
<div class="rounded bg-surface p-4">
  <h2 class="mb-2 text-sm text-muted">Recent transactions</h2>
  {#if $txs.length === 0}<p class="text-muted">No transactions.</p>{/if}
  {#each $txs as t}
    <div class="flex justify-between border-b border-bg/40 py-1 text-sm">
      <span class="{t.direction === 'send' ? 'text-error' : 'text-success'}">{t.direction}</span>
      <span class="font-mono">{shortAddress(t.counterparty)}</span>
      <span class="font-mono">{formatAmount(t.amount, 8)} {t.token}</span>
    </div>
  {/each}
</div>
```

`frontend/src/lib/components/AccountSwitcher.svelte`:
```svelte
<script lang="ts">
  import { wallet, select } from '../stores/wallet'
  async function onChange(e: Event) { await select(Number((e.target as HTMLSelectElement).value)) }
</script>
<select class="rounded bg-surface px-2 py-1 text-sm" on:change={onChange} value={$wallet.active}>
  {#each $wallet.accounts as a}<option value={a.index}>Account {a.index}</option>{/each}
</select>
```

`frontend/src/routes/Dashboard.svelte`:
```svelte
<script lang="ts">
  import { onMount } from 'svelte'
  import { wallet, lock } from '../lib/stores/wallet'
  import { node, initNodeEvents } from '../lib/stores/node'
  import { loadBalances } from '../lib/stores/balances'
  import { loadTxs } from '../lib/stores/txs'
  import AddressDisplay from '../lib/components/AddressDisplay.svelte'
  import BalanceList from '../lib/components/BalanceList.svelte'
  import TxHistory from '../lib/components/TxHistory.svelte'
  import StatusBar from '../lib/components/StatusBar.svelte'
  import AccountSwitcher from '../lib/components/AccountSwitcher.svelte'

  $: active = $wallet.accounts.find((a) => a.index === $wallet.active)
  async function refresh() { await Promise.all([loadBalances(), loadTxs()]) }
  onMount(() => { initNodeEvents(refresh); refresh() })
  $: if ($wallet.active >= 0) refresh()
</script>

<div class="mx-auto mt-8 w-[44rem] space-y-4">
  <div class="flex items-center justify-between">
    <StatusBar />
    <div class="flex items-center gap-2">
      <AccountSwitcher />
      <button class="rounded border border-muted/40 px-2 py-1 text-xs text-muted" on:click={lock}>Lock</button>
    </div>
  </div>
  {#if active}<AddressDisplay address={active.address} />{/if}
  <BalanceList />
  <TxHistory />
</div>
```

Modify `frontend/src/App.svelte`:
```svelte
<script lang="ts">
  import './app.css'
  import { wallet } from './lib/stores/wallet'
  import Unlock from './routes/Unlock.svelte'
  import Dashboard from './routes/Dashboard.svelte'
</script>
{#if $wallet.locked}
  <Unlock />
{:else}
  <Dashboard />
{/if}
```

- [ ] **Step 4: Run to verify pass + full frontend build**

Run: `cd frontend && pnpm test && pnpm run build`
Expected: PASS and a clean production build.

- [ ] **Step 5: Commit**

```bash
git add frontend/src
git commit -m "feat(frontend): dashboard (address/QR, balances, history, status, accounts)"
```

---

## Task 10: Connect-on-start wiring + manual acceptance

**Files:**
- Modify: `frontend/src/main.ts` (or `App.svelte` onMount) to connect the node on startup using saved settings.

**Interfaces:**
- Consumes: `ConfigService.GetSettings`, `NodeService.SetNode`.

- [ ] **Step 1: Connect node on app start**

In `frontend/src/App.svelte`, extend the script to connect on mount:
```svelte
<script lang="ts">
  import './app.css'
  import { onMount } from 'svelte'
  import { wallet } from './lib/stores/wallet'
  import * as Cfg from '../wailsjs/go/app/ConfigService'
  import * as N from '../wailsjs/go/app/NodeService'
  import Unlock from './routes/Unlock.svelte'
  import Dashboard from './routes/Dashboard.svelte'
  onMount(async () => {
    try { const s = await Cfg.GetSettings(); if (s.nodeUrl) await N.SetNode(s.nodeUrl) } catch {}
  })
</script>
{#if $wallet.locked}<Unlock />{:else}<Dashboard />{/if}
```

(Adjust the relative import path to `wailsjs` to match `App.svelte`'s location.)

- [ ] **Step 2: Full verification**

```bash
go test ./...                 # offline backend suite green
cd frontend && pnpm test && pnpm run build && cd ..
"$(go env GOPATH)/bin/wails" build   # produces build/bin/syrius
```

- [ ] **Step 3: Manual acceptance (Gate exit)**

1. Run the app (`wails dev` or the built binary).
2. Import `secrets/pillar.json`, unlock with its password.
3. Confirm: active address + QR shown; ZNN/QSR balances correct against the default node; recent transactions listed; StatusBar shows Connected and a rising height; account switcher changes the address and refreshes balances; Lock returns to the unlock screen.

- [ ] **Step 4: Commit**

```bash
git add frontend/src/App.svelte
git commit -m "feat(frontend): connect to saved node on startup; Phase 1 complete"
```

---

## Self-Review

**Spec coverage:** ConfigService (T2), WalletService import/unlock/lock/accounts (T3), NodeService remote+status+reads (T4), bindings (T5), Tailwind tokens (T6), stores+events (T7), unlock route+import (T8), dashboard with address/QR/balances/history/status/accounts (T9), connect-on-start + acceptance (T10), Wails scaffold (T1). All spec sections map to a task.

**Placeholder scan:** No TBD/TODO. Two items are explicitly flagged for environment verification (the `api.AccountBlock` embedded-field literal in T4 Step 1, and `nom.IsSendBlock`'s parameter type) with the exact `go doc` command to confirm — these are real APIs, surfaced for confirmation, not placeholders.

**Type consistency:** DTO field names (camelCase JSON) match between `app/dto.go` and the TS store types. `activeAddress()/Lock()/Disconnect()` referenced in `app.go` (T1) are defined in T3/T4. Store function names (`unlock`/`lock`/`select`/`loadBalances`/`loadTxs`/`initNodeEvents`) are consistent across stores and routes. Event names (`node:status`, `momentum:tick`, `wallet:locked`) match between `dto.go` and `node.ts`.

**Known environment dependencies (flagged, not hidden):** exact Wails v2 patch version and the `OpenFileDialog`/`ClipboardSetText` runtime signatures should be confirmed against the installed Wails version during T1/T8; adjust imports if the generated `wailsjs/runtime` differs.
