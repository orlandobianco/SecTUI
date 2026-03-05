package modules

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/orlandobianco/SecTUI/internal/core"
)

const filesystemModuleID = "filesystem"

type fileCheck struct {
	id            string
	path          string
	expectedMode  fs.FileMode
	expectedOwner string // "root" or empty (skip owner check)
	severity      core.Severity
	titleKey      string
	detailKey     string
	fixID         string
	linuxOnly     bool
}

type FilesystemModule struct {
	checks []fileCheck
}

func NewFilesystemModule() *FilesystemModule {
	return &FilesystemModule{
		checks: []fileCheck{
			{
				id:            "fs-001",
				path:          "/etc/passwd",
				expectedMode:  0o644,
				severity:      core.SeverityHigh,
				titleKey:      "finding.fs_passwd_perms.title",
				detailKey:     "finding.fs_passwd_perms.detail",
				fixID:         "fix-fs-001",
			},
			{
				id:            "fs-002",
				path:          "/etc/shadow",
				expectedMode:  0o640,
				severity:      core.SeverityCritical,
				titleKey:      "finding.fs_shadow_perms.title",
				detailKey:     "finding.fs_shadow_perms.detail",
				fixID:         "fix-fs-002",
				linuxOnly:     true,
			},
			{
				id:            "fs-003",
				path:          "/etc/ssh/sshd_config",
				expectedMode:  0o600,
				severity:      core.SeverityHigh,
				titleKey:      "finding.fs_sshd_config_perms.title",
				detailKey:     "finding.fs_sshd_config_perms.detail",
				fixID:         "fix-fs-003",
			},
			{
				id:            "fs-004",
				path:          "/etc/sudoers",
				expectedMode:  0o440,
				severity:      core.SeverityHigh,
				titleKey:      "finding.fs_sudoers_perms.title",
				detailKey:     "finding.fs_sudoers_perms.detail",
				fixID:         "fix-fs-004",
				linuxOnly:     true,
			},
		},
	}
}

func (m *FilesystemModule) ID() string             { return filesystemModuleID }
func (m *FilesystemModule) NameKey() string        { return "module.filesystem.name" }
func (m *FilesystemModule) DescriptionKey() string { return "module.filesystem.description" }
func (m *FilesystemModule) Priority() int          { return 60 }

func (m *FilesystemModule) IsApplicable(platform *core.PlatformInfo) bool {
	return platform != nil
}

func (m *FilesystemModule) Scan(ctx *core.ScanContext) []core.Finding {
	var findings []core.Finding

	// Fixed-path permission checks.
	for _, c := range m.checks {
		if c.linuxOnly && ctx.Platform != nil && ctx.Platform.OS != core.OSLinux {
			continue
		}

		info, err := os.Stat(c.path)
		if err != nil {
			continue // file doesn't exist, skip silently
		}

		actualMode := info.Mode().Perm()
		if isMorePermissive(actualMode, c.expectedMode) {
			findings = append(findings, core.Finding{
				ID:            c.id,
				Module:        filesystemModuleID,
				Severity:      c.severity,
				TitleKey:      c.titleKey,
				DetailKey:     c.detailKey,
				FixID:         c.fixID,
				CurrentValue:  fmt.Sprintf("%04o", actualMode),
				ExpectedValue: fmt.Sprintf("%04o", c.expectedMode),
			})
		}
	}

	// Home directory permissions.
	findings = append(findings, m.scanHomeDirectories(ctx)...)

	// World-writable files in /etc/.
	if ctx.Platform != nil && ctx.Platform.OS == core.OSLinux {
		findings = append(findings, m.scanWorldWritableEtc()...)
	}

	return findings
}

// scanHomeDirectories checks that human user home dirs are not group/world-readable.
func (m *FilesystemModule) scanHomeDirectories(ctx *core.ScanContext) []core.Finding {
	homeBase := "/home"
	if ctx.Platform != nil && ctx.Platform.OS == core.OSDarwin {
		homeBase = "/Users"
	}

	entries, err := os.ReadDir(homeBase)
	if err != nil {
		return nil
	}

	var findings []core.Finding
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		// Skip common system/hidden dirs.
		if strings.HasPrefix(name, ".") || name == "Shared" || name == "lost+found" {
			continue
		}

		homeDir := filepath.Join(homeBase, name)
		info, err := os.Stat(homeDir)
		if err != nil {
			continue
		}

		perm := info.Mode().Perm()
		// Home dirs should be 0700 or at most 0750.
		if perm&0o027 != 0 { // has group-write or any world bits
			findings = append(findings, core.Finding{
				ID:            "fs-005",
				Module:        filesystemModuleID,
				Severity:      core.SeverityMedium,
				TitleKey:      "finding.fs_home_dir_perms.title",
				DetailKey:     "finding.fs_home_dir_perms.detail",
				CurrentValue:  fmt.Sprintf("%s (%04o)", homeDir, perm),
				ExpectedValue: "0700 or 0750",
			})
		}
	}

	return findings
}

// scanWorldWritableEtc looks for world-writable files under /etc/.
func (m *FilesystemModule) scanWorldWritableEtc() []core.Finding {
	var worldWritable []string
	maxEntries := 10 // cap to avoid huge scans

	_ = filepath.WalkDir("/etc", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // skip unreadable dirs
		}
		if d.IsDir() {
			return nil
		}
		if len(worldWritable) >= maxEntries {
			return filepath.SkipAll
		}

		info, err := d.Info()
		if err != nil {
			return nil
		}

		if info.Mode().Perm()&0o002 != 0 {
			worldWritable = append(worldWritable, path)
		}
		return nil
	})

	if len(worldWritable) == 0 {
		return nil
	}

	detail := strings.Join(worldWritable, ", ")
	if len(worldWritable) >= maxEntries {
		detail += " (and possibly more)"
	}

	return []core.Finding{{
		ID:            "fs-006",
		Module:        filesystemModuleID,
		Severity:      core.SeverityMedium,
		TitleKey:      "finding.fs_world_writable_etc.title",
		DetailKey:     "finding.fs_world_writable_etc.detail",
		CurrentValue:  detail,
		ExpectedValue: "No world-writable files in /etc",
	}}
}

func (m *FilesystemModule) AvailableFixes() []core.Fix {
	var fixes []core.Fix
	for _, c := range m.checks {
		fixes = append(fixes, core.Fix{
			ID:          c.fixID,
			FindingID:   c.id,
			TitleKey:    c.titleKey,
			Description: fmt.Sprintf("Set %s permissions to %04o", c.path, c.expectedMode),
		})
	}
	return fixes
}

func (m *FilesystemModule) PreviewFix(fixID string, _ *core.ScanContext) (string, error) {
	check := m.findCheckByFixID(fixID)
	if check == nil {
		return "", fmt.Errorf("unknown fix: %s", fixID)
	}

	info, err := os.Stat(check.path)
	if err != nil {
		return "", fmt.Errorf("cannot stat %s: %w", check.path, err)
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("--- %s (current)\n", check.path))
	b.WriteString(fmt.Sprintf("+++ %s (fixed)\n", check.path))
	b.WriteString(fmt.Sprintf("- permissions: %04o\n", info.Mode().Perm()))
	b.WriteString(fmt.Sprintf("+ permissions: %04o\n", check.expectedMode))
	return b.String(), nil
}

func (m *FilesystemModule) ApplyFix(fixID string, ctx *core.ApplyContext) (*core.ApplyResult, error) {
	check := m.findCheckByFixID(fixID)
	if check == nil {
		return nil, fmt.Errorf("unknown fix: %s", fixID)
	}

	if ctx.DryRun {
		return &core.ApplyResult{
			Success: true,
			Message: fmt.Sprintf("[dry-run] Would set %s to %04o", check.path, check.expectedMode),
		}, nil
	}

	if err := os.Chmod(check.path, check.expectedMode); err != nil {
		return nil, fmt.Errorf("failed to chmod %s: %w", check.path, err)
	}

	return &core.ApplyResult{
		Success: true,
		Message: fmt.Sprintf("Set %s permissions to %04o", check.path, check.expectedMode),
	}, nil
}

func (m *FilesystemModule) findCheckByFixID(fixID string) *fileCheck {
	for i := range m.checks {
		if m.checks[i].fixID == fixID {
			return &m.checks[i]
		}
	}
	return nil
}

// isMorePermissive returns true if actual grants more access than expected.
func isMorePermissive(actual, expected fs.FileMode) bool {
	// Any bit set in actual that is NOT set in expected means more permissive.
	return actual&^expected != 0
}
