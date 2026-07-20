package app

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
)

const manifestFile = "wallets.json"

type walletManifest struct {
	Wallets []WalletMeta `json:"wallets"`
}

// newWalletID returns an opaque, collision-free keystore filename.
func newWalletID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b) + ".dat", nil
}

func (w *WalletService) manifestPath() (string, error) {
	dir, err := w.config.walletsDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, manifestFile), nil
}

func (w *WalletService) loadManifest() (walletManifest, error) {
	p, err := w.manifestPath()
	if err != nil {
		return walletManifest{}, err
	}
	data, err := os.ReadFile(p) // #nosec G304 -- constant filename within the app wallets dir
	if os.IsNotExist(err) {
		return walletManifest{Wallets: []WalletMeta{}}, nil
	}
	if err != nil {
		return walletManifest{}, err
	}
	var m walletManifest
	if err := json.Unmarshal(data, &m); err != nil {
		// A corrupt manifest must never hide on-disk wallets. Back it up and
		// return empty so ListWallets rebuilds the registry from the keystore
		// files (names fall back to the filename stem — recoverable).
		_ = os.Rename(p, p+".corrupt")
		return walletManifest{Wallets: []WalletMeta{}}, nil
	}
	if m.Wallets == nil {
		m.Wallets = []WalletMeta{}
	}
	return m, nil
}

// saveManifest persists the manifest as JSON atomically: a UNIQUE temp file in
// the same directory, fsync, then rename over wallets.json. The unique name (vs
// a fixed p+".tmp") means concurrent writers never fight over one temp path, and
// the fsync guarantees an interrupted write can never leave truncated JSON or a
// stray temp behind (audit GS-05). Callers serialize the surrounding
// read-modify-write with w.manifestMu; this function does not lock itself.
func (w *WalletService) saveManifest(m walletManifest) error {
	p, err := w.manifestPath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(p), "manifest-*.tmp")
	if err != nil {
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmp.Name())
		return err
	}
	if err := os.Rename(tmp.Name(), p); err != nil {
		_ = os.Remove(tmp.Name())
		return err
	}
	return nil
}
