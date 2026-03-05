package tools

import (
	"fmt"
	"os"
	"strings"

	"github.com/orlandobianco/SecTUI/internal/core"
)

type Fail2banTool struct{}

func (t *Fail2banTool) ID() string                  { return "fail2ban" }
func (t *Fail2banTool) Name() string                { return "fail2ban" }
func (t *Fail2banTool) Description() string         { return core.T("tool.fail2ban.description") }
func (t *Fail2banTool) Category() core.ToolCategory { return core.ToolCatIntrusionPrevention }

func (t *Fail2banTool) Detect(_ *core.PlatformInfo) core.ToolStatus {
	return detectStatus("fail2ban-client", "fail2ban")
}

func (t *Fail2banTool) InstallCommand(p *core.PlatformInfo) string {
	return installCmd("fail2ban", p.PackageManager)
}

func (t *Fail2banTool) IsApplicable(p *core.PlatformInfo) bool {
	return isLinux(p)
}

// --- ToolManager implementation ---

func (t *Fail2banTool) ToolID() string { return "fail2ban" }

func (t *Fail2banTool) GetServiceStatus() core.ServiceStatus {
	ss := serviceStatusInfo("fail2ban")

	// Enrich with version and jail count.
	if ver, err := runCmd("fail2ban-client", "--version"); err == nil {
		ss.Extra["version"] = strings.TrimSpace(ver)
	}
	if out, err := runCmdSudo("fail2ban-client", "status"); err == nil {
		ss.Extra["status_output"] = out
		// Parse jail count from "Number of jail: N"
		for _, line := range strings.Split(out, "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "Number of jail") {
				parts := strings.SplitN(line, ":", 2)
				if len(parts) == 2 {
					ss.Extra["jails"] = strings.TrimSpace(parts[1])
				}
			}
			if strings.HasPrefix(line, "Jail list") {
				parts := strings.SplitN(line, ":", 2)
				if len(parts) == 2 {
					ss.Extra["jail_list"] = strings.TrimSpace(parts[1])
				}
			}
		}
	}
	return ss
}

func (t *Fail2banTool) QuickActions() []core.QuickAction {
	actions := []core.QuickAction{
		{ID: "f2b_ssh_status", Label: "SSH jail status", Key: '1', Description: "Show fail2ban SSH jail details"},
		{ID: "f2b_banned", Label: "Banned IPs", Key: '2', Description: "List all currently banned IPs"},
	}

	// Dynamic start/stop based on current service state.
	if serviceActive("fail2ban") {
		actions = append(actions, core.QuickAction{
			ID: "f2b_stop", Label: "Stop", Key: '3', Dangerous: true,
			Description: "Stop fail2ban — brute-force protection will be disabled",
		})
	} else {
		actions = append(actions, core.QuickAction{
			ID: "f2b_start", Label: "Start", Key: '3', Dangerous: true,
			Description: "Start fail2ban service",
		})
	}

	actions = append(actions, core.QuickAction{
		ID: "f2b_unban_all", Label: "Unban all", Key: '4', Dangerous: true,
		Description: "Remove all IP bans",
	})

	return actions
}

func (t *Fail2banTool) ConfigSummary() []core.ConfigEntry {
	// Try jail.local first, then jail.conf.
	content := ""
	for _, path := range []string{"/etc/fail2ban/jail.local", "/etc/fail2ban/jail.conf"} {
		if _, err := os.Stat(path); err == nil {
			if c, err := readFile(path); err == nil {
				content = c
				break
			}
		}
	}
	if content == "" {
		return []core.ConfigEntry{{Key: "config", Value: "not found"}}
	}

	// Extract key values from [DEFAULT] section.
	want := map[string]bool{"bantime": true, "findtime": true, "maxretry": true, "ignoreip": true}
	var entries []core.ConfigEntry
	for _, e := range parseKeyValue(content, "=") {
		k := strings.ToLower(strings.TrimSpace(e.Key))
		if want[k] {
			entries = append(entries, core.ConfigEntry{Key: k, Value: e.Value})
		}
	}
	if len(entries) == 0 {
		return []core.ConfigEntry{{Key: "config", Value: "no matching keys found"}}
	}
	return entries
}

func (t *Fail2banTool) RecentActivity(n int) []core.ActivityEntry {
	return journalLines("fail2ban", n)
}

func (t *Fail2banTool) ExecuteAction(actionID string) core.ActionResult {
	switch actionID {
	case "f2b_ssh_status":
		out, err := runCmdSudo("fail2ban-client", "status", "sshd")
		if err != nil {
			return actionErr("fail2ban-client status sshd: %v\n%s", err, out)
		}
		return actionOK(out)

	case "f2b_banned":
		out, err := runCmdSudo("fail2ban-client", "banned")
		if err != nil {
			return actionErr("fail2ban-client banned: %v\n%s", err, out)
		}
		if strings.TrimSpace(out) == "" || strings.TrimSpace(out) == "[]" {
			return actionOK("No banned IPs.")
		}
		return actionOK(out)

	case "f2b_start":
		out, err := runCmdSudo("systemctl", "start", "fail2ban")
		if err != nil {
			return actionErr("start failed: %v\n%s", err, out)
		}
		return actionOK("fail2ban started successfully.")

	case "f2b_stop":
		out, err := runCmdSudo("systemctl", "stop", "fail2ban")
		if err != nil {
			return actionErr("stop failed: %v\n%s", err, out)
		}
		return actionOK("fail2ban stopped. Brute-force protection is now disabled.")

	case "f2b_unban_all":
		out, err := runCmdSudo("fail2ban-client", "unban", "--all")
		if err != nil {
			return actionErr("unban failed: %v\n%s", err, out)
		}
		return actionOK(fmt.Sprintf("All IPs unbanned.\n%s", out))

	default:
		return actionErr("unknown action: %s", actionID)
	}
}

func (t *Fail2banTool) RunScan() []core.Finding {
	return nil
}
