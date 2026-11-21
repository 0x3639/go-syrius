package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/0x3639/go-syrius/internal/version"
)

// ConfigService resolves the data directory and persists user settings.
type ConfigService struct {
	ctx context.Context

	// mu serializes every settings read-for-update and write. Settings are
	// mutated concurrently (UI toggles, account switches, auto-receive follows),
	// and unserialized get→modify→set cycles lose each other's fields.
	mu sync.Mutex
}

func newConfigService() *ConfigService { return &ConfigService{} }

// dataDir is the app data directory; GO_SYRIUS_DATA_DIR overrides it (tests).
func (c *ConfigService) dataDir() (string, error) {
	if d := os.Getenv("GO_SYRIUS_DATA_DIR"); d != "" {
		if err := os.MkdirAll(d, 0o700); err != nil { // #nosec G703 -- data dir from app-owned env override (GO_SYRIUS_DATA_DIR), not untrusted input
			return "", err
		}
		return d, nil
	}
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	d := filepath.Join(base, "go-syrius")
	if err := os.MkdirAll(d, 0o700); err != nil {
		return "", err
	}
	return d, nil
}

func (c *ConfigService) walletsDir() (string, error) {
	d, err := c.dataDir()
	if err != nil {
		return "", err
	}
	wd := filepath.Join(d, "wallets")
	if err := os.MkdirAll(wd, 0o700); err != nil {
		return "", err
	}
	return wd, nil
}

// defaultAutoLockMinutes is the inactivity auto-lock default (spec: 5 minutes).
const defaultAutoLockMinutes = 5

// intPtr returns a pointer to v (for optional-int settings fields).
func intPtr(v int) *int { return &v }

// UI layout modes. Everyday keeps operator tools out of the navigation;
// Advanced shows everything. A fresh install starts in Everyday; an install
// that predates the setting keeps the full (Advanced) layout it already had.
const (
	uiModeEveryday = "everyday"
	uiModeAdvanced = "advanced"
)

// maxPinnedActions bounds the Advanced overview's shortcut row.
const maxPinnedActions = 6

// pinnableActions is the allowlist of shortcut ids the frontend may pin.
var pinnableActions = map[string]bool{
	"fuse": true, "collect": true, "receive": true, "vote": true, "send": true,
	"stake": true, "tokens": true, "walletconnect": true, "node": true,
}

func defaultPinnedActions() []string { return []string{"fuse", "collect", "receive"} }

func defaultSettings() Settings {
	return Settings{
		NodeMode:        "remote",
		RemoteNodeURL:   defaultNodeURL,
		Theme:           "dark",
		ActiveAccount:   0,
		AutoLockMinutes: intPtr(defaultAutoLockMinutes),
		UIMode:          uiModeEveryday,
		PinnedActions:   defaultPinnedActions(),
	}
}

// sanitizePinnedActions keeps known ids, in order, without duplicates, capped
// at maxPinnedActions. It always returns a non-nil slice.
func sanitizePinnedActions(ids []string) []string {
	out := make([]string, 0, len(ids))
	seen := map[string]bool{}
	for _, id := range ids {
		if !pinnableActions[id] || seen[id] || len(out) >= maxPinnedActions {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}

// migrateSettings fills new node fields and migrates the deprecated single
// nodeUrl. Idempotent and safe on default settings.
func migrateSettings(s *Settings) {
	if s.RemoteNodeURL == "" {
		if s.NodeURL != "" {
			s.RemoteNodeURL = s.NodeURL
		} else {
			s.RemoteNodeURL = defaultNodeURL
		}
	}
	if s.NodeMode != "remote" && s.NodeMode != "embedded" {
		s.NodeMode = "remote" // includes migrating the removed "local" mode
	}
	if s.Theme == "" {
		s.Theme = "dark"
	}
	if s.AutoLockMinutes == nil {
		s.AutoLockMinutes = intPtr(defaultAutoLockMinutes)
	}
	// A settings file without a mode predates the two layouts: keep the full
	// navigation that install already had. (A fresh install never gets here
	// with an empty mode — defaultSettings sets Everyday.)
	if s.UIMode != uiModeEveryday && s.UIMode != uiModeAdvanced {
		s.UIMode = uiModeAdvanced
	}
	if s.PinnedActions == nil {
		s.PinnedActions = defaultPinnedActions()
	} else {
		s.PinnedActions = sanitizePinnedActions(s.PinnedActions)
	}
	s.NodeURL = "" // stop persisting the deprecated field
}

// hasExistingWallets reports whether the wallets directory already holds
// anything. Used only to pick the layout default when no settings file exists:
// wallets without settings means an existing install, not a fresh one.
func (c *ConfigService) hasExistingWallets() bool {
	d, err := c.dataDir()
	if err != nil {
		return false
	}
	entries, err := os.ReadDir(filepath.Join(d, "wallets"))
	return err == nil && len(entries) > 0
}

// GetSettings returns persisted settings, or defaults if none exist.
func (c *ConfigService) GetSettings() (Settings, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.getSettingsLocked()
}

// getSettingsLocked is the lock-free core of GetSettings. Callers must hold c.mu.
func (c *ConfigService) getSettingsLocked() (Settings, error) {
	d, err := c.dataDir()
	if err != nil {
		return Settings{}, err
	}
	raw, err := os.ReadFile(filepath.Join(d, "settings.json")) // #nosec G304 -- constant filename within the app data dir
	if os.IsNotExist(err) {
		s := defaultSettings()
		if c.hasExistingWallets() {
			s.UIMode = uiModeAdvanced
		}
		return s, nil
	}
	if err != nil {
		return Settings{}, err
	}
	var s Settings
	if err := json.Unmarshal(raw, &s); err != nil {
		return Settings{}, err
	}
	migrateSettings(&s)
	return s, nil
}

// setSettingsLocked persists settings as JSON atomically: temp file in the same
// directory, fsync, then rename over settings.json — an interrupted write can
// never leave truncated JSON behind. Callers must hold c.mu. There is no
// exported whole-settings setter: the frontend gets targeted setters only, so
// it can never clobber fields it doesn't own.
func (c *ConfigService) setSettingsLocked(s Settings) error {
	d, err := c.dataDir()
	if err != nil {
		return err
	}
	raw, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(d, "settings-*.tmp")
	if err != nil {
		return err
	}
	if _, err := tmp.Write(raw); err != nil {
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
	if err := os.Rename(tmp.Name(), filepath.Join(d, "settings.json")); err != nil {
		_ = os.Remove(tmp.Name())
		return err
	}
	return nil
}

// updateSettings applies fn to the current settings and persists the result as
// ONE atomic read-modify-write under the mutex. An error from fn aborts without
// writing. All internal settings mutation goes through here.
func (c *ConfigService) updateSettings(fn func(*Settings) error) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	s, err := c.getSettingsLocked()
	if err != nil {
		return err
	}
	if err := fn(&s); err != nil {
		return err
	}
	return c.setSettingsLocked(s)
}

// SetChainID persists the chain identifier the wallet builds transactions for.
func (c *ConfigService) SetChainID(id uint64) error {
	return c.updateSettings(func(s *Settings) error {
		s.ChainID = id
		return nil
	})
}

// SetAllowMainnetSend persists the explicit opt-in for signing and publishing
// transactions on chain 1. The TxService guard remains authoritative; this
// setter only records the user's decision.
func (c *ConfigService) SetAllowMainnetSend(v bool) error {
	return c.updateSettings(func(s *Settings) error {
		s.AllowMainnetSend = v
		return nil
	})
}

// SetAutoReceive persists the auto-receive toggle.
func (c *ConfigService) SetAutoReceive(v bool) error {
	return c.updateSettings(func(s *Settings) error {
		s.AutoReceive = v
		return nil
	})
}

// SetUIMode persists the shell layout mode. It is a display preference only.
func (c *ConfigService) SetUIMode(mode string) error {
	if mode != uiModeEveryday && mode != uiModeAdvanced {
		return errors.New("unknown layout mode")
	}
	return c.updateSettings(func(s *Settings) error {
		s.UIMode = mode
		return nil
	})
}

// SetPinnedActions persists the Advanced overview's pinned shortcuts. Unknown
// ids are rejected rather than dropped so a frontend bug cannot pass silently.
func (c *ConfigService) SetPinnedActions(ids []string) error {
	if len(ids) > maxPinnedActions {
		return fmt.Errorf("at most %d actions can be pinned", maxPinnedActions)
	}
	clean := sanitizePinnedActions(ids)
	if len(clean) != len(ids) {
		return errors.New("unknown or duplicate pinned action")
	}
	return c.updateSettings(func(s *Settings) error {
		s.PinnedActions = clean
		return nil
	})
}

// GetBuildInfo reports the running binary's release version and build commit.
// Read-only and secret-free by construction — safe to expose to the WebView.
func (c *ConfigService) GetBuildInfo() BuildInfo {
	return BuildInfo{Version: version.Version, Commit: version.Commit()}
}
