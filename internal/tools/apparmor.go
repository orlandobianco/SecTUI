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
	actions := []core.QuickAction{
		{ID: "aa_full_status", Label: "Full status", Key: '1', Description: "Show complete AppArmor status"},
		{ID: "aa_profiles", Label: "List profiles", Key: '2', Description: "List loaded profiles"},
		{ID: "aa_reload", Label: "Reload", Key: '3', Description: "Reload AppArmor profiles"},
	}

	// Dynamic start/stop based on current service state.
	if serviceActive("apparmor") {
		actions = append(actions, core.QuickAction{
			ID: "aa_stop", Label: "Stop", Key: '4', Dangerous: true,
			Description: "Stop AppArmor — mandatory access control will be disabled",
		})
	} else {
		actions = append(actions, core.QuickAction{
			ID: "aa_start", Label: "Start", Key: '4', Dangerous: true,
			Description: "Start AppArmor service",
		})
	}

	return actions
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
		return actionOK(formatAAStatus(out))

	case "aa_profiles":
		out, err := runCmdSudo("aa-status", "--profiled")
		if err != nil {
			out, err = runCmdSudo("aa-status")
			if err != nil {
				return actionErr("aa-status: %v\n%s", err, out)
			}
		}
		return actionOK(formatAAProfiles(out))

	case "aa_reload":
		out, err := runCmdSudo("systemctl", "reload", "apparmor")
		if err != nil {
			return actionErr("reload failed: %v\n%s", err, out)
		}
		return actionOK("AppArmor profiles reloaded.")

	case "aa_start":
		out, err := runCmdSudo("systemctl", "start", "apparmor")
		if err != nil {
			return actionErr("start failed: %v\n%s", err, out)
		}
		return actionOK("AppArmor started successfully.")

	case "aa_stop":
		out, err := runCmdSudo("systemctl", "stop", "apparmor")
		if err != nil {
			return actionErr("stop failed: %v\n%s", err, out)
		}
		return actionOK("AppArmor stopped. Mandatory access control is now disabled.")

	default:
		return actionErr("unknown action: %s", actionID)
	}
}

func (t *AppArmorTool) RunScan() []core.Finding {
	return nil
}

// formatAAStatus restructures aa-status output into clearly labeled sections.
func formatAAStatus(raw string) string {
	var b strings.Builder
	lines := strings.Split(raw, "\n")

	var section string
	var items []string

	flushSection := func() {
		if section != "" {
			b.WriteString(section + "\n")
			for _, it := range items {
				b.WriteString("  " + it + "\n")
			}
			if len(items) == 0 {
				b.WriteString("  (none)\n")
			}
			b.WriteByte('\n')
		}
		section = ""
		items = nil
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		// Summary lines like "42 profiles are loaded."
		if strings.Contains(trimmed, " profiles are ") || strings.Contains(trimmed, " processes ") ||
			strings.HasPrefix(trimmed, "apparmor module") {

			flushSection()

			// Make it a section header with colon for the renderer.
			clean := strings.TrimSuffix(trimmed, ".")
			section = clean + ":"
			continue
		}

		// Indented items belong to the current section.
		if strings.HasPrefix(line, "   ") || strings.HasPrefix(line, "\t") {
			items = append(items, trimmed)
			continue
		}

		// Anything else: treat as its own line.
		flushSection()
		b.WriteString(trimmed + "\n")
	}
	flushSection()

	return strings.TrimSpace(b.String())
}

// formatAAProfiles formats the profile list output.
func formatAAProfiles(raw string) string {
	lines := strings.Split(raw, "\n")
	var profiles []string
	var counts []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		// Summary lines.
		if strings.Contains(trimmed, " profiles") || strings.Contains(trimmed, " processes") ||
			strings.HasPrefix(trimmed, "apparmor") {
			counts = append(counts, trimmed)
			continue
		}
		// Profile paths start with /.
		if strings.HasPrefix(trimmed, "/") {
			profiles = append(profiles, trimmed)
			continue
		}
		// Indented profile names.
		if strings.HasPrefix(line, "   ") || strings.HasPrefix(line, "\t") {
			profiles = append(profiles, trimmed)
			continue
		}
		profiles = append(profiles, trimmed)
	}

	var b strings.Builder
	if len(counts) > 0 {
		b.WriteString("Summary:\n")
		for _, c := range counts {
			b.WriteString("  " + c + "\n")
		}
		b.WriteByte('\n')
	}

	b.WriteString(fmt.Sprintf("Profiles (%d):\n", len(profiles)))
	for _, p := range profiles {
		b.WriteString("  " + p + "\n")
	}

	return strings.TrimSpace(b.String())
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
