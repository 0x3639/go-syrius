package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	bip39 "github.com/tyler-smith/go-bip39"
	"github.com/zenon-network/go-zenon/wallet"
)

// unlockTestWallet installs a deterministic, offline keystore into w (no secrets
// file or RPC needed) and advances the session generation, mirroring Unlock.
// Useful for ConfirmPublish tests that only need activeAddress()+sessionGen().
func unlockTestWallet(t *testing.T, w *WalletService) {
	t.Helper()
	const mnemonic = "test test test test test test test test test test test junk"
	ks := &wallet.KeyStore{Mnemonic: mnemonic, Seed: bip39.NewSeed(mnemonic, "")}
	if _, kp, err := ks.DeriveForIndexPath(0); err == nil {
		ks.BaseAddress = kp.Address
	} else {
		t.Fatalf("derive base address: %v", err)
	}
	w.mu.Lock()
	w.keystore = ks
	w.active = 0
	w.gen++
	w.mu.Unlock()
}

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

func TestSigningKeyPairMatchesActiveAddress(t *testing.T) {
	ksPath, pw := locateSecretsKeystore(t)
	w := newTestWalletService(t)
	meta, err := w.ImportKeystore(ksPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Unlock(meta.Name, pw); err != nil {
		t.Fatal(err)
	}

	kp, err := w.signingKeyPair()
	if err != nil {
		t.Fatalf("signingKeyPair: %v", err)
	}
	addr, err := kp.GetAddress()
	if err != nil {
		t.Fatal(err)
	}
	want, _ := w.activeAddress()
	if *addr != want {
		t.Fatalf("sdk keypair address %s != active %s", addr, want)
	}

	_ = w.Lock()
	if _, err := w.signingKeyPair(); err == nil {
		t.Fatal("expected signingKeyPair to fail when locked")
	}
}

func TestGenerateMnemonic24Words(t *testing.T) {
	w := newTestWalletService(t)
	m, err := w.GenerateMnemonic()
	if err != nil {
		t.Fatalf("GenerateMnemonic: %v", err)
	}
	if n := len(strings.Fields(m)); n != 24 {
		t.Fatalf("expected 24 words, got %d", n)
	}
	m2, _ := w.GenerateMnemonic()
	if m == m2 {
		t.Fatal("expected distinct mnemonics")
	}
}

func TestImportMnemonicRoundTrip(t *testing.T) {
	w := newTestWalletService(t)
	m, _ := w.GenerateMnemonic()

	meta, err := w.ImportMnemonic("created.dat", "pw123", m)
	if err != nil {
		t.Fatalf("ImportMnemonic: %v", err)
	}
	if !strings.HasPrefix(meta.BaseAddress, "z1") {
		t.Fatalf("bad baseAddress %q", meta.BaseAddress)
	}

	// The written keystore must open via go-zenon and derive the same address.
	if err := w.Unlock("created.dat", "pw123"); err != nil {
		t.Fatalf("Unlock created wallet: %v", err)
	}
	accts, err := w.CurrentAccounts()
	if err != nil || accts[0].Address != meta.BaseAddress {
		t.Fatalf("round-trip address mismatch: %v / %v", accts, err)
	}

	// Refuse overwrite; reject invalid mnemonic.
	if _, err := w.ImportMnemonic("created.dat", "pw123", m); err == nil {
		t.Fatal("expected overwrite to be refused")
	}
	if _, err := w.ImportMnemonic("bad.dat", "pw", "not a valid mnemonic phrase"); err == nil {
		t.Fatal("expected invalid mnemonic to be rejected")
	}
}

func TestChangePassword(t *testing.T) {
	w := newTestWalletService(t)
	m, _ := w.GenerateMnemonic()
	if _, err := w.ImportMnemonic("cp.dat", "old-pw", m); err != nil {
		t.Fatal(err)
	}

	if err := w.ChangePassword("cp.dat", "wrong", "new-pw"); err == nil {
		t.Fatal("expected wrong old password to fail")
	}
	if err := w.ChangePassword("cp.dat", "old-pw", "new-pw"); err != nil {
		t.Fatalf("ChangePassword: %v", err)
	}

	if err := w.Unlock("cp.dat", "old-pw"); err == nil {
		t.Fatal("old password should no longer work")
	}
	if err := w.Unlock("cp.dat", "new-pw"); err != nil {
		t.Fatalf("new password should work: %v", err)
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

func TestRevealMnemonic(t *testing.T) {
	w := newTestWalletService(t)
	m, _ := w.GenerateMnemonic()
	if _, err := w.ImportMnemonic("rv.dat", "pw", m); err != nil {
		t.Fatal(err)
	}

	if _, err := w.RevealMnemonic("pw"); err == nil {
		t.Fatal("expected RevealMnemonic to fail when locked")
	}
	if err := w.Unlock("rv.dat", "pw"); err != nil {
		t.Fatal(err)
	}
	if _, err := w.RevealMnemonic("wrong"); err == nil {
		t.Fatal("expected wrong password to fail")
	}
	got, err := w.RevealMnemonic("pw")
	if err != nil {
		t.Fatalf("RevealMnemonic: %v", err)
	}
	if got != m {
		t.Fatalf("revealed mnemonic mismatch")
	}
}
