package modules

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/orlandobianco/SecTUI/internal/core"
)

func TestParseSSHDConfig_Basic(t *testing.T) {
	content := `# SSH config
PermitRootLogin no
PasswordAuthentication yes
MaxAuthTries 3
`
	tmpFile := writeTempFile(t, content)

	settings, err := parseSSHDConfig(tmpFile)
	if err != nil {
		t.Fatalf("parseSSHDConfig: %v", err)
	}

	tests := []struct {
		key, want string
	}{
		{"permitrootlogin", "no"},
		{"passwordauthentication", "yes"},
		{"maxauthtries", "3"},
	}

	for _, tt := range tests {
		if got := settings[tt.key]; got != tt.want {
			t.Errorf("settings[%q] = %q, want %q", tt.key, got, tt.want)
		}
	}
}

func TestParseSSHDConfig_IgnoresComments(t *testing.T) {
	content := `# This is a comment
#PermitRootLogin yes
PasswordAuthentication no
`
	tmpFile := writeTempFile(t, content)
	settings, err := parseSSHDConfig(tmpFile)
	if err != nil {
		t.Fatalf("parseSSHDConfig: %v", err)
	}

	if _, exists := settings["permitrootlogin"]; exists {
		t.Error("commented-out PermitRootLogin should not be parsed")
	}
	if settings["passwordauthentication"] != "no" {
		t.Errorf("PasswordAuthentication = %q, want no", settings["passwordauthentication"])
	}
}

func TestParseSSHDConfig_EqualsFormat(t *testing.T) {
	content := `PermitRootLogin=no
MaxAuthTries=5
`
	tmpFile := writeTempFile(t, content)
	settings, err := parseSSHDConfig(tmpFile)
	if err != nil {
		t.Fatalf("parseSSHDConfig: %v", err)
	}

	if settings["permitrootlogin"] != "no" {
		t.Errorf("PermitRootLogin = %q, want no", settings["permitrootlogin"])
	}
	if settings["maxauthtries"] != "5" {
		t.Errorf("MaxAuthTries = %q, want 5", settings["maxauthtries"])
	}
}

func TestParseSSHDConfig_MissingFile(t *testing.T) {
	_, err := parseSSHDConfig("/nonexistent/path/sshd_config")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestSetSSHDValue_ReplacesExisting(t *testing.T) {
	content := `PermitRootLogin yes
PasswordAuthentication yes
`
	result := setSSHDValue(content, "PermitRootLogin", "no")

	settings := parseSSHDContent(t, result)
	if settings["permitrootlogin"] != "no" {
		t.Errorf("PermitRootLogin = %q, want no", settings["permitrootlogin"])
	}
	// Other settings should be untouched.
	if settings["passwordauthentication"] != "yes" {
		t.Errorf("PasswordAuthentication = %q, want yes", settings["passwordauthentication"])
	}
}

func TestSetSSHDValue_UncommentsAndReplaces(t *testing.T) {
	content := `#PermitRootLogin prohibit-password
PasswordAuthentication yes
`
	result := setSSHDValue(content, "PermitRootLogin", "no")

	settings := parseSSHDContent(t, result)
	if settings["permitrootlogin"] != "no" {
		t.Errorf("PermitRootLogin = %q, want no", settings["permitrootlogin"])
	}
}

func TestSetSSHDValue_AppendsIfMissing(t *testing.T) {
	content := `PasswordAuthentication yes
`
	result := setSSHDValue(content, "MaxAuthTries", "3")

	settings := parseSSHDContent(t, result)
	if settings["maxauthtries"] != "3" {
		t.Errorf("MaxAuthTries = %q, want 3", settings["maxauthtries"])
	}
}

func TestSshExpectExact(t *testing.T) {
	tests := []struct {
		actual, expected string
		want             bool
	}{
		{"no", "no", true},
		{"No", "no", true},
		{"yes", "no", false},
		{"", "no", false},
	}
	for _, tt := range tests {
		got := sshExpectExact(tt.actual, tt.expected)
		if got != tt.want {
			t.Errorf("sshExpectExact(%q, %q) = %v, want %v", tt.actual, tt.expected, got, tt.want)
		}
	}
}

func TestSshExpectLessOrEqual(t *testing.T) {
	tests := []struct {
		actual, expected string
		want             bool
	}{
		{"3", "3", true},
		{"1", "3", true},
		{"6", "3", false},
		{"abc", "3", false},
		{"3", "abc", false},
	}
	for _, tt := range tests {
		got := sshExpectLessOrEqual(tt.actual, tt.expected)
		if got != tt.want {
			t.Errorf("sshExpectLessOrEqual(%q, %q) = %v, want %v", tt.actual, tt.expected, got, tt.want)
		}
	}
}

func TestSSHModule_ID(t *testing.T) {
	m := NewSSHModule()
	if m.ID() != "ssh" {
		t.Errorf("ID() = %q, want ssh", m.ID())
	}
}

func TestSSHModule_AvailableFixes(t *testing.T) {
	m := NewSSHModule()
	fixes := m.AvailableFixes()
	if len(fixes) != 7 {
		t.Errorf("AvailableFixes() len = %d, want 7", len(fixes))
	}
	for _, f := range fixes {
		if f.ID == "" {
			t.Error("fix has empty ID")
		}
		if f.FindingID == "" {
			t.Error("fix has empty FindingID")
		}
	}
}

func TestSSHModule_PreviewFix_UnknownFix(t *testing.T) {
	m := NewSSHModule()
	_, err := m.PreviewFix("nonexistent", nil)
	if err == nil {
		t.Error("expected error for unknown fix ID")
	}
}

func TestSSHModule_ApplyFix_UnknownFix(t *testing.T) {
	m := NewSSHModule()
	_, err := m.ApplyFix("nonexistent", &core.ApplyContext{DryRun: true})
	if err == nil {
		t.Error("expected error for unknown fix ID")
	}
}

func TestSSHModule_Scan_WithTempConfig(t *testing.T) {
	// Create a temp sshd_config with known insecure values.
	content := `PermitRootLogin yes
PasswordAuthentication yes
PermitEmptyPasswords no
PubkeyAuthentication yes
MaxAuthTries 6
X11Forwarding yes
LoginGraceTime 120
`
	tmpFile := writeTempFile(t, content)

	// We can't easily override the sshdConfigPath constant,
	// so we test the parsing logic directly.
	settings, err := parseSSHDConfig(tmpFile)
	if err != nil {
		t.Fatalf("parseSSHDConfig: %v", err)
	}

	// Verify the insecure values are correctly parsed.
	if settings["permitrootlogin"] != "yes" {
		t.Errorf("PermitRootLogin = %q, want yes", settings["permitrootlogin"])
	}
	if settings["x11forwarding"] != "yes" {
		t.Errorf("X11Forwarding = %q, want yes", settings["x11forwarding"])
	}
	if settings["maxauthtries"] != "6" {
		t.Errorf("MaxAuthTries = %q, want 6", settings["maxauthtries"])
	}
}

// --- helpers ---

func writeTempFile(t *testing.T, content string) string {
	t.Helper()
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "sshd_config")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	return path
}

// parseSSHDContent is a test helper that parses SSH config from a string.
func parseSSHDContent(t *testing.T, content string) map[string]string {
	t.Helper()
	tmpFile := writeTempFile(t, content)
	settings, err := parseSSHDConfig(tmpFile)
	if err != nil {
		t.Fatalf("parseSSHDConfig: %v", err)
	}
	return settings
}
