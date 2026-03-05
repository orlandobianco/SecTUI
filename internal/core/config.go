package core

import (
	"bytes"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// --- Config structs ---

type AppConfig struct {
	General       GeneralConfig       `toml:"general"`
	Scan          ScanConfig          `toml:"scan"`
	Notifications NotificationsConfig `toml:"notifications"`
	Dashboard     DashboardConfig     `toml:"dashboard"`
	Harden        HardenConfig        `toml:"harden"`
}

type GeneralConfig struct {
	Locale string `toml:"locale"`
	Color  bool   `toml:"color"`
}

type ScanConfig struct {
	DefaultType     string   `toml:"default_type"`
	ExcludedModules []string `toml:"excluded_modules"`
}

type NotificationsConfig struct {
	Enabled  bool           `toml:"enabled"`
	Telegram TelegramConfig `toml:"telegram"`
	Discord  DiscordConfig  `toml:"discord"`
}

type TelegramConfig struct {
	Enabled bool   `toml:"enabled"`
	Token   string `toml:"token"`
	ChatID  string `toml:"chat_id"`
}

type DiscordConfig struct {
	Enabled    bool   `toml:"enabled"`
	WebhookURL string `toml:"webhook_url"`
}

type DashboardConfig struct {
	RefreshInterval int `toml:"refresh_interval"`
}

type HardenConfig struct {
	AutoBackup    bool `toml:"auto_backup"`
	BackupDefault bool `toml:"backup_default"`
	DryRunDefault bool `toml:"dry_run_default"`
}

// --- Functions ---

// DefaultConfig returns an AppConfig with sensible defaults.
func DefaultConfig() *AppConfig {
	return &AppConfig{
		General: GeneralConfig{
			Locale: "en",
			Color:  true,
		},
		Scan: ScanConfig{
			DefaultType:     "quick",
			ExcludedModules: []string{},
		},
		Notifications: NotificationsConfig{
			Enabled: false,
			Telegram: TelegramConfig{
				Enabled: false,
			},
			Discord: DiscordConfig{
				Enabled: false,
			},
		},
		Dashboard: DashboardConfig{
			RefreshInterval: 5,
		},
		Harden: HardenConfig{
			AutoBackup:    true,
			BackupDefault: true,
			DryRunDefault: true,
		},
	}
}

// ConfigPath returns the path to the SecTUI config file.
// Uses os.UserConfigDir to resolve the platform-appropriate config directory.
func ConfigPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "sectui", "config.toml"), nil
}

// LoadConfigFrom reads the config from a specific file path.
func LoadConfigFrom(path string) (*AppConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return DefaultConfig(), err
	}

	cfg := DefaultConfig()
	if _, err := toml.Decode(string(data), cfg); err != nil {
		return DefaultConfig(), err
	}

	return cfg, nil
}

// LoadConfig reads the config from disk. If the file does not exist,
// it returns the default config without error.
func LoadConfig() (*AppConfig, error) {
	path, err := ConfigPath()
	if err != nil {
		return DefaultConfig(), err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultConfig(), nil
		}
		return DefaultConfig(), err
	}

	cfg := DefaultConfig()
	if _, err := toml.Decode(string(data), cfg); err != nil {
		return DefaultConfig(), err
	}

	return cfg, nil
}

// SaveConfig writes the config to disk as TOML, creating parent
// directories if they do not exist.
func SaveConfig(cfg *AppConfig) error {
	path, err := ConfigPath()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	var buf bytes.Buffer
	enc := toml.NewEncoder(&buf)
	if err := enc.Encode(cfg); err != nil {
		return err
	}

	return os.WriteFile(path, buf.Bytes(), 0o644)
}
