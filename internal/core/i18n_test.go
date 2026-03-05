package core

import (
	"testing"
	"testing/fstest"
)

func TestInitI18nAndT(t *testing.T) {
	yamlContent := `
app:
  name: "TestApp"
finding:
  ssh_root_login:
    title: "Root login enabled"
    detail: "This is a detail"
common:
  yes: "Yes"
`
	fs := fstest.MapFS{
		"en.yml": &fstest.MapFile{Data: []byte(yamlContent)},
	}

	if err := InitI18n(fs, "en"); err != nil {
		t.Fatalf("InitI18n failed: %v", err)
	}

	tests := []struct {
		key      string
		expected string
	}{
		{"app.name", "TestApp"},
		{"finding.ssh_root_login.title", "Root login enabled"},
		{"finding.ssh_root_login.detail", "This is a detail"},
		{"common.yes", "Yes"},
		{"nonexistent.key", "nonexistent.key"}, // fallback to key itself
	}

	for _, tt := range tests {
		got := T(tt.key)
		if got != tt.expected {
			t.Errorf("T(%q) = %q, want %q", tt.key, got, tt.expected)
		}
	}
}

func TestTfPlaceholderReplacement(t *testing.T) {
	yamlContent := `
scan:
  progress: "Scanning %{module}... %{percent}%"
`
	fs := fstest.MapFS{
		"en.yml": &fstest.MapFile{Data: []byte(yamlContent)},
	}

	if err := InitI18n(fs, "en"); err != nil {
		t.Fatalf("InitI18n failed: %v", err)
	}

	got := Tf("scan.progress", "module", "ssh", "percent", "75")
	expected := "Scanning ssh... 75%"
	if got != expected {
		t.Errorf("Tf() = %q, want %q", got, expected)
	}
}

func TestInitI18nFallbackToEnglish(t *testing.T) {
	yamlContent := `
app:
  name: "FallbackApp"
`
	fs := fstest.MapFS{
		"en.yml": &fstest.MapFile{Data: []byte(yamlContent)},
	}

	// Request a locale that does not exist; should fall back to "en".
	if err := InitI18n(fs, "fr"); err != nil {
		t.Fatalf("InitI18n with fallback failed: %v", err)
	}

	if CurrentLocale() != "en" {
		t.Errorf("CurrentLocale() = %q, want %q", CurrentLocale(), "en")
	}

	got := T("app.name")
	if got != "FallbackApp" {
		t.Errorf("T(app.name) = %q, want %q", got, "FallbackApp")
	}
}

func TestInitI18nMissingEnglishFails(t *testing.T) {
	fs := fstest.MapFS{} // No files at all.

	err := InitI18n(fs, "en")
	if err == nil {
		t.Fatal("expected error when en.yml is missing, got nil")
	}
}

func TestTBeforeInit(t *testing.T) {
	// Reset translations to simulate pre-init state.
	translations = nil

	got := T("any.key")
	if got != "any.key" {
		t.Errorf("T() before init should return key, got %q", got)
	}
}
