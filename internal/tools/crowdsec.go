package tools

import (
	"fmt"
	"strings"

	"github.com/orlandobianco/SecTUI/internal/core"
)

type CrowdSecTool struct{}

func (t *CrowdSecTool) ID() string                  { return "crowdsec" }
func (t *CrowdSecTool) Name() string                { return "CrowdSec" }
func (t *CrowdSecTool) Description() string         { return core.T("tool.crowdsec.description") }
func (t *CrowdSecTool) Category() core.ToolCategory { return core.ToolCatIntrusionPrevention }

func (t *CrowdSecTool) Detect(_ *core.PlatformInfo) core.ToolStatus {
	return detectStatus("cscli", "crowdsec")
}

func (t *CrowdSecTool) InstallCommand(_ *core.PlatformInfo) string {
	return "curl -s https://install.crowdsec.net | sudo bash"
}

func (t *CrowdSecTool) IsApplicable(p *core.PlatformInfo) bool {
	return isLinux(p)
}

// --- ToolManager implementation ---

func (t *CrowdSecTool) ToolID() string { return "crowdsec" }

func (t *CrowdSecTool) GetServiceStatus() core.ServiceStatus {
	ss := serviceStatusInfo("crowdsec")

	if ver, err := runCmd("cscli", "version"); err == nil {
		for _, line := range strings.Split(ver, "\n") {
			line = strings.TrimSpace(line)
			if line != "" {
				ss.Extra["version"] = line
				break
			}
		}
	}
	return ss
}

func (t *CrowdSecTool) QuickActions() []core.QuickAction {
	actions := []core.QuickAction{
		{ID: "cs_decisions", Label: "Active decisions", Key: '1', Description: "Show active ban decisions"},
		{ID: "cs_alerts", Label: "Recent alerts", Key: '2', Description: "Show last 10 alerts"},
		{ID: "cs_hub_update", Label: "Update hub", Key: '3', Description: "Update CrowdSec hub (collections, parsers)"},
	}

	// Dynamic start/stop based on current service state.
	if serviceActive("crowdsec") {
		actions = append(actions, core.QuickAction{
			ID: "cs_stop", Label: "Stop", Key: '4', Dangerous: true,
			Description: "Stop CrowdSec — threat detection and IP blocking will be disabled",
		})
	} else {
		actions = append(actions, core.QuickAction{
			ID: "cs_start", Label: "Start", Key: '4', Dangerous: true,
			Description: "Start CrowdSec service",
		})
	}

	return actions
}

func (t *CrowdSecTool) ConfigSummary() []core.ConfigEntry {
	var entries []core.ConfigEntry

	if out, err := runCmdSudo("cscli", "collections", "list", "--no-color", "-o", "raw"); err == nil {
		n := countOutputLines(out)
		entries = append(entries, core.ConfigEntry{Key: "collections", Value: fmt.Sprintf("%d", n)})
	}

	if out, err := runCmdSudo("cscli", "bouncers", "list", "--no-color", "-o", "raw"); err == nil {
		n := countOutputLines(out)
		entries = append(entries, core.ConfigEntry{Key: "bouncers", Value: fmt.Sprintf("%d", n)})
	}

	if out, err := runCmdSudo("cscli", "parsers", "list", "--no-color", "-o", "raw"); err == nil {
		n := countOutputLines(out)
		entries = append(entries, core.ConfigEntry{Key: "parsers", Value: fmt.Sprintf("%d", n)})
	}

	if len(entries) == 0 {
		return []core.ConfigEntry{{Key: "config", Value: "unavailable"}}
	}
	return entries
}

func (t *CrowdSecTool) RecentActivity(n int) []core.ActivityEntry {
	return journalLines("crowdsec", n)
}

func (t *CrowdSecTool) ExecuteAction(actionID string) core.ActionResult {
	switch actionID {
	case "cs_decisions":
		out, err := runCmdSudo("cscli", "decisions", "list", "--no-color")
		if err != nil {
			return actionErr("cscli decisions list: %v\n%s", err, out)
		}
		if strings.TrimSpace(out) == "" {
			return actionOK("No active decisions.")
		}
		return actionOK(out)

	case "cs_alerts":
		out, err := runCmdSudo("cscli", "alerts", "list", "--no-color", "-l", "10")
		if err != nil {
			return actionErr("cscli alerts list: %v\n%s", err, out)
		}
		if strings.TrimSpace(out) == "" {
			return actionOK("No recent alerts.")
		}
		return actionOK(out)

	case "cs_hub_update":
		out, err := runCmdSudo("cscli", "hub", "update")
		if err != nil {
			return actionErr("cscli hub update: %v\n%s", err, out)
		}
		return actionOK(fmt.Sprintf("Hub updated.\n%s", out))

	case "cs_start":
		out, err := runCmdSudo("systemctl", "start", "crowdsec")
		if err != nil {
			return actionErr("start failed: %v\n%s", err, out)
		}
		return actionOK("CrowdSec started successfully.")

	case "cs_stop":
		out, err := runCmdSudo("systemctl", "stop", "crowdsec")
		if err != nil {
			return actionErr("stop failed: %v\n%s", err, out)
		}
		return actionOK("CrowdSec stopped. Threat detection and IP blocking are now disabled.")

	default:
		return actionErr("unknown action: %s", actionID)
	}
}

func (t *CrowdSecTool) RunScan() []core.Finding {
	return nil
}

// countOutputLines counts non-empty, non-header lines in raw cscli output.
func countOutputLines(out string) int {
	n := 0
	for _, line := range strings.Split(out, "\n") {
		if strings.TrimSpace(line) != "" {
			n++
		}
	}
	if n > 0 {
		n-- // subtract header row
	}
	return n
}
