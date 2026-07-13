package app

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

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

	meta, err := w.ImportKeystore(ksPath, "")
	if err != nil {
		t.Fatalf("ImportKeystore: %v", err)
	}
	if !strings.HasPrefix(meta.BaseAddress, "z1") {
		t.Fatalf("bad baseAddress: %q", meta.BaseAddress)
	}
	if !strings.HasSuffix(meta.ID, ".dat") || meta.ID == filepath.Base(ksPath) {
		t.Fatalf("expected a uuid filename id, got %q", meta.ID)
	}

	list, err := w.ListWallets()
	if err != nil || len(list) != 1 {
		t.Fatalf("ListWallets = %v, %v", list, err)
	}

	if err := w.Unlock(meta.ID, "wrong-password"); err == nil {
		t.Fatal("expected unlock to fail with wrong password")
	}
	if err := w.Unlock(meta.ID, pw); err != nil {
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
	meta, err := w.ImportKeystore(ksPath, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Unlock(meta.ID, pw); err != nil {
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

	meta, err := w.ImportMnemonic("Created", "pw123", m)
	if err != nil {
		t.Fatalf("ImportMnemonic: %v", err)
	}
	if !strings.HasPrefix(meta.BaseAddress, "z1") {
		t.Fatalf("bad baseAddress %q", meta.BaseAddress)
	}
	if meta.Name != "Created" || !strings.HasSuffix(meta.ID, ".dat") || meta.ID == "Created" {
		t.Fatalf("expected uuid id + given name, got %+v", meta)
	}

	// The keystore file must be the uuid, and a manifest entry must exist.
	dir, _ := w.config.walletsDir()
	if _, err := os.Stat(filepath.Join(dir, meta.ID)); err != nil {
		t.Fatalf("uuid keystore not on disk: %v", err)
	}
	list, err := w.ListWallets()
	if err != nil || len(list) != 1 || list[0].ID != meta.ID || list[0].Name != "Created" {
		t.Fatalf("manifest entry not recorded: %v / %v", list, err)
	}

	// The written keystore must open via go-zenon (by id) and derive the same address.
	if err := w.Unlock(meta.ID, "pw123"); err != nil {
		t.Fatalf("Unlock created wallet: %v", err)
	}
	accts, err := w.CurrentAccounts()
	if err != nil || accts[0].Address != meta.BaseAddress {
		t.Fatalf("round-trip address mismatch: %v / %v", accts, err)
	}

	// Reject invalid mnemonic.
	if _, err := w.ImportMnemonic("bad", "pw", "not a valid mnemonic phrase"); err == nil {
		t.Fatal("expected invalid mnemonic to be rejected")
	}
}

// TestImportKeystoreSameNameCoexist asserts importing the same-named source
// twice yields two distinct uuid keystore files and two manifest entries.
func TestImportKeystoreSameNameCoexist(t *testing.T) {
	w := newTestWalletService(t)

	// Build a real source keystore via the write path, then read it back out as
	// a standalone source file we can import from repeatedly.
	m, _ := w.GenerateMnemonic()
	seed, err := w.ImportMnemonic("seed", "pw", m)
	if err != nil {
		t.Fatal(err)
	}
	dir, _ := w.config.walletsDir()
	srcDir := t.TempDir()
	srcPath := filepath.Join(srcDir, "wallet.dat") // same source name both times
	if err := copyFile(filepath.Join(dir, seed.ID), srcPath); err != nil {
		t.Fatal(err)
	}

	a, err := w.ImportKeystore(srcPath, "")
	if err != nil {
		t.Fatalf("first import: %v", err)
	}
	b, err := w.ImportKeystore(srcPath, "")
	if err != nil {
		t.Fatalf("second import (same-named source): %v", err)
	}
	if a.ID == b.ID {
		t.Fatalf("expected distinct uuid ids, got %q twice", a.ID)
	}
	if a.ID == "wallet.dat" || b.ID == "wallet.dat" {
		t.Fatalf("written filename must not be the source name: %q %q", a.ID, b.ID)
	}
	if a.Name != "wallet" || b.Name != "wallet" {
		t.Fatalf("name should default to source stem: %q %q", a.Name, b.Name)
	}
	for _, id := range []string{a.ID, b.ID} {
		if _, err := os.Stat(filepath.Join(dir, id)); err != nil {
			t.Fatalf("uuid keystore %q missing: %v", id, err)
		}
	}
	list, err := w.ListWallets()
	if err != nil {
		t.Fatal(err)
	}
	// seed + 2 imports = 3 entries.
	if len(list) != 3 {
		t.Fatalf("expected 3 manifest entries, got %d: %+v", len(list), list)
	}
}

func TestRenameWallet(t *testing.T) {
	w := newTestWalletService(t)
	m, _ := w.GenerateMnemonic()
	meta, err := w.ImportMnemonic("Original", "pw", m)
	if err != nil {
		t.Fatal(err)
	}

	if err := w.RenameWallet(meta.ID, "Renamed"); err != nil {
		t.Fatalf("RenameWallet: %v", err)
	}
	list, _ := w.ListWallets()
	if len(list) != 1 || list[0].Name != "Renamed" || list[0].ID != meta.ID {
		t.Fatalf("rename not applied: %+v", list)
	}

	if err := w.RenameWallet(meta.ID, "   "); err == nil {
		t.Fatal("expected empty/whitespace name to be rejected")
	}
	if err := w.RenameWallet("no-such-id.dat", "X"); err == nil {
		t.Fatal("expected unknown id to error")
	}
}

func TestChangePassword(t *testing.T) {
	w := newTestWalletService(t)
	m, _ := w.GenerateMnemonic()
	meta, err := w.ImportMnemonic("cp", "old-pw", m)
	if err != nil {
		t.Fatal(err)
	}

	if err := w.ChangePassword(meta.ID, "wrong", "new-pw"); err == nil {
		t.Fatal("expected wrong old password to fail")
	}
	if err := w.ChangePassword(meta.ID, "old-pw", "new-pw"); err != nil {
		t.Fatalf("ChangePassword: %v", err)
	}

	if err := w.Unlock(meta.ID, "old-pw"); err == nil {
		t.Fatal("old password should no longer work")
	}
	if err := w.Unlock(meta.ID, "new-pw"); err != nil {
		t.Fatalf("new password should work: %v", err)
	}
}

func TestImportRejectsNonKeystore(t *testing.T) {
	w := newTestWalletService(t)
	bad := filepath.Join(t.TempDir(), "notakeystore.json")
	if err := os.WriteFile(bad, []byte(`{"hello":"world"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := w.ImportKeystore(bad, ""); err == nil {
		t.Fatal("expected ImportKeystore to reject a non-keystore file")
	}
}

func TestRevealMnemonic(t *testing.T) {
	w := newTestWalletService(t)
	m, _ := w.GenerateMnemonic()
	meta, err := w.ImportMnemonic("rv", "pw", m)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := w.RevealMnemonic("pw"); err == nil {
		t.Fatal("expected RevealMnemonic to fail when locked")
	}
	if err := w.Unlock(meta.ID, "pw"); err != nil {
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

func TestRejectsTraversalNames(t *testing.T) {
	w := newTestWalletService(t)

	// Display names are no longer used as filenames (uuid storage), so a "../evil"
	// name is harmless on the write path. The id-keyed lookups still validate
	// against traversal via walletPath.
	if err := w.Unlock("../evil", "pw"); err == nil || !strings.Contains(err.Error(), "invalid wallet name") {
		t.Fatalf("Unlock traversal: expected invalid name error, got %v", err)
	}
	if err := w.ChangePassword("../evil", "a", "b"); err == nil || !strings.Contains(err.Error(), "invalid wallet name") {
		t.Fatalf("ChangePassword traversal: expected invalid name error, got %v", err)
	}
}

func TestRejectsEmptyPassword(t *testing.T) {
	w := newTestWalletService(t)
	m, _ := w.GenerateMnemonic()

	if _, err := w.ImportMnemonic("ok", "", m); err == nil {
		t.Fatal("expected ImportMnemonic with empty password to fail")
	}
	meta, err := w.ImportMnemonic("ok", "pw", m)
	if err != nil {
		t.Fatalf("ImportMnemonic: %v", err)
	}
	if err := w.ChangePassword(meta.ID, "pw", ""); err == nil {
		t.Fatal("expected ChangePassword to empty new password to fail")
	}
}

func TestAccountLabels(t *testing.T) {
	w := newTestWalletService(t)
	m, _ := w.GenerateMnemonic()
	meta, err := w.ImportMnemonic("lbl", "pw", m)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Unlock(meta.ID, "pw"); err != nil {
		t.Fatal(err)
	}

	if err := w.SetAccountLabel(0, "Savings"); err != nil {
		t.Fatalf("SetAccountLabel: %v", err)
	}
	accts, err := w.CurrentAccounts()
	if err != nil {
		t.Fatal(err)
	}
	if accts[0].Label != "Savings" {
		t.Fatalf("label not applied: %+v", accts[0])
	}
	if err := w.SetAccountLabel(maxAccounts, "x"); err == nil {
		t.Fatal("expected out-of-range index to fail")
	}
}

func TestAddAccount(t *testing.T) {
	w := newTestWalletService(t)
	m, _ := w.GenerateMnemonic()
	meta, err := w.ImportMnemonic("add", "pw", m)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Unlock(meta.ID, "pw"); err != nil {
		t.Fatal(err)
	}
	base, err := w.CurrentAccounts()
	if err != nil {
		t.Fatal(err)
	}
	if len(base) != accountRange {
		t.Fatalf("default account count = %d, want %d", len(base), accountRange)
	}
	got, err := w.AddAccount()
	if err != nil {
		t.Fatalf("AddAccount: %v", err)
	}
	if len(got) != accountRange+1 {
		t.Fatalf("after AddAccount count = %d, want %d", len(got), accountRange+1)
	}
	// The newly revealed index has a non-empty address; earlier ones are unchanged.
	if got[accountRange].Index != accountRange || got[accountRange].Address == "" {
		t.Fatalf("new account malformed: %+v", got[accountRange])
	}
	if got[0].Address != base[0].Address {
		t.Fatal("existing account address changed after AddAccount")
	}
}

func TestSelectAccountRejectsUnrevealedIndex(t *testing.T) {
	w := newTestWalletService(t)
	m, _ := w.GenerateMnemonic()
	meta, err := w.ImportMnemonic("sel", "pw", m)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Unlock(meta.ID, "pw"); err != nil {
		t.Fatal(err)
	}
	// Default reveals accountRange accounts: indices 0..accountRange-1 selectable.
	if _, err := w.SelectAccount(accountRange - 1); err != nil {
		t.Fatalf("SelectAccount(%d) should succeed: %v", accountRange-1, err)
	}
	// An index at/above the revealed count must be rejected. Previously SelectAccount
	// only bounded by maxAccounts, so a direct Wails call could activate — and then
	// sign from — an account the UI never revealed.
	if _, err := w.SelectAccount(accountRange); err == nil {
		t.Fatalf("SelectAccount(%d) must be rejected (only %d revealed)", accountRange, accountRange)
	}
	if _, err := w.SelectAccount(maxAccounts - 1); err == nil {
		t.Fatal("SelectAccount(maxAccounts-1) must be rejected while unrevealed")
	}
	// Revealing one more makes exactly the next index selectable.
	if _, err := w.AddAccount(); err != nil {
		t.Fatalf("AddAccount: %v", err)
	}
	if _, err := w.SelectAccount(accountRange); err != nil {
		t.Fatalf("SelectAccount(%d) should succeed after AddAccount: %v", accountRange, err)
	}
}

func TestSelectAccountBumpsSessionGen(t *testing.T) {
	w := newTestWalletService(t)
	unlockTestWallet(t, w)
	gen := w.sessionGen()
	if _, err := w.SelectAccount(1); err != nil {
		t.Fatalf("SelectAccount: %v", err)
	}
	if w.sessionGen() == gen {
		t.Fatal("switching accounts must bump the session generation (invalidates pending tx)")
	}
	gen = w.sessionGen()
	if _, err := w.SelectAccount(1); err != nil {
		t.Fatalf("SelectAccount: %v", err)
	}
	if w.sessionGen() != gen {
		t.Fatal("re-selecting the already-active account must not bump the generation")
	}
}

func TestSelectAccountInvokesSessionChange(t *testing.T) {
	w := newTestWalletService(t)
	unlockTestWallet(t, w)
	calls := 0
	w.setOnSessionChange(func() { calls++ })
	if _, err := w.SelectAccount(2); err != nil {
		t.Fatalf("SelectAccount: %v", err)
	}
	if calls != 1 {
		t.Fatalf("account switch must fire the session-change callback once, got %d", calls)
	}
	if _, err := w.SelectAccount(2); err != nil {
		t.Fatalf("SelectAccount: %v", err)
	}
	if calls != 1 {
		t.Fatalf("same-index select must not fire the callback, got %d", calls)
	}
}

func TestUnlockZeroesPriorKeystore(t *testing.T) {
	w := newTestWalletService(t)
	m, err := w.GenerateMnemonic()
	if err != nil {
		t.Fatal(err)
	}
	meta, err := w.ImportMnemonic("zero", "pw", m)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Unlock(meta.ID, "pw"); err != nil {
		t.Fatal(err)
	}
	prior := w.keystore
	if err := w.Unlock(meta.ID, "pw"); err != nil {
		t.Fatal(err)
	}
	if prior.Mnemonic != "" || prior.Seed != nil || prior.Entropy != nil {
		t.Fatal("re-unlock must zero the previously decrypted keystore, not abandon it to the GC")
	}
}

// TestSelectAccountSerialized forces the adverse interleaving from the review:
// selection A pauses at the persistence boundary while selection B runs. The
// selection mutex must hold B back until A completes, so the backend active
// index and the persisted index always agree with the LAST completed call.
func TestSelectAccountSerialized(t *testing.T) {
	w := newTestWalletService(t)
	unlockTestWallet(t, w)

	entered := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	w.beforeSelectPersist = func() {
		once.Do(func() {
			close(entered)
			<-release
		})
	}

	firstDone := make(chan error, 1)
	go func() {
		_, err := w.SelectAccount(1)
		firstDone <- err
	}()
	<-entered // selection 1 is paused between mutation and persistence

	secondDone := make(chan error, 1)
	go func() {
		_, err := w.SelectAccount(2)
		secondDone <- err
	}()
	select {
	case <-secondDone:
		t.Fatal("a second selection completed while the first was mid-operation")
	case <-time.After(100 * time.Millisecond):
	}

	close(release)
	if err := <-firstDone; err != nil {
		t.Fatalf("first SelectAccount: %v", err)
	}
	if err := <-secondDone; err != nil {
		t.Fatalf("second SelectAccount: %v", err)
	}

	w.mu.Lock()
	active := w.active
	w.mu.Unlock()
	if active != 2 {
		t.Fatalf("backend active = %d, want 2 (the last completed selection)", active)
	}
	s, err := w.config.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if s.ActiveAccount != 2 {
		t.Fatalf("persisted ActiveAccount = %d, want 2 — a stale selection persisted last", s.ActiveAccount)
	}
}

// TestSelectAccountReturnsAuthoritativeSelection: the caller gets the index and
// address the backend signer will actually use.
func TestSelectAccountReturnsAuthoritativeSelection(t *testing.T) {
	w := newTestWalletService(t)
	unlockTestWallet(t, w)
	info, err := w.SelectAccount(3)
	if err != nil {
		t.Fatalf("SelectAccount: %v", err)
	}
	if info.Index != 3 {
		t.Fatalf("returned index %d, want 3", info.Index)
	}
	addr, ok := w.activeAddress()
	if !ok || info.Address != addr.String() {
		t.Fatalf("returned address %q does not match the active signer address %q", info.Address, addr.String())
	}
}

// TestLockWaitsForInFlightSelection: the wallet-lifecycle session swap must
// serialize with a selection that is mid-operation — otherwise a selection
// paused before persistence can resolve against a different session than it
// validated (round-3 review P1).
func TestLockWaitsForInFlightSelection(t *testing.T) {
	w := newTestWalletService(t)
	unlockTestWallet(t, w)

	entered := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	w.beforeSelectPersist = func() {
		once.Do(func() {
			close(entered)
			<-release
		})
	}

	selDone := make(chan error, 1)
	go func() {
		_, err := w.SelectAccount(1)
		selDone <- err
	}()
	<-entered // the selection is paused mid-operation

	lockDone := make(chan error, 1)
	go func() { lockDone <- w.Lock() }()
	select {
	case <-lockDone:
		t.Fatal("Lock completed while a selection was mid-operation; the session swap must wait")
	case <-time.After(100 * time.Millisecond):
	}

	close(release)
	if err := <-selDone; err != nil {
		t.Fatalf("SelectAccount: %v", err)
	}
	if err := <-lockDone; err != nil {
		t.Fatalf("Lock: %v", err)
	}
	// The lock ran strictly AFTER the selection completed.
	w.mu.Lock()
	locked := w.keystore == nil
	w.mu.Unlock()
	if !locked {
		t.Fatal("wallet must be locked after Lock")
	}
}

// TestUnlockSerializesWithSelection: an unlock issued while a selection is
// paused must apply strictly after it, leaving the NEW session's defaults
// (account 0) as the final backend state.
func TestUnlockSerializesWithSelection(t *testing.T) {
	w := newTestWalletService(t)
	m, err := w.GenerateMnemonic()
	if err != nil {
		t.Fatal(err)
	}
	meta, err := w.ImportMnemonic("lifecycle", "pw", m)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Unlock(meta.ID, "pw"); err != nil {
		t.Fatal(err)
	}

	entered := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	w.beforeSelectPersist = func() {
		once.Do(func() {
			close(entered)
			<-release
		})
	}

	selDone := make(chan error, 1)
	go func() {
		_, err := w.SelectAccount(2)
		selDone <- err
	}()
	<-entered

	unlockDone := make(chan error, 1)
	go func() { unlockDone <- w.Unlock(meta.ID, "pw") }()
	// Give the unlock time to finish its KDF and reach the session swap, then
	// release the paused selection. Serialization means the swap applied AFTER
	// the selection, so the final active index is the new session's 0.
	time.Sleep(50 * time.Millisecond)
	close(release)
	if err := <-selDone; err != nil {
		t.Fatalf("SelectAccount: %v", err)
	}
	if err := <-unlockDone; err != nil {
		t.Fatalf("Unlock: %v", err)
	}
	w.mu.Lock()
	active := w.active
	w.mu.Unlock()
	if active != 0 {
		t.Fatalf("after a serialized re-unlock the active index must be the new session's 0, got %d", active)
	}
}

// TestSelectAccountFailsWhenPersistenceFails: persistence is part of the
// operation — a selection that cannot persist must fail WITHOUT changing the
// signer (round-3 review P2).
func TestSelectAccountFailsWhenPersistenceFails(t *testing.T) {
	w := newTestWalletService(t)
	unlockTestWallet(t, w)
	dir := os.Getenv("GO_SYRIUS_DATA_DIR")
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })

	genBefore := w.sessionGen()
	if _, err := w.SelectAccount(1); err == nil {
		t.Fatal("a selection whose persistence fails must fail the call")
	}
	w.mu.Lock()
	active := w.active
	w.mu.Unlock()
	if active != 0 {
		t.Fatalf("a failed selection must not change the signer, got active %d", active)
	}
	if w.sessionGen() != genBefore {
		t.Fatal("a failed selection must not invalidate the session")
	}
}
