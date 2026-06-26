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
		return walletManifest{}, err
	}
	if m.Wallets == nil {
		m.Wallets = []WalletMeta{}
	}
	return m, nil
}

func (w *WalletService) saveManifest(m walletManifest) error {
	p, err := w.manifestPath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, p) // atomic
}
