package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeFixtureKeystore writes a real, valid keystore into the wallets dir under
// the given filename via the same Encrypt/Write path the create flow uses, and
// returns its baseAddress. This gives ListWallets a genuine keystore to read.
func writeFixtureKeystore(t *testing.T, w *WalletService, filename string) string {
	t.Helper()
	m, err := w.GenerateMnemonic()
	if err != nil {
		t.Fatalf("GenerateMnemonic: %v", err)
	}
	meta, err := w.ImportMnemonic(filename, "pw", m)
	if err != nil {
		t.Fatalf("ImportMnemonic(%q): %v", filename, err)
	}
	return meta.BaseAddress
}

func TestLoadManifestMissingIsEmpty(t *testing.T) {
	w := newTestWalletService(t)
	m, err := w.loadManifest()
	if err != nil {
		t.Fatalf("loadManifest: %v", err)
	}
	if m.Wallets == nil {
		t.Fatal("expected non-nil empty Wallets slice")
	}
	if len(m.Wallets) != 0 {
		t.Fatalf("expected empty manifest, got %d entries", len(m.Wallets))
	}
}

func TestSaveLoadManifestRoundTrip(t *testing.T) {
	w := newTestWalletService(t)
	want := walletManifest{Wallets: []WalletMeta{
		{ID: "abc.dat", Name: "Savings", BaseAddress: "z1qaddr1"},
		{ID: "def.dat", Name: "Spending", BaseAddress: "z1qaddr2"},
	}}
	if err := w.saveManifest(want); err != nil {
		t.Fatalf("saveManifest: %v", err)
	}
	got, err := w.loadManifest()
	if err != nil {
		t.Fatalf("loadManifest: %v", err)
	}
	if len(got.Wallets) != len(want.Wallets) {
		t.Fatalf("len = %d, want %d", len(got.Wallets), len(want.Wallets))
	}
	for i := range want.Wallets {
		if got.Wallets[i] != want.Wallets[i] {
			t.Fatalf("entry %d = %+v, want %+v", i, got.Wallets[i], want.Wallets[i])
		}
	}
}

func TestNewWalletIDDistinctDat(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 100; i++ {
		id, err := newWalletID()
		if err != nil {
			t.Fatalf("newWalletID: %v", err)
		}
		if !strings.HasSuffix(id, ".dat") {
			t.Fatalf("id %q does not end in .dat", id)
		}
		if seen[id] {
			t.Fatalf("duplicate id %q", id)
		}
		seen[id] = true
	}
}

// TestListWalletsRegistersLegacyKeystore covers the migration: a keystore file
// present on disk but absent from the manifest is registered with id=filename
// and name=filename stem, and the manifest is persisted.
func TestListWalletsRegistersLegacyKeystore(t *testing.T) {
	w := newTestWalletService(t)
	addr := writeFixtureKeystore(t, w, "legacy.dat")

	list, err := w.ListWallets()
	if err != nil {
		t.Fatalf("ListWallets: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 wallet, got %d: %+v", len(list), list)
	}
	got := list[0]
	if got.ID != "legacy.dat" {
		t.Fatalf("ID = %q, want legacy.dat", got.ID)
	}
	if got.Name != "legacy" {
		t.Fatalf("Name = %q, want legacy (filename stem)", got.Name)
	}
	if got.BaseAddress != addr {
		t.Fatalf("BaseAddress = %q, want %q", got.BaseAddress, addr)
	}

	// The manifest must have been persisted by the reconcile.
	dir, _ := w.config.walletsDir()
	if _, err := os.Stat(filepath.Join(dir, manifestFile)); err != nil {
		t.Fatalf("expected manifest persisted: %v", err)
	}

	// A second call must be stable (no duplicate registration).
	list2, err := w.ListWallets()
	if err != nil {
		t.Fatalf("ListWallets (2): %v", err)
	}
	if len(list2) != 1 || list2[0] != got {
		t.Fatalf("second ListWallets not stable: %+v", list2)
	}
}

// TestListWalletsDropsMissingKeystore covers reconcile: a manifest entry whose
// keystore file is gone is dropped on the next ListWallets.
func TestListWalletsDropsMissingKeystore(t *testing.T) {
	w := newTestWalletService(t)
	writeFixtureKeystore(t, w, "real.dat")

	// Seed the manifest with a real entry plus a phantom whose file never existed.
	if err := w.saveManifest(walletManifest{Wallets: []WalletMeta{
		{ID: "phantom.dat", Name: "Ghost", BaseAddress: "z1qghost"},
	}}); err != nil {
		t.Fatalf("saveManifest: %v", err)
	}

	list, err := w.ListWallets()
	if err != nil {
		t.Fatalf("ListWallets: %v", err)
	}
	// phantom dropped; real.dat registered.
	if len(list) != 1 {
		t.Fatalf("expected 1 wallet after reconcile, got %d: %+v", len(list), list)
	}
	if list[0].ID != "real.dat" {
		t.Fatalf("ID = %q, want real.dat", list[0].ID)
	}
	for _, e := range list {
		if e.ID == "phantom.dat" {
			t.Fatal("phantom entry should have been dropped")
		}
	}
}
