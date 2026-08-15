package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func newTestConfig(t *testing.T) *ConfigService {
	t.Helper()
	t.Setenv("GO_SYRIUS_DATA_DIR", t.TempDir())
	return newConfigService()
}

func TestSettingsRoundTripAndDefaults(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GO_SYRIUS_DATA_DIR", dir)

	c := newConfigService()

	// Defaults on first read (no file yet).
	got, err := c.GetSettings()
	if err != nil {
		t.Fatalf("GetSettings: %v", err)
	}
	if got.RemoteNodeURL != defaultNodeURL || got.Theme != "dark" {
		t.Fatalf("defaults wrong: %+v", got)
	}

	// Round-trip.
	if err := c.updateSettings(func(s *Settings) error {
		s.RemoteNodeURL = "ws://127.0.0.1:35998"
		s.ActiveAccount = 3
		return nil
	}); err != nil {
		t.Fatalf("updateSettings: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "settings.json")); err != nil {
		t.Fatalf("settings.json not written: %v", err)
	}
	again, err := c.GetSettings()
	if err != nil {
		t.Fatalf("GetSettings 2: %v", err)
	}
	if again.RemoteNodeURL != "ws://127.0.0.1:35998" || again.ActiveAccount != 3 {
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

func TestSettingsMigrationFromLegacyNodeURL(t *testing.T) {
	c := newTestConfig(t)
	d, _ := c.dataDir()
	if err := os.WriteFile(filepath.Join(d, "settings.json"), []byte(`{"nodeUrl":"wss://custom:35998","theme":"dark"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	s, err := c.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if s.RemoteNodeURL != "wss://custom:35998" {
		t.Fatalf("legacy nodeUrl should migrate to RemoteNodeURL, got %q", s.RemoteNodeURL)
	}
	if s.NodeMode != "remote" {
		t.Fatalf("NodeMode default remote, got %q", s.NodeMode)
	}
	if s.NodeURL != "" {
		t.Fatalf("legacy NodeURL should be cleared, got %q", s.NodeURL)
	}
}

func TestSettingsNormalizesInvalidNodeMode(t *testing.T) {
	c := newTestConfig(t)
	d, _ := c.dataDir()
	if err := os.WriteFile(filepath.Join(d, "settings.json"), []byte(`{"nodeMode":"bogus","remoteNodeUrl":"wss://x"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	s, err := c.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if s.NodeMode != "remote" {
		t.Fatalf("invalid NodeMode should normalize to remote, got %q", s.NodeMode)
	}
}

func TestSettingsDefaultsWhenNoFile(t *testing.T) {
	c := newTestConfig(t)
	s, err := c.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if s.NodeMode != "remote" || s.RemoteNodeURL != defaultNodeURL {
		t.Fatalf("unexpected defaults: %+v", s)
	}
}

func TestActiveNodeURL(t *testing.T) {
	s := Settings{NodeMode: "remote", RemoteNodeURL: "wss://r"}
	if s.ActiveNodeURL() != "wss://r" {
		t.Fatalf("remote active: %q", s.ActiveNodeURL())
	}
	s.NodeMode = "embedded"
	if s.ActiveNodeURL() != defaultEmbeddedNodeURL {
		t.Fatalf("embedded active: %q", s.ActiveNodeURL())
	}
}

func TestMigrateSettingsLocalModeBecomesRemote(t *testing.T) {
	s := Settings{NodeMode: "local", RemoteNodeURL: "wss://example.org:35998"}
	migrateSettings(&s)
	if s.NodeMode != "remote" {
		t.Fatalf("NodeMode = %q, want remote", s.NodeMode)
	}
	if s.RemoteNodeURL != "wss://example.org:35998" {
		t.Fatalf("RemoteNodeURL changed: %q", s.RemoteNodeURL)
	}
}

func TestSettingsJSONDropsLegacyLocalKey(t *testing.T) {
	// Old settings.json with a localNodeUrl key must still parse (unknown keys
	// are ignored) and must not resurface on the next marshal.
	raw := []byte(`{"nodeMode":"local","remoteNodeUrl":"wss://r","localNodeUrl":"ws://127.0.0.1:35998"}`)
	var s Settings
	if err := json.Unmarshal(raw, &s); err != nil {
		t.Fatalf("unmarshal legacy settings: %v", err)
	}
	migrateSettings(&s)
	out, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(out), "localNodeUrl") {
		t.Fatalf("legacy localNodeUrl key persisted: %s", out)
	}
	if s.NodeMode != "remote" {
		t.Fatalf("NodeMode = %q, want remote", s.NodeMode)
	}
}

// Spec §3: saving drops the legacy key. The in-memory marshal test above cannot
// prove this — the real path is read a legacy settings.json → persist → the key
// is gone from the FILE and the removed "local" mode has become "remote".
func TestSavingDropsLegacyLocalKeyOnDisk(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GO_SYRIUS_DATA_DIR", dir)
	path := filepath.Join(dir, "settings.json")
	raw := []byte(`{"nodeMode":"local","remoteNodeUrl":"wss://r","localNodeUrl":"ws://x","theme":"dark"}`)
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatalf("seed settings.json: %v", err)
	}

	c := newConfigService()
	s, err := c.GetSettings()
	if err != nil {
		t.Fatalf("GetSettings: %v", err)
	}
	if s.NodeMode != "remote" {
		t.Fatalf("NodeMode = %q, want remote", s.NodeMode)
	}
	// A no-op mutator still rewrites the file through the migrated struct.
	if err := c.updateSettings(func(*Settings) error { return nil }); err != nil {
		t.Fatalf("updateSettings: %v", err)
	}

	out, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back settings.json: %v", err)
	}
	if strings.Contains(string(out), "localNodeUrl") {
		t.Fatalf("legacy localNodeUrl key survived a save: %s", out)
	}
	if !strings.Contains(string(out), `"nodeMode": "remote"`) {
		t.Fatalf("persisted mode should be remote: %s", out)
	}
	if s.RemoteNodeURL != "wss://r" {
		t.Fatalf("RemoteNodeURL changed: %q", s.RemoteNodeURL)
	}
}

func TestMigrationIdempotent(t *testing.T) {
	c := newTestConfig(t)
	s1, _ := c.GetSettings()
	if err := c.updateSettings(func(*Settings) error { return nil }); err != nil {
		t.Fatal(err)
	}
	s2, _ := c.GetSettings()
	if s2.RemoteNodeURL != s1.RemoteNodeURL || s2.NodeMode != s1.NodeMode {
		t.Fatalf("not idempotent: %+v vs %+v", s1, s2)
	}
}

func TestUpdateSettingsConcurrentNoLostUpdates(t *testing.T) {
	c := newTestConfig(t)
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_ = c.updateSettings(func(s *Settings) error {
				if s.AccountLabels == nil {
					s.AccountLabels = map[string]string{}
				}
				s.AccountLabels[fmt.Sprintf("w:%d", i)] = "x"
				return nil
			})
		}(i)
	}
	wg.Wait()
	s, err := c.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if len(s.AccountLabels) != 20 {
		t.Fatalf("concurrent updates lost fields: got %d of 20 labels", len(s.AccountLabels))
	}
}

func TestTargetedSettersPersist(t *testing.T) {
	c := newTestConfig(t)
	if err := c.SetChainID(3); err != nil {
		t.Fatal(err)
	}
	if err := c.SetAutoReceive(true); err != nil {
		t.Fatal(err)
	}
	if err := c.SetAllowMainnetSend(true); err != nil {
		t.Fatal(err)
	}
	if err := c.SetShowGovernance(true); err != nil {
		t.Fatal(err)
	}
	s, err := c.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if s.ChainID != 3 || !s.AutoReceive || !s.AllowMainnetSend || !s.ShowGovernance {
		t.Fatalf("targeted setters did not persist: %+v", s)
	}
	// The atomic write path must not leave temp files behind.
	d, _ := c.dataDir()
	entries, _ := os.ReadDir(d)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "settings-") && strings.HasSuffix(e.Name(), ".tmp") {
			t.Fatalf("leftover temp file %s", e.Name())
		}
	}
}

func TestUpdateSettingsErrorAbortsWrite(t *testing.T) {
	c := newTestConfig(t)
	if err := c.SetChainID(7); err != nil {
		t.Fatal(err)
	}
	wantErr := errors.New("nope")
	if err := c.updateSettings(func(s *Settings) error {
		s.ChainID = 999
		return wantErr
	}); err != wantErr {
		t.Fatalf("expected fn error to propagate, got %v", err)
	}
	s, _ := c.GetSettings()
	if s.ChainID != 7 {
		t.Fatalf("an erroring update must not persist, got ChainID %d", s.ChainID)
	}
}

// Auto-lock timeout persistence. The field is a *int so migration can tell
// "absent" (pre-feature settings.json → default 5) from an explicit 0 (Never).
func TestAutoLockMinutes_DefaultAndExplicitZero(t *testing.T) {
	t.Setenv("GO_SYRIUS_DATA_DIR", t.TempDir())
	c := newConfigService()

	// Fresh install: no settings.json → default 5.
	s, err := c.GetSettings()
	if err != nil {
		t.Fatalf("GetSettings: %v", err)
	}
	if s.AutoLockMinutes == nil || *s.AutoLockMinutes != 5 {
		t.Fatalf("fresh settings must default auto-lock to 5, got %v", s.AutoLockMinutes)
	}

	// Pre-feature settings.json (field absent) → migrated to 5 on read.
	if err := c.SetChainID(1); err != nil { // forces a settings.json write
		t.Fatalf("SetChainID: %v", err)
	}
	s, err = c.GetSettings()
	if err != nil || s.AutoLockMinutes == nil || *s.AutoLockMinutes != 5 {
		t.Fatalf("absent field must migrate to 5, got %v (err %v)", s.AutoLockMinutes, err)
	}

	// Explicit 0 (Never) must survive a persist → read round-trip.
	if err := c.updateSettings(func(s *Settings) error { s.AutoLockMinutes = intPtr(0); return nil }); err != nil {
		t.Fatalf("persist 0: %v", err)
	}
	s, err = c.GetSettings()
	if err != nil || s.AutoLockMinutes == nil || *s.AutoLockMinutes != 0 {
		t.Fatalf("explicit 0 must round-trip, got %v (err %v)", s.AutoLockMinutes, err)
	}
}

// A truly pre-feature settings.json (field absent) must migrate to the 5-minute
// default — this exercises migrateSettings' nil-fill on the real JSON path
// (defaultSettings-based writes always carry the field, so the earlier
// round-trip test cannot cover absence).
func TestAutoLockMinutes_AbsentFieldMigrates(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GO_SYRIUS_DATA_DIR", dir)
	raw := []byte(`{"nodeMode":"remote","remoteNodeUrl":"wss://x","localNodeUrl":"ws://127.0.0.1:35998","theme":"dark"}`)
	if err := os.WriteFile(filepath.Join(dir, "settings.json"), raw, 0o600); err != nil {
		t.Fatalf("seed settings.json: %v", err)
	}
	c := newConfigService()
	s, err := c.GetSettings()
	if err != nil {
		t.Fatalf("GetSettings: %v", err)
	}
	if s.AutoLockMinutes == nil || *s.AutoLockMinutes != 5 {
		t.Fatalf("absent autoLockMinutes must migrate to 5, got %v", s.AutoLockMinutes)
	}
}

func TestGetBuildInfo(t *testing.T) {
	c := newConfigService()
	info := c.GetBuildInfo()
	// Unstamped test build: version must be the honest "dev" fallback, and the
	// commit must be a non-empty displayable string (real hash or "unknown").
	if info.Version != "dev" {
		t.Fatalf("BuildInfo.Version = %q, want \"dev\" in tests", info.Version)
	}
	if info.Commit == "" {
		t.Fatal("BuildInfo.Commit is empty")
	}
}
