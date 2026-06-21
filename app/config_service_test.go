package app

import (
	"os"
	"path/filepath"
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
	got.RemoteNodeURL = "ws://127.0.0.1:35998"
	got.ActiveAccount = 3
	if err := c.SetSettings(got); err != nil {
		t.Fatalf("SetSettings: %v", err)
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
	if s.LocalNodeURL != defaultLocalNodeURL {
		t.Fatalf("LocalNodeURL default, got %q", s.LocalNodeURL)
	}
	if s.NodeMode != "remote" {
		t.Fatalf("NodeMode default remote, got %q", s.NodeMode)
	}
	if s.NodeURL != "" {
		t.Fatalf("legacy NodeURL should be cleared, got %q", s.NodeURL)
	}
}

func TestSettingsDefaultsWhenNoFile(t *testing.T) {
	c := newTestConfig(t)
	s, err := c.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if s.NodeMode != "remote" || s.RemoteNodeURL != defaultNodeURL || s.LocalNodeURL != defaultLocalNodeURL {
		t.Fatalf("unexpected defaults: %+v", s)
	}
}

func TestActiveNodeURL(t *testing.T) {
	s := Settings{NodeMode: "remote", RemoteNodeURL: "wss://r", LocalNodeURL: "ws://l"}
	if s.ActiveNodeURL() != "wss://r" {
		t.Fatalf("remote active: %q", s.ActiveNodeURL())
	}
	s.NodeMode = "local"
	if s.ActiveNodeURL() != "ws://l" {
		t.Fatalf("local active: %q", s.ActiveNodeURL())
	}
}

func TestMigrationIdempotent(t *testing.T) {
	c := newTestConfig(t)
	s1, _ := c.GetSettings()
	if err := c.SetSettings(s1); err != nil {
		t.Fatal(err)
	}
	s2, _ := c.GetSettings()
	if s2.RemoteNodeURL != s1.RemoteNodeURL || s2.LocalNodeURL != s1.LocalNodeURL || s2.NodeMode != s1.NodeMode {
		t.Fatalf("not idempotent: %+v vs %+v", s1, s2)
	}
}
