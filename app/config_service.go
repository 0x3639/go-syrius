package app

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
)

// ConfigService resolves the data directory and persists user settings.
type ConfigService struct {
	ctx context.Context
}

func newConfigService() *ConfigService { return &ConfigService{} }

// dataDir is the app data directory; GO_SYRIUS_DATA_DIR overrides it (tests).
func (c *ConfigService) dataDir() (string, error) {
	if d := os.Getenv("GO_SYRIUS_DATA_DIR"); d != "" {
		if err := os.MkdirAll(d, 0o700); err != nil {
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

func defaultSettings() Settings {
	return Settings{
		NodeMode:      "remote",
		RemoteNodeURL: defaultNodeURL,
		LocalNodeURL:  defaultLocalNodeURL,
		Theme:         "dark",
		ActiveAccount: 0,
	}
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
	if s.LocalNodeURL == "" {
		s.LocalNodeURL = defaultLocalNodeURL
	}
	if s.NodeMode == "" {
		s.NodeMode = "remote"
	}
	if s.Theme == "" {
		s.Theme = "dark"
	}
	s.NodeURL = "" // stop persisting the deprecated field
}

// GetSettings returns persisted settings, or defaults if none exist.
func (c *ConfigService) GetSettings() (Settings, error) {
	d, err := c.dataDir()
	if err != nil {
		return Settings{}, err
	}
	raw, err := os.ReadFile(filepath.Join(d, "settings.json"))
	if os.IsNotExist(err) {
		return defaultSettings(), nil
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

// SetSettings persists settings as JSON.
func (c *ConfigService) SetSettings(s Settings) error {
	d, err := c.dataDir()
	if err != nil {
		return err
	}
	raw, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(d, "settings.json"), raw, 0o600)
}
