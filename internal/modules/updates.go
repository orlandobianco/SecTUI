package modules

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/orlandobianco/SecTUI/internal/core"
)

const updatesModuleID = "updates"

// UpdatesModule checks the system's update configuration and pending patches.
// All checks are read-only; no system state is modified during scanning.
type UpdatesModule struct{}

func NewUpdatesModule() *UpdatesModule {
	return &UpdatesModule{}
}

func (m *UpdatesModule) ID() string             { return updatesModuleID }
func (m *UpdatesModule) NameKey() string        { return "module.updates.name" }
func (m *UpdatesModule) DescriptionKey() string { return "module.updates.description" }
func (m *UpdatesModule) Priority() int          { return 50 }

func (m *UpdatesModule) IsApplicable(_ *core.PlatformInfo) bool { return true }

func (m *UpdatesModule) Scan(ctx *core.ScanContext) []core.Finding {
	var findings []core.Finding

	if f := m.checkAutoUpdates(ctx); f != nil {
		findings = append(findings, *f)
	}
	if f := m.checkPendingUpdates(ctx); f != nil {
		findings = append(findings, *f)
	}
	if f := m.checkCacheStaleness(ctx); f != nil {
		findings = append(findings, *f)
	}

	return findings
}

func (m *UpdatesModule) AvailableFixes() []core.Fix {
	return []core.Fix{
		{
			ID:          "fix-upd-001",
			FindingID:   "upd-001",
			TitleKey:    "fix.upd_enable_auto_updates.title",
			Description: "Install and configure unattended-upgrades (Debian/Ubuntu only)",
			Dangerous:   false,
		},
	}
}

func (m *UpdatesModule) PreviewFix(fixID string, ctx *core.ScanContext) (string, error) {
	if fixID != "fix-upd-001" {
		return "", fmt.Errorf("unknown fix: %s", fixID)
	}

	if ctx.Platform.PackageManager != core.PkgApt {
		return "", fmt.Errorf("automatic updates fix is only available on Debian/Ubuntu (apt)")
	}

	var b strings.Builder
	b.WriteString("The following commands will be executed:\n")
	b.WriteString("  sudo apt-get install -y unattended-upgrades\n")
	b.WriteString("  sudo dpkg-reconfigure -plow unattended-upgrades\n")
	b.WriteString("\nThis will install and enable automatic security updates.")
	return b.String(), nil
}

func (m *UpdatesModule) ApplyFix(fixID string, ctx *core.ApplyContext) (*core.ApplyResult, error) {
	if fixID != "fix-upd-001" {
		return nil, fmt.Errorf("unknown fix: %s", fixID)
	}

	if ctx.Platform.PackageManager != core.PkgApt {
		return nil, fmt.Errorf("automatic updates fix is only available on Debian/Ubuntu (apt)")
	}

	if ctx.DryRun {
		return &core.ApplyResult{
			Success: true,
			Message: "[dry-run] Would install and configure unattended-upgrades",
		}, nil
	}

	// Step 1: Install unattended-upgrades
	installCmd := exec.Command("sudo", "apt-get", "install", "-y", "unattended-upgrades")
	if out, err := installCmd.CombinedOutput(); err != nil {
		return &core.ApplyResult{
			Success: false,
			Message: fmt.Sprintf("Failed to install unattended-upgrades: %s (%s)", err, strings.TrimSpace(string(out))),
		}, nil
	}

	// Step 2: Configure unattended-upgrades with non-interactive reconfigure
	configCmd := exec.Command("sudo", "dpkg-reconfigure", "-plow", "unattended-upgrades")
	if out, err := configCmd.CombinedOutput(); err != nil {
		return &core.ApplyResult{
			Success: false,
			Message: fmt.Sprintf("Installed unattended-upgrades but failed to configure: %s (%s)", err, strings.TrimSpace(string(out))),
		}, nil
	}

	return &core.ApplyResult{
		Success: true,
		Message: "Installed and configured unattended-upgrades for automatic security updates",
	}, nil
}

// --- check: automatic security updates (upd-001) ---

func (m *UpdatesModule) checkAutoUpdates(ctx *core.ScanContext) *core.Finding {
	switch {
	case ctx.Platform.PackageManager == core.PkgApt:
		return m.checkAutoUpdatesApt()
	case ctx.Platform.PackageManager == core.PkgDnf:
		return m.checkAutoUpdatesDnf()
	case ctx.Platform.OS == core.OSDarwin:
		return m.checkAutoUpdatesDarwin()
	default:
		return nil
	}
}

// checkAutoUpdatesApt verifies that unattended-upgrades is installed on Debian/Ubuntu.
func (m *UpdatesModule) checkAutoUpdatesApt() *core.Finding {
	out, err := exec.Command("dpkg", "-l", "unattended-upgrades").CombinedOutput()
	if err != nil {
		// dpkg -l returns non-zero when the package is not installed
		return &core.Finding{
			ID:            "upd-001",
			Module:        updatesModuleID,
			Severity:      core.SeverityHigh,
			TitleKey:      "finding.upd_no_auto_updates.title",
			DetailKey:     "finding.upd_no_auto_updates.detail",
			FixID:         "fix-upd-001",
			CurrentValue:  "unattended-upgrades not installed",
			ExpectedValue: "unattended-upgrades installed and configured",
		}
	}

	// dpkg -l may succeed but show "un" (un-installed) or "rc" (removed but config remains).
	// A properly installed package line starts with "ii".
	if !strings.Contains(string(out), "ii  unattended-upgrades") {
		return &core.Finding{
			ID:            "upd-001",
			Module:        updatesModuleID,
			Severity:      core.SeverityHigh,
			TitleKey:      "finding.upd_no_auto_updates.title",
			DetailKey:     "finding.upd_no_auto_updates.detail",
			FixID:         "fix-upd-001",
			CurrentValue:  "unattended-upgrades not properly installed",
			ExpectedValue: "unattended-upgrades installed and configured",
		}
	}

	return nil
}

// checkAutoUpdatesDnf verifies that dnf-automatic is installed and its timer is enabled
// on RHEL/Fedora systems.
func (m *UpdatesModule) checkAutoUpdatesDnf() *core.Finding {
	// Check if dnf-automatic is installed
	_, err := exec.Command("rpm", "-q", "dnf-automatic").CombinedOutput()
	if err != nil {
		return &core.Finding{
			ID:            "upd-001",
			Module:        updatesModuleID,
			Severity:      core.SeverityHigh,
			TitleKey:      "finding.upd_no_auto_updates.title",
			DetailKey:     "finding.upd_no_auto_updates.detail",
			CurrentValue:  "dnf-automatic not installed",
			ExpectedValue: "dnf-automatic installed and timer enabled",
		}
	}

	// Check if the systemd timer is enabled
	out, err := exec.Command("systemctl", "is-enabled", "dnf-automatic.timer").CombinedOutput()
	if err != nil || strings.TrimSpace(string(out)) != "enabled" {
		return &core.Finding{
			ID:            "upd-001",
			Module:        updatesModuleID,
			Severity:      core.SeverityHigh,
			TitleKey:      "finding.upd_no_auto_updates.title",
			DetailKey:     "finding.upd_no_auto_updates.detail",
			CurrentValue:  fmt.Sprintf("dnf-automatic.timer: %s", strings.TrimSpace(string(out))),
			ExpectedValue: "dnf-automatic.timer enabled",
		}
	}

	return nil
}

// checkAutoUpdatesDarwin verifies that macOS automatic update checking is enabled.
func (m *UpdatesModule) checkAutoUpdatesDarwin() *core.Finding {
	out, err := exec.Command(
		"defaults", "read",
		"/Library/Preferences/com.apple.SoftwareUpdate",
		"AutomaticCheckEnabled",
	).CombinedOutput()
	if err != nil {
		// If the key does not exist, defaults read returns non-zero.
		// Treat missing key as auto-updates not configured.
		return &core.Finding{
			ID:            "upd-001",
			Module:        updatesModuleID,
			Severity:      core.SeverityHigh,
			TitleKey:      "finding.upd_no_auto_updates.title",
			DetailKey:     "finding.upd_no_auto_updates.detail",
			CurrentValue:  "AutomaticCheckEnabled not set",
			ExpectedValue: "AutomaticCheckEnabled = 1",
		}
	}

	value := strings.TrimSpace(string(out))
	if value != "1" {
		return &core.Finding{
			ID:            "upd-001",
			Module:        updatesModuleID,
			Severity:      core.SeverityHigh,
			TitleKey:      "finding.upd_no_auto_updates.title",
			DetailKey:     "finding.upd_no_auto_updates.detail",
			CurrentValue:  fmt.Sprintf("AutomaticCheckEnabled = %s", value),
			ExpectedValue: "AutomaticCheckEnabled = 1",
		}
	}

	return nil
}

// --- check: pending security updates (upd-002) ---

func (m *UpdatesModule) checkPendingUpdates(ctx *core.ScanContext) *core.Finding {
	switch {
	case ctx.Platform.PackageManager == core.PkgApt:
		return m.checkPendingUpdatesApt()
	case ctx.Platform.PackageManager == core.PkgDnf:
		return m.checkPendingUpdatesDnf()
	case ctx.Platform.OS == core.OSDarwin:
		return m.checkPendingUpdatesDarwin()
	default:
		return nil
	}
}

// checkPendingUpdatesApt counts upgradable packages on Debian/Ubuntu.
func (m *UpdatesModule) checkPendingUpdatesApt() *core.Finding {
	out, err := exec.Command("apt", "list", "--upgradable").CombinedOutput()
	if err != nil {
		// Command not available or failed; skip gracefully.
		return nil
	}

	count := countUpgradableApt(string(out))
	if count == 0 {
		return nil
	}

	return &core.Finding{
		ID:            "upd-002",
		Module:        updatesModuleID,
		Severity:      core.SeverityMedium,
		TitleKey:      "finding.upd_pending_updates.title",
		DetailKey:     "finding.upd_pending_updates.detail",
		CurrentValue:  fmt.Sprintf("%d pending update(s)", count),
		ExpectedValue: "0 pending updates",
	}
}

// countUpgradableApt counts non-empty, non-header lines from `apt list --upgradable`.
// The first line is typically "Listing..." and should be skipped.
func countUpgradableApt(output string) int {
	count := 0
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "Listing") {
			continue
		}
		count++
	}
	return count
}

// checkPendingUpdatesDnf uses `dnf check-update --security` on RHEL/Fedora.
// Exit code 100 means updates are available; 0 means none.
func (m *UpdatesModule) checkPendingUpdatesDnf() *core.Finding {
	cmd := exec.Command("dnf", "check-update", "--security")
	out, err := cmd.CombinedOutput()

	if err != nil {
		// dnf check-update exits with code 100 when updates are available.
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 100 {
			count := countDnfUpdates(string(out))
			return &core.Finding{
				ID:            "upd-002",
				Module:        updatesModuleID,
				Severity:      core.SeverityMedium,
				TitleKey:      "finding.upd_pending_updates.title",
				DetailKey:     "finding.upd_pending_updates.detail",
				CurrentValue:  fmt.Sprintf("%d pending security update(s)", count),
				ExpectedValue: "0 pending security updates",
			}
		}
		// Any other error: skip gracefully.
		return nil
	}

	// Exit code 0: no updates available.
	return nil
}

// countDnfUpdates counts package lines from dnf check-update output.
// Lines with package updates have 3+ whitespace-separated fields (name, version, repo).
// There is a blank separator line before the package list.
func countDnfUpdates(output string) int {
	count := 0
	pastHeader := false
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			pastHeader = true
			continue
		}
		if !pastHeader {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) >= 3 {
			count++
		}
	}
	if count == 0 {
		count = 1 // At minimum, we know updates exist from exit code 100.
	}
	return count
}

// checkPendingUpdatesDarwin uses `softwareupdate -l` on macOS.
func (m *UpdatesModule) checkPendingUpdatesDarwin() *core.Finding {
	out, err := exec.Command("softwareupdate", "-l").CombinedOutput()
	if err != nil {
		return nil
	}

	output := string(out)

	// If the output contains "No new software available", there are no updates.
	if strings.Contains(output, "No new software available") {
		return nil
	}

	count := countMacOSUpdates(output)
	if count == 0 {
		return nil
	}

	return &core.Finding{
		ID:            "upd-002",
		Module:        updatesModuleID,
		Severity:      core.SeverityMedium,
		TitleKey:      "finding.upd_pending_updates.title",
		DetailKey:     "finding.upd_pending_updates.detail",
		CurrentValue:  fmt.Sprintf("%d pending update(s)", count),
		ExpectedValue: "0 pending updates",
	}
}

// countMacOSUpdates counts lines starting with "   * " which denote individual updates
// in `softwareupdate -l` output.
func countMacOSUpdates(output string) int {
	count := 0
	for _, line := range strings.Split(output, "\n") {
		if strings.HasPrefix(line, "   * ") {
			count++
		}
	}
	return count
}

// --- check: package manager cache staleness (upd-003) ---

func (m *UpdatesModule) checkCacheStaleness(ctx *core.ScanContext) *core.Finding {
	if ctx.Platform.PackageManager != core.PkgApt {
		return nil
	}

	return m.checkCacheStalenessApt()
}

// checkCacheStalenessApt checks the modification time of the apt package cache.
// A cache older than 7 days is considered stale.
func (m *UpdatesModule) checkCacheStalenessApt() *core.Finding {
	const cachePath = "/var/cache/apt/pkgcache.bin"
	const staleDays = 7

	info, err := os.Stat(cachePath)
	if err != nil {
		// Cache file does not exist or is not readable; skip gracefully.
		return nil
	}

	age := time.Since(info.ModTime())
	staleThreshold := time.Duration(staleDays) * 24 * time.Hour

	if age <= staleThreshold {
		return nil
	}

	ageDays := int(age.Hours() / 24)

	return &core.Finding{
		ID:            "upd-003",
		Module:        updatesModuleID,
		Severity:      core.SeverityLow,
		TitleKey:      "finding.upd_cache_stale.title",
		DetailKey:     "finding.upd_cache_stale.detail",
		CurrentValue:  fmt.Sprintf("Last updated %d day(s) ago", ageDays),
		ExpectedValue: fmt.Sprintf("Updated within the last %d days", staleDays),
	}
}
