package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	stdruntime "runtime"
	"strings"
	"time"

	"sync"

	sdkwallet "github.com/0x3639/znn-sdk-go/wallet"
	bip39 "github.com/tyler-smith/go-bip39"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"github.com/zenon-network/go-zenon/common/types"
	"github.com/zenon-network/go-zenon/wallet"
)

const accountRange = 10

// maxAccounts caps how many accounts a wallet can reveal (derivation indices).
const maxAccounts = 100

// accountCountFor returns how many accounts the given wallet reveals: the stored
// count clamped to [accountRange, maxAccounts]. Unset falls back to accountRange.
func accountCountFor(s Settings, wallet string) int {
	c := s.AccountCounts[wallet]
	if c < accountRange {
		return accountRange
	}
	if c > maxAccounts {
		return maxAccounts
	}
	return c
}

var errLocked = errors.New("wallet is locked")

// WalletService imports, unlocks, and derives accounts from syrius keystores
// using go-zenon's canonical wallet implementation.
type WalletService struct {
	ctx    context.Context
	config *ConfigService

	// mu protects keystore/active/gen. Go mutexes are NOT reentrant, so internal
	// callers must use the *Locked() helpers while public methods take the lock
	// once. No method calls another lock-taking method while holding mu.
	mu           sync.Mutex
	keystore     *wallet.KeyStore // nil when locked
	active       int
	activeWallet string // name of the unlocked keystore; "" when locked
	gen          uint64 // session generation, bumped on Unlock, Lock, and account switch

	// onSessionChange is invoked whenever the signing session changes — Lock and
	// account switch (e.g. clear the pending tx so it can't be signed by the
	// wrong key).
	onSessionChange func()

	// selMu serializes the COMPLETE account-selection operation — validation,
	// persistence, active-index mutation, and session invalidation — AND the
	// keystore/session swaps in Unlock and Lock, so a wallet-lifecycle change
	// can never interleave a selection mid-operation (a paused selection could
	// otherwise resolve against a different wallet than it validated). It is
	// separate from w.mu on purpose: other wallet methods must not block on
	// settings disk I/O, and Unlock's slow KDF runs before taking it.
	selMu sync.Mutex

	// Auto-lock watchdog state. autoMu is separate from w.mu so activity pings
	// and the 15s tick never contend with wallet operations; it is only ever
	// taken alone or strictly after a released w.mu (never nested inside it).
	autoMu       sync.Mutex
	lastActivity time.Time
	autoLockMins int           // 0 = Never
	autoLockStop chan struct{} // non-nil while the watchdog runs
	autoLockTick time.Duration // test seam; zero → 15s

	// beforeSelectPersist, when non-nil, runs between a selection's in-memory
	// switch and its persistence — a test hook for deterministic interleaving.
	beforeSelectPersist func()
}

// setOnSessionChange wires a callback invoked on Lock and account switch,
// mirroring the NodeService.setReceiveFunc pattern to avoid a hard
// WalletService→TxService dep.
func (w *WalletService) setOnSessionChange(fn func()) { w.onSessionChange = fn }

func newWalletService(c *ConfigService) *WalletService {
	return &WalletService{config: c}
}

// walletPath validates a wallet file name and returns its absolute path inside
// the wallets dir. It rejects empty names, ".", "..", and any name containing a
// path separator or absolute path, preventing traversal outside walletsDir.
func (w *WalletService) walletPath(name string) (string, error) {
	if name == "" || name == "." || name == ".." || name != filepath.Base(name) {
		return "", fmt.Errorf("invalid wallet name %q", name)
	}
	dir, err := w.config.walletsDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, name), nil
}

// ListWallets reconciles the manifest with the keystore files on disk: it drops
// manifest entries whose keystore is gone, registers keystore files not yet in
// the manifest (the legacy-file migration; id=filename, name=filename stem,
// baseAddress from the keyfile), persists the manifest if it changed, and
// returns the entries in manifest order. No keystore content is modified; no
// file is renamed or moved.
func (w *WalletService) ListWallets() ([]WalletMeta, error) {
	dir, err := w.config.walletsDir()
	if err != nil {
		return nil, err
	}
	m, err := w.loadManifest()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	// Valid keystore filenames present on disk.
	files := map[string]string{} // filename -> baseAddress
	for _, e := range entries {
		if e.IsDir() || e.Name() == manifestFile {
			continue
		}
		kf, err := wallet.ReadKeyFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue // not a keystore
		}
		files[e.Name()] = kf.BaseAddress.String()
	}
	changed := false
	// Drop manifest entries whose keystore is gone.
	kept := m.Wallets[:0]
	known := map[string]bool{}
	for _, e := range m.Wallets {
		if _, ok := files[e.ID]; ok {
			kept = append(kept, e)
			known[e.ID] = true
		} else {
			changed = true
		}
	}
	m.Wallets = kept
	// Register keystore files not yet in the manifest (migration of legacy files).
	for name, addr := range files {
		if known[name] {
			continue
		}
		display := name[:len(name)-len(filepath.Ext(name))] // filename stem
		m.Wallets = append(m.Wallets, WalletMeta{ID: name, Name: display, BaseAddress: addr})
		changed = true
	}
	if changed {
		if err := w.saveManifest(m); err != nil {
			return nil, err
		}
	}
	return m.Wallets, nil
}

// ImportKeystore validates a keystore file and copies it into the wallets dir
// under a fresh uuid filename, recording a manifest entry with the display name
// (defaulting to the source filename stem when empty). The source filename is
// not reused, so same-named imports no longer collide. Duplicate base addresses
// are not blocked here — the frontend warns by comparing the returned
// baseAddress against its existing list. No keystore content is modified.
func (w *WalletService) ImportKeystore(srcPath, name string) (WalletMeta, error) {
	kf, err := wallet.ReadKeyFile(srcPath)
	if err != nil {
		return WalletMeta{}, fmt.Errorf("not a valid syrius keystore: %w", err)
	}
	if name == "" {
		base := filepath.Base(srcPath)
		name = base[:len(base)-len(filepath.Ext(base))]
	}
	id, err := newWalletID()
	if err != nil {
		return WalletMeta{}, err
	}
	dst, err := w.walletPath(id)
	if err != nil {
		return WalletMeta{}, err
	}
	if err := copyFile(srcPath, dst); err != nil {
		return WalletMeta{}, err
	}
	meta := WalletMeta{ID: id, Name: name, BaseAddress: kf.BaseAddress.String()}
	if err := w.addToManifest(meta); err != nil {
		return WalletMeta{}, err
	}
	return meta, nil
}

// addToManifest loads the manifest, appends meta, and persists it atomically.
func (w *WalletService) addToManifest(meta WalletMeta) error {
	m, err := w.loadManifest()
	if err != nil {
		return err
	}
	m.Wallets = append(m.Wallets, meta)
	return w.saveManifest(m)
}

// RenameWallet updates the display name of the manifest entry identified by id.
// It rejects an empty name and errors when the id is unknown. No keystore file
// is touched.
func (w *WalletService) RenameWallet(id, newName string) error {
	if strings.TrimSpace(newName) == "" {
		return errors.New("wallet name must not be empty")
	}
	m, err := w.loadManifest()
	if err != nil {
		return err
	}
	for i := range m.Wallets {
		if m.Wallets[i].ID == id {
			m.Wallets[i].Name = strings.TrimSpace(newName)
			return w.saveManifest(m)
		}
	}
	return fmt.Errorf("wallet %q not found", id)
}

// GenerateMnemonic returns a fresh 24-word (256-bit) BIP-39 mnemonic. It
// persists nothing — the create wizard shows it for backup before calling
// ImportMnemonic.
func (w *WalletService) GenerateMnemonic() (string, error) {
	entropy, err := bip39.NewEntropy(256)
	if err != nil {
		return "", err
	}
	return bip39.NewMnemonic(entropy)
}

// ImportMnemonic creates a keystore from a BIP-39 mnemonic (used for both
// "create new" after backup-verify and "import existing").
func (w *WalletService) ImportMnemonic(name, password, mnemonic string) (WalletMeta, error) {
	return w.writeKeystoreFromMnemonic(name, password, strings.TrimSpace(mnemonic))
}

// writeKeystoreFromMnemonic assembles a go-zenon KeyStore from the mnemonic and
// writes it as a syrius-compatible keyfile under a fresh uuid filename, recording
// a manifest entry with the display name. The uuid filename cannot collide, so
// there is no overwrite check.
func (w *WalletService) writeKeystoreFromMnemonic(name, password, mnemonic string) (WalletMeta, error) {
	if strings.TrimSpace(name) == "" {
		return WalletMeta{}, errors.New("wallet name must not be empty")
	}
	if password == "" {
		return WalletMeta{}, errors.New("password must not be empty")
	}
	if !bip39.IsMnemonicValid(mnemonic) {
		return WalletMeta{}, errors.New("invalid mnemonic")
	}
	entropy, err := bip39.EntropyFromMnemonic(mnemonic)
	if err != nil {
		return WalletMeta{}, fmt.Errorf("invalid mnemonic: %w", err)
	}
	ks := &wallet.KeyStore{
		Entropy:  entropy,
		Seed:     bip39.NewSeed(mnemonic, ""),
		Mnemonic: mnemonic,
	}
	defer ks.Zero()
	_, kp, err := ks.DeriveForIndexPath(0)
	if err != nil {
		return WalletMeta{}, err
	}
	ks.BaseAddress = kp.Address

	id, err := newWalletID()
	if err != nil {
		return WalletMeta{}, err
	}
	dst, err := w.walletPath(id)
	if err != nil {
		return WalletMeta{}, err
	}
	kf, err := ks.Encrypt(password)
	if err != nil {
		return WalletMeta{}, err
	}
	kf.Path = dst
	if err := kf.Write(); err != nil {
		return WalletMeta{}, err
	}
	meta := WalletMeta{ID: id, Name: name, BaseAddress: ks.BaseAddress.String()}
	if err := w.addToManifest(meta); err != nil {
		return WalletMeta{}, err
	}
	return meta, nil
}

// PickKeystoreFile opens a native file dialog and returns the chosen absolute
// path, or "" if the user cancels.
func (w *WalletService) PickKeystoreFile() (string, error) {
	return runtime.OpenFileDialog(w.ctx, runtime.OpenDialogOptions{
		Title:   "Import keystore",
		Filters: []runtime.FileFilter{{DisplayName: "Keystore (*.json, *.dat)", Pattern: "*.json;*.dat;*"}},
	})
}

// Unlock decrypts the named keystore and holds it in memory.
func (w *WalletService) Unlock(name, password string) error {
	path, err := w.walletPath(name)
	if err != nil {
		return err
	}
	kf, err := wallet.ReadKeyFile(path)
	if err != nil {
		return fmt.Errorf("cannot read keystore: %w", err)
	}
	ks, err := kf.Decrypt(password)
	if err != nil {
		return errors.New("incorrect password")
	}
	// The session swap serializes with account selection (selMu): an in-flight
	// selection completes fully before the wallet changes underneath it. The
	// slow KDF above deliberately runs outside the lock.
	w.selMu.Lock()
	defer w.selMu.Unlock()
	w.mu.Lock()
	if w.keystore != nil {
		// Unlocking over an already-unlocked wallet: zero the prior decrypted
		// seed instead of leaving it for the GC (minimize decrypted-seed lifetime).
		w.keystore.Zero()
	}
	w.keystore = ks
	w.active = 0
	w.activeWallet = name
	w.gen++
	w.mu.Unlock()
	w.startAutoLock()
	return nil
}

// ChangePassword re-encrypts the named keystore under a new password, writing
// atomically (temp file + rename) so a failure never corrupts the original.
func (w *WalletService) ChangePassword(name, oldPassword, newPassword string) error {
	if newPassword == "" {
		return errors.New("new password must not be empty")
	}
	path, err := w.walletPath(name)
	if err != nil {
		return err
	}
	kf, err := wallet.ReadKeyFile(path)
	if err != nil {
		return fmt.Errorf("cannot read keystore: %w", err)
	}
	ks, err := kf.Decrypt(oldPassword)
	if err != nil {
		return errors.New("incorrect password")
	}
	defer ks.Zero()
	newKf, err := ks.Encrypt(newPassword)
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	newKf.Path = tmp
	if err := newKf.Write(); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

// Lock zeroes and drops the decrypted keystore.
func (w *WalletService) Lock() error {
	// Serialize with account selection (see selMu): a selection paused before
	// persistence must complete before the session it validated against ends.
	w.selMu.Lock()
	defer w.selMu.Unlock()
	w.mu.Lock()
	if w.keystore != nil {
		w.keystore.Zero()
		w.keystore = nil
	}
	w.activeWallet = ""
	w.gen++
	onChange := w.onSessionChange
	w.mu.Unlock()
	w.stopAutoLock()

	if onChange != nil {
		onChange()
	}
	if w.ctx != nil {
		runtime.EventsEmit(w.ctx, EventWalletLocked)
	}
	return nil
}

// NoteActivity records user activity for the auto-lock watchdog. The frontend
// calls it (throttled) on genuine user input; every call is cheap and it never
// errors. No-op while locked.
func (w *WalletService) NoteActivity() {
	w.mu.Lock()
	locked := w.keystore == nil
	w.mu.Unlock()
	if locked {
		return
	}
	w.autoMu.Lock()
	w.lastActivity = time.Now()
	w.autoMu.Unlock()
}

// validAutoLockMinutes reports whether m is one of the Settings presets.
func validAutoLockMinutes(m int) bool {
	switch m {
	case 0, 1, 5, 15, 30:
		return true
	}
	return false
}

// SetAutoLockMinutes updates the live watchdog timeout and persists it.
// The live value updates even if persistence fails (in-memory wins for the
// session); 0 disables auto-lock ("Never").
func (w *WalletService) SetAutoLockMinutes(m int) error {
	if !validAutoLockMinutes(m) {
		return fmt.Errorf("auto-lock minutes must be one of 0, 1, 5, 15, 30; got %d", m)
	}
	w.autoMu.Lock()
	w.autoLockMins = m
	w.autoMu.Unlock()
	return w.config.updateSettings(func(s *Settings) error {
		s.AutoLockMinutes = intPtr(m)
		return nil
	})
}

// autoLockExpired reports whether inactivity exceeded the configured timeout.
func (w *WalletService) autoLockExpired(now time.Time) bool {
	w.autoMu.Lock()
	defer w.autoMu.Unlock()
	if w.autoLockMins <= 0 {
		return false
	}
	return now.Sub(w.lastActivity) > time.Duration(w.autoLockMins)*time.Minute
}

// startAutoLock (re)starts the inactivity watchdog. Called on Unlock. The
// keystore's owner enforces the deadline (fails CLOSED: a wedged WebView stops
// pinging and the wallet still locks). Worst-case lock is timeout + one tick.
func (w *WalletService) startAutoLock() {
	minutes := defaultAutoLockMinutes
	if s, err := w.config.GetSettings(); err == nil && s.AutoLockMinutes != nil {
		minutes = *s.AutoLockMinutes
	}
	w.autoMu.Lock()
	if w.autoLockStop != nil {
		close(w.autoLockStop)
	}
	stop := make(chan struct{})
	w.autoLockStop = stop
	w.autoLockMins = minutes
	w.lastActivity = time.Now()
	tick := w.autoLockTick
	if tick == 0 {
		tick = 15 * time.Second
	}
	w.autoMu.Unlock()

	go func() {
		t := time.NewTicker(tick)
		defer t.Stop()
		for {
			select {
			case <-stop:
				return
			case <-t.C:
				if w.autoLockExpired(time.Now()) {
					_ = w.Lock() // Lock() also stops this watchdog
					return
				}
			}
		}
	}()
}

// stopAutoLock halts the watchdog (called by Lock, manual or automatic).
func (w *WalletService) stopAutoLock() {
	w.autoMu.Lock()
	if w.autoLockStop != nil {
		close(w.autoLockStop)
		w.autoLockStop = nil
	}
	w.autoMu.Unlock()
}

// sessionGen returns the current session generation under the lock. It changes
// on every Unlock and Lock, letting callers detect a wallet state change.
func (w *WalletService) sessionGen() uint64 {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.gen
}

// labelKey is the settings map key for a given wallet's account index.
func labelKey(wallet string, index int) string { return fmt.Sprintf("%s:%d", wallet, index) }

// CurrentAccounts derives indices 0..accountRange-1 from the unlocked keystore.
func (w *WalletService) CurrentAccounts() ([]AccountInfo, error) {
	w.mu.Lock()
	if w.keystore == nil {
		w.mu.Unlock()
		return nil, errLocked
	}
	name := w.activeWallet
	w.mu.Unlock()

	// Read the per-wallet count + labels off the lock (disk I/O); fall back to
	// the default range/empty labels if settings are unavailable.
	count := accountRange
	var labels map[string]string
	if s, err := w.config.GetSettings(); err == nil {
		count = accountCountFor(s, name)
		labels = s.AccountLabels
	}

	w.mu.Lock()
	defer w.mu.Unlock()
	if w.keystore == nil {
		return nil, errLocked
	}
	out := make([]AccountInfo, 0, count)
	for i := 0; i < count; i++ {
		_, kp, err := w.keystore.DeriveForIndexPath(uint32(i))
		if err != nil {
			return nil, err
		}
		out = append(out, AccountInfo{Index: i, Address: kp.Address.String(), Label: labels[labelKey(name, i)]})
	}
	return out, nil
}

// SelectAccount sets the active account index, persists it, and returns the
// authoritative selection (index + address) so the frontend renders what the
// backend signer will actually use instead of assuming the requested index
// won. The whole operation is serialized under selMu: two overlapping calls
// apply strictly one after the other, so the backend active index and the
// persisted index always describe the same (last-completed) selection.
func (w *WalletService) SelectAccount(index int) (AccountInfo, error) {
	if index < 0 {
		return AccountInfo{}, fmt.Errorf("account index %d out of range", index)
	}
	w.selMu.Lock()
	defer w.selMu.Unlock()

	w.mu.Lock()
	if w.keystore == nil {
		w.mu.Unlock()
		return AccountInfo{}, errLocked
	}
	name := w.activeWallet
	w.mu.Unlock()

	// Settings are PART of the operation, not best-effort: the bound comes from
	// the revealed account count and the selection must persist, so a settings
	// failure fails the call with the signer unchanged (never a "successful"
	// selection that silently didn't stick).
	s, err := w.config.GetSettings()
	if err != nil {
		return AccountInfo{}, fmt.Errorf("cannot read settings: %w", err)
	}
	// Bound by the wallet's REVEALED account count, not maxAccounts.
	// CurrentAccounts only exposes accountCountFor(...) accounts; without this a
	// direct Wails call could activate — and then sign from — an index the UI
	// never revealed.
	if count := accountCountFor(s, name); index >= count {
		return AccountInfo{}, fmt.Errorf("account index %d out of range (only %d revealed)", index, count)
	}
	label := s.AccountLabels[labelKey(name, index)]

	if w.beforeSelectPersist != nil {
		w.beforeSelectPersist()
	}
	// Persist BEFORE the in-memory transition: either both commit, or the call
	// fails here and the signer never changes. (The reverse order could report
	// success while leaving ActiveAccount stale on disk.)
	if err := w.config.updateSettings(func(s *Settings) error {
		s.ActiveAccount = index
		return nil
	}); err != nil {
		return AccountInfo{}, fmt.Errorf("cannot persist the account selection: %w", err)
	}

	w.mu.Lock()
	if w.keystore == nil {
		// Defensive only: Lock/Unlock serialize on selMu, so the session cannot
		// change mid-operation.
		w.mu.Unlock()
		return AccountInfo{}, errLocked
	}
	changed := w.active != index
	w.active = index
	if changed {
		// An account switch is a session change: a transaction prepared under the
		// previous account must never be finalized with this account's key (the SDK
		// stamps the block sender from the CURRENT keypair at publish time), so the
		// generation bump makes ConfirmPublish refuse any pending hold.
		w.gen++
	}
	onChange := w.onSessionChange
	addr, ok := w.activeAddressLocked()
	w.mu.Unlock()
	if !ok {
		return AccountInfo{}, fmt.Errorf("cannot derive the address for account %d", index)
	}

	if changed && onChange != nil {
		onChange()
	}
	return AccountInfo{Index: index, Address: addr.String(), Label: label}, nil
}

// SetAccountLabel persists a human label for the active wallet's account index.
func (w *WalletService) SetAccountLabel(index int, label string) error {
	if index < 0 || index >= maxAccounts {
		return fmt.Errorf("account index %d out of range", index)
	}
	w.mu.Lock()
	name := w.activeWallet
	w.mu.Unlock()
	if name == "" {
		return errLocked
	}
	return w.config.updateSettings(func(s *Settings) error {
		if s.AccountLabels == nil {
			s.AccountLabels = map[string]string{}
		}
		s.AccountLabels[labelKey(name, index)] = label
		return nil
	})
}

// AddAccount reveals one more account (derivation index) for the active wallet
// by bumping its stored count, then returns the refreshed account list. It only
// derives addresses — no signing or keystore mutation.
func (w *WalletService) AddAccount() ([]AccountInfo, error) {
	w.mu.Lock()
	name := w.activeWallet
	locked := w.keystore == nil
	w.mu.Unlock()
	if locked || name == "" {
		return nil, errLocked
	}
	if err := w.config.updateSettings(func(s *Settings) error {
		cur := accountCountFor(*s, name)
		if cur >= maxAccounts {
			return fmt.Errorf("maximum of %d accounts reached", maxAccounts)
		}
		if s.AccountCounts == nil {
			s.AccountCounts = map[string]int{}
		}
		s.AccountCounts[name] = cur + 1
		return nil
	}); err != nil {
		return nil, err
	}
	return w.CurrentAccounts()
}

// activeAddress returns the active account's address, false if locked.
func (w *WalletService) activeAddress() (types.Address, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.activeAddressLocked()
}

// activeAddressLocked is the lock-free core of activeAddress. Callers MUST hold
// w.mu.
func (w *WalletService) activeAddressLocked() (types.Address, bool) {
	if w.keystore == nil {
		return types.Address{}, false
	}
	_, kp, err := w.keystore.DeriveForIndexPath(uint32(w.active)) // #nosec G115 -- account index is bounded small
	if err != nil {
		return types.Address{}, false
	}
	return kp.Address, true
}

// zeroBytes overwrites b with zeros. stdruntime.KeepAlive keeps the slice live
// until the writes complete so the compiler cannot elide them as dead stores
// (mirrors the SDK's own secure-zero in znn-sdk-go/wallet).
func zeroBytes(b []byte) {
	for i := range b {
		b[i] = 0
	}
	stdruntime.KeepAlive(b)
}

// signingKeyPair derives the SDK keypair for the active account from the
// unlocked mnemonic and asserts it matches the go-zenon active address (the
// Phase-0 cross-check). The mnemonic and keypair stay backend-only.
func (w *WalletService) signingKeyPair() (*sdkwallet.KeyPair, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.keystore == nil {
		return nil, errLocked
	}
	sdkKs, err := sdkwallet.NewKeyStoreFromMnemonic(w.keystore.Mnemonic)
	if err != nil {
		return nil, err
	}
	// GetKeyPair copies the derived Ed25519 key out of the BIP32 child seed
	// (ed25519.NewKeyFromSeed allocates a fresh private key), so the returned
	// keypair is independent of this keystore. Zero the transient BIP39 seed and
	// entropy once derivation is done instead of leaving them for the GC. This is
	// defense-in-depth — the unlocked go-zenon keystore intentionally holds the
	// mnemonic for the whole session — but there is no reason to keep a second,
	// derived-seed copy alive after the keypair exists. (The Mnemonic field is an
	// immutable string aliasing the resident mnemonic and cannot/need not be
	// zeroed.)
	defer func() {
		zeroBytes(sdkKs.Seed)
		zeroBytes(sdkKs.Entropy)
	}()
	kp, err := sdkKs.GetKeyPair(w.active)
	if err != nil {
		return nil, err
	}
	addr, err := kp.GetAddress()
	if err != nil {
		return nil, err
	}
	want, ok := w.activeAddressLocked()
	if !ok {
		return nil, errLocked
	}
	if *addr != want {
		return nil, fmt.Errorf("SDK-derived address %s does not match active address %s", addr.String(), want.String())
	}
	return kp, nil
}

// RevealMnemonic returns the active wallet's mnemonic after re-verifying the
// password against the keystore file. Requires an unlocked wallet. The mnemonic
// is never logged.
func (w *WalletService) RevealMnemonic(password string) (string, error) {
	w.mu.Lock()
	locked := w.keystore == nil
	name := w.activeWallet
	gen := w.gen
	w.mu.Unlock()
	if locked {
		return "", errLocked
	}
	path, err := w.walletPath(name)
	if err != nil {
		return "", err
	}
	kf, err := wallet.ReadKeyFile(path)
	if err != nil {
		return "", err
	}
	ks, err := kf.Decrypt(password)
	if err != nil {
		return "", errors.New("incorrect password")
	}
	ks.Zero()
	// The KDF above takes noticeable time. Re-check the session under the lock:
	// if the wallet was locked or switched meanwhile, no secret may cross the
	// binding — and the mnemonic returned is read from the CURRENT keystore, so
	// it can never be a stale capture from a superseded session.
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.keystore == nil || w.gen != gen {
		return "", errors.New("wallet state changed; not revealing")
	}
	return w.keystore.Mnemonic, nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src) // #nosec G304 -- src is an app-internal keystore path, not user/network input
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600) // #nosec G304 -- dst is an app-internal keystore path
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		_ = os.Remove(dst)
		return err
	}
	if err := out.Sync(); err != nil {
		_ = out.Close()
		_ = os.Remove(dst)
		return err
	}
	if err := out.Close(); err != nil {
		_ = os.Remove(dst)
		return err
	}
	return nil
}
