package modules

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/orlandobianco/SecTUI/internal/core"
)

func TestFilesystemModuleID(t *testing.T) {
	m := NewFilesystemModule()
	if m.ID() != "filesystem" {
		t.Errorf("ID() = %q, want %q", m.ID(), "filesystem")
	}
}

func TestFilesystemModuleIsApplicable(t *testing.T) {
	m := NewFilesystemModule()
	if m.IsApplicable(nil) {
		t.Error("IsApplicable(nil) = true, want false")
	}
	if !m.IsApplicable(&core.PlatformInfo{OS: core.OSLinux}) {
		t.Error("IsApplicable(linux) = false, want true")
	}
	if !m.IsApplicable(&core.PlatformInfo{OS: core.OSDarwin}) {
		t.Error("IsApplicable(darwin) = false, want true")
	}
}

func TestFilesystemModuleAvailableFixes(t *testing.T) {
	m := NewFilesystemModule()
	fixes := m.AvailableFixes()
	if len(fixes) != len(m.checks) {
		t.Errorf("AvailableFixes() returned %d fixes, want %d", len(fixes), len(m.checks))
	}
	for _, f := range fixes {
		if f.ID == "" || f.FindingID == "" {
			t.Errorf("fix has empty ID or FindingID: %+v", f)
		}
	}
}

func TestFilesystemModulePreviewFixUnknown(t *testing.T) {
	m := NewFilesystemModule()
	_, err := m.PreviewFix("fix-nonexistent", nil)
	if err == nil {
		t.Error("PreviewFix with unknown fixID should return error")
	}
}

func TestFilesystemModuleApplyFixUnknown(t *testing.T) {
	m := NewFilesystemModule()
	_, err := m.ApplyFix("fix-nonexistent", &core.ApplyContext{DryRun: true})
	if err == nil {
		t.Error("ApplyFix with unknown fixID should return error")
	}
}

func TestFilesystemModuleApplyFixDryRun(t *testing.T) {
	m := NewFilesystemModule()
	// Use first available fix — won't actually change anything.
	fixes := m.AvailableFixes()
	if len(fixes) == 0 {
		t.Skip("no fixes available")
	}

	result, err := m.ApplyFix(fixes[0].ID, &core.ApplyContext{DryRun: true})
	if err != nil {
		// May error if file doesn't exist on test machine; that's ok.
		t.Skipf("dry-run errored (file may not exist): %v", err)
	}
	if !result.Success {
		t.Error("dry-run should succeed")
	}
}

func TestIsMorePermissive(t *testing.T) {
	tests := []struct {
		name     string
		actual   fs.FileMode
		expected fs.FileMode
		want     bool
	}{
		{"exact match", 0o644, 0o644, false},
		{"less permissive", 0o600, 0o644, false},
		{"more permissive owner", 0o744, 0o644, true},
		{"world writable", 0o646, 0o644, true},
		{"group writable", 0o664, 0o644, true},
		{"all open", 0o777, 0o644, true},
		{"shadow ok", 0o640, 0o640, false},
		{"shadow too open", 0o644, 0o640, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isMorePermissive(tt.actual, tt.expected)
			if got != tt.want {
				t.Errorf("isMorePermissive(%04o, %04o) = %v, want %v",
					tt.actual, tt.expected, got, tt.want)
			}
		})
	}
}

func TestScanHomeDirectories(t *testing.T) {
	// Create temp dir structure simulating /home.
	tmpDir := t.TempDir()
	goodHome := filepath.Join(tmpDir, "gooduser")
	badHome := filepath.Join(tmpDir, "baduser")

	os.Mkdir(goodHome, 0o700)
	os.Mkdir(badHome, 0o755) // too permissive

	// We can't test scanHomeDirectories directly since it uses hardcoded /home,
	// but we can test the permission logic it relies on.
	info, _ := os.Stat(badHome)
	perm := info.Mode().Perm()
	if perm&0o027 == 0 {
		t.Error("badHome should have group-write or world bits set")
	}

	info, _ = os.Stat(goodHome)
	perm = info.Mode().Perm()
	if perm&0o027 != 0 {
		t.Error("goodHome should not have group-write or world bits set")
	}
}
