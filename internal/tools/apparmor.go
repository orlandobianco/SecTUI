package tools

import (
	"fmt"
	"strings"

	"github.com/orlandobianco/SecTUI/internal/core"
)

type AppArmorTool struct{}

func (t *AppArmorTool) ID() string                  { return "apparmor" }
func (t *AppArmorTool) Name() string                { return "AppArmor" }
func (t *AppArmorTool) Description() string         { return core.T("tool.apparmor.description") }
func (t *AppArmorTool) Category() core.ToolCategory { return core.ToolCatAccessControl }

func (t *AppArmorTool) Detect(_ *core.PlatformInfo) core.ToolStatus {
	if !binaryExists("aa-status") && !binaryExists("apparmor_status") {
		return core.ToolNotInstalled
	}
	if serviceActive("apparmor") {
		return core.ToolActive
	}
	return core.ToolInstalled
}

func (t *AppArmorTool) InstallCommand(p *core.PlatformInfo) string {
	if p.PackageManager == core.PkgApt {
		return "sudo apt install -y apparmor apparmor-utils"
	}
	return installCmd("apparmor", p.PackageManager)
}

func (t *AppArmorTool) IsApplicable(p *core.PlatformInfo) bool {
	return isLinux(p) && isDebianBased(p)
}

// --- ToolManager implementation ---

func (t *AppArmorTool) ToolID() string { return "apparmor" }

func (t *AppArmorTool) GetServiceStatus() core.ServiceStatus {
	ss := serviceStatusInfo("apparmor")

	// Enrich with aa-status counts.
	if out, err := runCmdSudo("aa-status"); err == nil {
		for _, line := range strings.Split(out, "\n") {
			line = strings.TrimSpace(line)
			if strings.Contains(line, "profiles are loaded") {
				ss.Extra["profiles_loaded"] = extractLeadingNumber(line)
			} else if strings.Contains(line, "profiles are in enforce mode") {
				ss.Extra["enforce"] = extractLeadingNumber(line)
			} else if strings.Contains(line, "profiles are in complain mode") {
				ss.Extra["complain"] = extractLeadingNumber(line)
			} else if strings.Contains(line, "processes are unconfined") {
				ss.Extra["unconfined"] = extractLeadingNumber(line)
			}
		}
	}
	return ss
}

func (t *AppArmorTool) QuickActions() []core.QuickAction {
	return []core.QuickAction{
		{ID: "aa_full_status", Label: "Full status", Key: '1', Description: "Show complete AppArmor status"},
		{ID: "aa_profiles", Label: "List profiles", Key: '2', Description: "List loaded profiles"},
		{ID: "aa_reload", Label: "Reload", Key: '3', Description: "Reload AppArmor profiles"},
		{ID: "aa_restart", Label: "Restart", Key: '4', Dangerous: true, Description: "Restart AppArmor service"},
	}
}

func (t *AppArmorTool) ConfigSummary() []core.ConfigEntry {
	var entries []core.ConfigEntry

	n := countFiles("/etc/apparmor.d")
	entries = append(entries, core.ConfigEntry{Key: "profiles_dir", Value: "/etc/apparmor.d"})
	entries = append(entries, core.ConfigEntry{Key: "profile_files", Value: fmt.Sprintf("%d", n)})

	return entries
}

func (t *AppArmorTool) RecentActivity(n int) []core.ActivityEntry {
	return journalLinesGrep("apparmor", n, "-k")
}

func (t *AppArmorTool) ExecuteAction(actionID string) core.ActionResult {
	switch actionID {
	case "aa_full_status":
		out, err := runCmdSudo("aa-status")
		if err != nil {
			return actionErr("aa-status: %v\n%s", err, out)
		}
		return actionOK(out)

	case "aa_profiles":
		out, err := runCmdSudo("aa-status", "--profiled")
		if err != nil {
			// Some versions don't support --profiled; fallback to full status.
			out, err = runCmdSudo("aa-status")
			if err != nil {
				return actionErr("aa-status: %v\n%s", err, out)
			}
		}
		return actionOK(out)

	case "aa_reload":
		out, err := runCmdSudo("systemctl", "reload", "apparmor")
		if err != nil {
			return actionErr("reload failed: %v\n%s", err, out)
		}
		return actionOK("AppArmor profiles reloaded.")

	case "aa_restart":
		out, err := runCmdSudo("systemctl", "restart", "apparmor")
		if err != nil {
			return actionErr("restart failed: %v\n%s", err, out)
		}
		return actionOK("AppArmor restarted successfully.")

	default:
		return actionErr("unknown action: %s", actionID)
	}
}

func (t *AppArmorTool) RunScan() []core.Finding {
	return nil
}

// extractLeadingNumber extracts the first number from a string like "42 profiles are loaded".
func extractLeadingNumber(s string) string {
	s = strings.TrimSpace(s)
	var num []byte
	for _, c := range []byte(s) {
		if c >= '0' && c <= '9' {
			num = append(num, c)
		} else if len(num) > 0 {
			break
		}
	}
	if len(num) == 0 {
		return "0"
	}
	return string(num)
}
