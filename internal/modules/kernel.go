package modules

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/orlandobianco/SecTUI/internal/core"
)

const (
	kernelModuleID   = "kernel"
	sysctlConfPath   = "/etc/sysctl.d/99-sectui.conf"
	sysctlConfHeader = "# Managed by SecTUI - Kernel hardening parameters\n# Generated: %s\n# Do not edit manually; changes may be overwritten by SecTUI.\n\n"
)

// kernelCheck defines a single sysctl parameter to audit against an expected hardened value.
type kernelCheck struct {
	id        string
	param     string
	expected  string
	severity  core.Severity
	titleKey  string
	detailKey string
	fixID     string
	// linuxOnly indicates parameters that only exist on Linux (e.g. net.ipv4.*).
	// When true the check is silently skipped on non-Linux platforms.
	linuxOnly bool
}

// KernelModule audits kernel sysctl parameters for security hardening.
// The Scan method is strictly read-only -- it never writes to the system.
type KernelModule struct {
	checks []kernelCheck
}

// NewKernelModule returns a KernelModule pre-loaded with all hardening checks.
func NewKernelModule() *KernelModule {
	return &KernelModule{
		checks: []kernelCheck{
			{
				id:        "kern-001",
				param:     "net.ipv4.conf.all.rp_filter",
				expected:  "1",
				severity:  core.SeverityMedium,
				titleKey:  "finding.kern_rp_filter.title",
				detailKey: "finding.kern_rp_filter.detail",
				fixID:     "fix-kern-001",
				linuxOnly: true,
			},
			{
				id:        "kern-002",
				param:     "net.ipv4.icmp_echo_ignore_broadcasts",
				expected:  "1",
				severity:  core.SeverityMedium,
				titleKey:  "finding.kern_icmp_broadcast.title",
				detailKey: "finding.kern_icmp_broadcast.detail",
				fixID:     "fix-kern-002",
				linuxOnly: true,
			},
			{
				id:        "kern-003",
				param:     "net.ipv4.conf.all.accept_redirects",
				expected:  "0",
				severity:  core.SeverityMedium,
				titleKey:  "finding.kern_accept_redirects.title",
				detailKey: "finding.kern_accept_redirects.detail",
				fixID:     "fix-kern-003",
				linuxOnly: true,
			},
			{
				id:        "kern-004",
				param:     "net.ipv4.conf.all.send_redirects",
				expected:  "0",
				severity:  core.SeverityMedium,
				titleKey:  "finding.kern_send_redirects.title",
				detailKey: "finding.kern_send_redirects.detail",
				fixID:     "fix-kern-004",
				linuxOnly: true,
			},
			{
				id:        "kern-005",
				param:     "net.ipv4.conf.all.accept_source_route",
				expected:  "0",
				severity:  core.SeverityMedium,
				titleKey:  "finding.kern_accept_source_route.title",
				detailKey: "finding.kern_accept_source_route.detail",
				fixID:     "fix-kern-005",
				linuxOnly: true,
			},
			{
				id:        "kern-006",
				param:     "net.ipv4.tcp_syncookies",
				expected:  "1",
				severity:  core.SeverityHigh,
				titleKey:  "finding.kern_tcp_syncookies.title",
				detailKey: "finding.kern_tcp_syncookies.detail",
				fixID:     "fix-kern-006",
				linuxOnly: true,
			},
			{
				id:        "kern-007",
				param:     "kernel.randomize_va_space",
				expected:  "2",
				severity:  core.SeverityHigh,
				titleKey:  "finding.kern_aslr.title",
				detailKey: "finding.kern_aslr.detail",
				fixID:     "fix-kern-007",
				linuxOnly: true,
			},
			{
				id:        "kern-008",
				param:     "kernel.kptr_restrict",
				expected:  "2",
				severity:  core.SeverityMedium,
				titleKey:  "finding.kern_kptr_restrict.title",
				detailKey: "finding.kern_kptr_restrict.detail",
				fixID:     "fix-kern-008",
				linuxOnly: true,
			},
			{
				id:        "kern-009",
				param:     "kernel.dmesg_restrict",
				expected:  "1",
				severity:  core.SeverityLow,
				titleKey:  "finding.kern_dmesg_restrict.title",
				detailKey: "finding.kern_dmesg_restrict.detail",
				fixID:     "fix-kern-009",
				linuxOnly: true,
			},
			{
				id:        "kern-010",
				param:     "kernel.yama.ptrace_scope",
				expected:  "1",
				severity:  core.SeverityMedium,
				titleKey:  "finding.kern_ptrace_scope.title",
				detailKey: "finding.kern_ptrace_scope.detail",
				fixID:     "fix-kern-010",
				linuxOnly: true,
			},
			{
				id:        "kern-011",
				param:     "fs.suid_dumpable",
				expected:  "0",
				severity:  core.SeverityMedium,
				titleKey:  "finding.kern_suid_dumpable.title",
				detailKey: "finding.kern_suid_dumpable.detail",
				fixID:     "fix-kern-011",
				linuxOnly: true,
			},
			{
				id:        "kern-012",
				param:     "fs.protected_hardlinks",
				expected:  "1",
				severity:  core.SeverityLow,
				titleKey:  "finding.kern_protected_hardlinks.title",
				detailKey: "finding.kern_protected_hardlinks.detail",
				fixID:     "fix-kern-012",
				linuxOnly: true,
			},
			{
				id:        "kern-013",
				param:     "fs.protected_symlinks",
				expected:  "1",
				severity:  core.SeverityLow,
				titleKey:  "finding.kern_protected_symlinks.title",
				detailKey: "finding.kern_protected_symlinks.detail",
				fixID:     "fix-kern-013",
				linuxOnly: true,
			},
		},
	}
}

// ---------------------------------------------------------------------------
// SecurityModule interface
// ---------------------------------------------------------------------------

func (m *KernelModule) ID() string             { return kernelModuleID }
func (m *KernelModule) NameKey() string        { return "module.kernel.name" }
func (m *KernelModule) DescriptionKey() string { return "module.kernel.description" }
func (m *KernelModule) Priority() int          { return 60 }

// IsApplicable returns true only on Linux -- most sysctl parameters checked
// here are Linux-specific and do not exist on macOS.
func (m *KernelModule) IsApplicable(platform *core.PlatformInfo) bool {
	if platform == nil {
		return false
	}
	return platform.OS == core.OSLinux
}

// Scan reads each sysctl parameter and compares it against the hardened value.
// This method is strictly read-only and never modifies system state.
func (m *KernelModule) Scan(ctx *core.ScanContext) []core.Finding {
	var findings []core.Finding

	for _, c := range m.checks {
		// Skip Linux-only checks when not running on Linux.
		if c.linuxOnly && ctx != nil && ctx.Platform != nil && ctx.Platform.OS != core.OSLinux {
			continue
		}

		actual, err := readSysctl(c.param)
		if err != nil {
			// Parameter does not exist on this kernel -- skip silently.
			continue
		}

		if actual != c.expected {
			findings = append(findings, core.Finding{
				ID:            c.id,
				Module:        kernelModuleID,
				Severity:      c.severity,
				TitleKey:      c.titleKey,
				DetailKey:     c.detailKey,
				FixID:         c.fixID,
				CurrentValue:  actual,
				ExpectedValue: c.expected,
			})
		}
	}

	return findings
}

// AvailableFixes returns one fix per check (set param to expected value).
func (m *KernelModule) AvailableFixes() []core.Fix {
	fixes := make([]core.Fix, len(m.checks))
	for i, c := range m.checks {
		fixes[i] = core.Fix{
			ID:          c.fixID,
			FindingID:   c.id,
			TitleKey:    c.titleKey,
			Description: fmt.Sprintf("Set %s = %s in %s", c.param, c.expected, sysctlConfPath),
			Dangerous:   c.severity >= core.SeverityHigh,
		}
	}
	return fixes
}

// PreviewFix returns a human-readable description of what ApplyFix would do
// for a single check, without making any changes.
func (m *KernelModule) PreviewFix(fixID string, _ *core.ScanContext) (string, error) {
	check := m.findCheckByFixID(fixID)
	if check == nil {
		return "", fmt.Errorf("unknown fix: %s", fixID)
	}

	current, err := readSysctl(check.param)
	if err != nil {
		current = "<unavailable>"
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("Would set %s = %s in %s\n", check.param, check.expected, sysctlConfPath))
	b.WriteString(fmt.Sprintf("  Current value : %s\n", current))
	b.WriteString(fmt.Sprintf("  Expected value: %s\n", check.expected))
	b.WriteString(fmt.Sprintf("Then run: sysctl --system\n"))
	return b.String(), nil
}

// ApplyFix writes all unhardened values for the given fixID to the sysctl
// drop-in configuration file and reloads sysctl. It respects DryRun and
// Backup flags in ApplyContext.
func (m *KernelModule) ApplyFix(fixID string, ctx *core.ApplyContext) (*core.ApplyResult, error) {
	check := m.findCheckByFixID(fixID)
	if check == nil {
		return nil, fmt.Errorf("unknown fix: %s", fixID)
	}

	// ---- Dry-run: just report what would change ----
	if ctx.DryRun {
		return &core.ApplyResult{
			Success: true,
			Message: fmt.Sprintf("[dry-run] Would set %s = %s in %s and run sysctl --system", check.param, check.expected, sysctlConfPath),
		}, nil
	}

	// ---- Backup existing config if requested and file exists ----
	backupPath := ""
	if ctx.Backup {
		if _, err := os.Stat(sysctlConfPath); err == nil {
			backupPath = sysctlConfPath + ".bak"
			data, err := os.ReadFile(sysctlConfPath)
			if err != nil {
				return nil, fmt.Errorf("failed to read existing config for backup: %w", err)
			}
			if err := os.WriteFile(backupPath, data, 0o644); err != nil {
				return nil, fmt.Errorf("failed to create backup at %s: %w", backupPath, err)
			}
		}
	}

	// ---- Read or initialise the config file ----
	var existing string
	if data, err := os.ReadFile(sysctlConfPath); err == nil {
		existing = string(data)
	} else {
		existing = fmt.Sprintf(sysctlConfHeader, time.Now().UTC().Format(time.RFC3339))
	}

	// ---- Upsert the parameter ----
	newContent := upsertSysctlParam(existing, check.param, check.expected)

	if err := os.WriteFile(sysctlConfPath, []byte(newContent), 0o644); err != nil {
		return nil, fmt.Errorf("failed to write %s: %w", sysctlConfPath, err)
	}

	// ---- Reload sysctl ----
	if out, err := exec.Command("sysctl", "--system").CombinedOutput(); err != nil {
		return nil, fmt.Errorf("sysctl --system failed: %s: %w", strings.TrimSpace(string(out)), err)
	}

	return &core.ApplyResult{
		Success:    true,
		Message:    fmt.Sprintf("Set %s = %s and reloaded sysctl", check.param, check.expected),
		BackupPath: backupPath,
	}, nil
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

// findCheckByFixID returns a pointer to the check matching the given fix ID,
// or nil if not found.
func (m *KernelModule) findCheckByFixID(fixID string) *kernelCheck {
	for i := range m.checks {
		if m.checks[i].fixID == fixID {
			return &m.checks[i]
		}
	}
	return nil
}

// readSysctl executes `sysctl -n <param>` and returns the trimmed output.
// It returns an error if the parameter does not exist or the command fails.
func readSysctl(param string) (string, error) {
	out, err := exec.Command("sysctl", "-n", param).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("sysctl -n %s: %w", param, err)
	}
	return strings.TrimSpace(string(out)), nil
}

// upsertSysctlParam sets or replaces a parameter in sysctl.conf content.
// If the parameter already exists (commented or active), the line is replaced.
// Otherwise the parameter is appended at the end.
func upsertSysctlParam(content, param, value string) string {
	lines := strings.Split(content, "\n")
	entry := param + " = " + value
	replaced := false

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Match active line: "param = value" or "param=value"
		if keyFromSysctlLine(trimmed) == param {
			lines[i] = entry
			replaced = true
			break
		}

		// Match commented-out line: "# param = value"
		if strings.HasPrefix(trimmed, "#") {
			uncommented := strings.TrimSpace(strings.TrimPrefix(trimmed, "#"))
			if keyFromSysctlLine(uncommented) == param {
				lines[i] = entry
				replaced = true
				break
			}
		}
	}

	if !replaced {
		// Ensure there is a trailing newline before appending.
		if len(lines) > 0 && lines[len(lines)-1] != "" {
			lines = append(lines, "")
		}
		lines = append(lines, entry)
	}

	return strings.Join(lines, "\n")
}

// keyFromSysctlLine extracts the parameter name from a sysctl.conf line.
// It handles both "key = value" and "key=value" formats.
func keyFromSysctlLine(line string) string {
	// Try "key = value" first (space-separated with =)
	if idx := strings.IndexByte(line, '='); idx != -1 {
		return strings.TrimSpace(line[:idx])
	}
	// Fallback: first whitespace-delimited token
	if fields := strings.Fields(line); len(fields) >= 1 {
		return fields[0]
	}
	return ""
}
