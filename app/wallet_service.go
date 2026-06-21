package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	sdkwallet "github.com/0x3639/znn-sdk-go/wallet"
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
	if w.onLock != nil {
		w.onLock()
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

// signingKeyPair derives the SDK keypair for the active account from the
// unlocked mnemonic and asserts it matches the go-zenon active address (the
// Phase-0 cross-check). The mnemonic and keypair stay backend-only.
func (w *WalletService) signingKeyPair() (*sdkwallet.KeyPair, error) {
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
	want, ok := w.activeAddress()
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
