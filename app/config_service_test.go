package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSettingsRoundTripAndDefaults(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GO_SYRIUS_DATA_DIR", dir)

	c := newConfigService()

	// Defaults on first read (no file yet).
	got, err := c.GetSettings()
	if err != nil {
		t.Fatalf("GetSettings: %v", err)
	}
	if got.NodeURL != defaultNodeURL || got.Theme != "dark" {
		t.Fatalf("defaults wrong: %+v", got)
	}

	// Round-trip.
	got.NodeURL = "ws://127.0.0.1:35998"
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
	if again.NodeURL != "ws://127.0.0.1:35998" || again.ActiveAccount != 3 {
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
