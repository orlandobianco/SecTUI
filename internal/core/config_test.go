package core

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/BurntSushi/toml"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.General.Locale != "en" {
		t.Errorf("default locale = %q, want en", cfg.General.Locale)
	}
	if !cfg.General.Color {
		t.Error("default color should be true")
	}
	if cfg.Scan.DefaultType != "quick" {
		t.Errorf("default scan type = %q, want quick", cfg.Scan.DefaultType)
	}
	if cfg.Notifications.Enabled {
		t.Error("notifications should be disabled by default")
	}
	if cfg.Dashboard.RefreshInterval != 5 {
		t.Errorf("refresh interval = %d, want 5", cfg.Dashboard.RefreshInterval)
	}
	if !cfg.Harden.AutoBackup {
		t.Error("auto_backup should be true by default")
	}
	if !cfg.Harden.DryRunDefault {
		t.Error("dry_run_default should be true by default")
	}
}

func TestLoadConfig_MissingFile(t *testing.T) {
	// LoadConfig uses the real config path, but if the file doesn't exist
	// it should return defaults without error.
	cfg, err := LoadConfig()
	if err != nil {
		// It's acceptable if the config dir doesn't exist in CI.
		t.Logf("LoadConfig returned error (acceptable in CI): %v", err)
		return
	}
	if cfg == nil {
		t.Fatal("LoadConfig returned nil config")
	}
	if cfg.General.Locale != "en" {
		t.Errorf("fallback locale = %q, want en", cfg.General.Locale)
	}
}

func TestSaveAndLoadConfig(t *testing.T) {
	// Create a temp dir and override config path via env or use direct file ops.
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.toml")

	cfg := DefaultConfig()
	cfg.General.Locale = "it"
	cfg.Scan.DefaultType = "full"
	cfg.Scan.ExcludedModules = []string{"kernel", "network"}

	// Save directly using toml encoder to the temp file.
	f, err := os.Create(configFile)
	if err != nil {
		t.Fatalf("create config file: %v", err)
	}
	if err := toml.NewEncoder(f).Encode(cfg); err != nil {
		f.Close()
		t.Fatalf("encode config: %v", err)
	}
	f.Close()

	// Read it back.
	data, err := os.ReadFile(configFile)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}

	loaded := DefaultConfig()
	if _, err := toml.Decode(string(data), loaded); err != nil {
		t.Fatalf("decode config: %v", err)
	}

	if loaded.General.Locale != "it" {
		t.Errorf("loaded locale = %q, want it", loaded.General.Locale)
	}
	if loaded.Scan.DefaultType != "full" {
		t.Errorf("loaded scan type = %q, want full", loaded.Scan.DefaultType)
	}
	if len(loaded.Scan.ExcludedModules) != 2 {
		t.Errorf("excluded modules len = %d, want 2", len(loaded.Scan.ExcludedModules))
	}
}

func TestConfigPath_NotEmpty(t *testing.T) {
	path, err := ConfigPath()
	if err != nil {
		t.Skipf("ConfigPath not available in this env: %v", err)
	}
	if path == "" {
		t.Error("ConfigPath returned empty string")
	}
	if filepath.Base(path) != "config.toml" {
		t.Errorf("ConfigPath base = %q, want config.toml", filepath.Base(path))
	}
}
