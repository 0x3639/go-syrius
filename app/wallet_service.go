package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"sync"

	sdkwallet "github.com/0x3639/znn-sdk-go/wallet"
	bip39 "github.com/tyler-smith/go-bip39"
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

	// mu protects keystore/active/gen. Go mutexes are NOT reentrant, so internal
	// callers must use the *Locked() helpers while public methods take the lock
	// once. No method calls another lock-taking method while holding mu.
	mu       sync.Mutex
	keystore *wallet.KeyStore // nil when locked
	active   int
	gen      uint64 // session generation, bumped on each Unlock and Lock

	onLock func() // optional callback invoked on Lock (e.g. clear pending tx)
}

// setOnLock wires a callback invoked when the wallet is locked, mirroring the
// NodeService.setReceiveFunc pattern to avoid a hard WalletService→TxService dep.
func (w *WalletService) setOnLock(fn func()) { w.onLock = fn }

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
// writes it as a syrius-compatible keyfile, refusing to overwrite an existing file.
func (w *WalletService) writeKeystoreFromMnemonic(name, password, mnemonic string) (WalletMeta, error) {
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

	dir, err := w.config.walletsDir()
	if err != nil {
		return WalletMeta{}, err
	}
	dst := filepath.Join(dir, name)
	if _, err := os.Stat(dst); err == nil {
		return WalletMeta{}, fmt.Errorf("a wallet named %q already exists", name)
	}
	kf, err := ks.Encrypt(password)
	if err != nil {
		return WalletMeta{}, err
	}
	kf.Path = dst
	if err := kf.Write(); err != nil {
		return WalletMeta{}, err
	}
	return WalletMeta{Name: name, BaseAddress: ks.BaseAddress.String()}, nil
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
	w.mu.Lock()
	w.keystore = ks
	w.active = 0
	w.gen++
	w.mu.Unlock()
	return nil
}

// Lock zeroes and drops the decrypted keystore.
func (w *WalletService) Lock() error {
	w.mu.Lock()
	if w.keystore != nil {
		w.keystore.Zero()
		w.keystore = nil
	}
	w.gen++
	onLock := w.onLock
	w.mu.Unlock()

	if onLock != nil {
		onLock()
	}
	if w.ctx != nil {
		runtime.EventsEmit(w.ctx, EventWalletLocked)
	}
	return nil
}

// sessionGen returns the current session generation under the lock. It changes
// on every Unlock and Lock, letting callers detect a wallet state change.
func (w *WalletService) sessionGen() uint64 {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.gen
}

// CurrentAccounts derives indices 0..accountRange-1 from the unlocked keystore.
func (w *WalletService) CurrentAccounts() ([]AccountInfo, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
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
	if index < 0 || index >= accountRange {
		return fmt.Errorf("account index %d out of range", index)
	}
	w.mu.Lock()
	if w.keystore == nil {
		w.mu.Unlock()
		return errLocked
	}
	w.active = index
	w.mu.Unlock()

	s, err := w.config.GetSettings()
	if err == nil {
		s.ActiveAccount = index
		_ = w.config.SetSettings(s)
	}
	return nil
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
	_, kp, err := w.keystore.DeriveForIndexPath(uint32(w.active))
	if err != nil {
		return types.Address{}, false
	}
	return kp.Address, true
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
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		os.Remove(dst)
		return err
	}
	if err := out.Sync(); err != nil {
		out.Close()
		os.Remove(dst)
		return err
	}
	if err := out.Close(); err != nil {
		os.Remove(dst)
		return err
	}
	return nil
}
