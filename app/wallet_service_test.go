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
