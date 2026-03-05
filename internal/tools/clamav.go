package tools

import (
	"fmt"
	"strings"

	"github.com/orlandobianco/SecTUI/internal/core"
)

type ClamAVTool struct{}

func (t *ClamAVTool) ID() string                  { return "clamav" }
func (t *ClamAVTool) Name() string                { return "ClamAV" }
func (t *ClamAVTool) Description() string         { return core.T("tool.clamav.description") }
func (t *ClamAVTool) Category() core.ToolCategory { return core.ToolCatMalware }

func (t *ClamAVTool) Detect(p *core.PlatformInfo) core.ToolStatus {
	if !binaryExists("clamscan") {
		return core.ToolNotInstalled
	}
	if isDarwin(p) {
		return core.ToolInstalled
	}
	if serviceActive("clamav-daemon") || serviceActive("clamd") {
		return core.ToolActive
	}
	return core.ToolInstalled
}

func (t *ClamAVTool) InstallCommand(p *core.PlatformInfo) string {
	switch p.PackageManager {
	case core.PkgApt:
		return "sudo apt install -y clamav clamav-daemon"
	case core.PkgDnf:
		return "sudo dnf install -y clamav clamav-update clamd"
	case core.PkgPacman:
		return "sudo pacman -S --noconfirm clamav"
	case core.PkgBrew:
		return "brew install clamav"
	default:
		return ""
	}
}

func (t *ClamAVTool) IsApplicable(_ *core.PlatformInfo) bool {
	return true
}

// --- ToolManager implementation ---

func (t *ClamAVTool) ToolID() string { return "clamav" }

func (t *ClamAVTool) GetServiceStatus() core.ServiceStatus {
	ss := serviceStatusInfo("clamav-daemon")
	// Fallback to clamd service name.
	if !ss.Running {
		alt := serviceStatusInfo("clamd")
		if alt.Running {
			ss = alt
		}
	}

	if ver, err := runCmd("clamscan", "--version"); err == nil {
		ss.Extra["version"] = strings.TrimSpace(ver)
	}
	return ss
}

func (t *ClamAVTool) QuickActions() []core.QuickAction {
	actions := []core.QuickAction{
		{ID: "clam_scan_home", Label: "Scan /home", Key: '1', Description: "Recursive virus scan of /home"},
		{ID: "clam_scan_tmp", Label: "Scan /tmp", Key: '2', Description: "Recursive virus scan of /tmp"},
		{ID: "clam_update_db", Label: "Update DB", Key: '3', Description: "Update virus definition database"},
	}

	// Dynamic start/stop based on current service state.
	if serviceActive("clamav-daemon") || serviceActive("clamd") {
		actions = append(actions, core.QuickAction{
			ID: "clam_stop", Label: "Stop daemon", Key: '4', Dangerous: true,
			Description: "Stop ClamAV — real-time malware scanning will be disabled",
		})
	} else {
		actions = append(actions, core.QuickAction{
			ID: "clam_start", Label: "Start daemon", Key: '4', Dangerous: true,
			Description: "Start clamav-daemon service",
		})
	}

	return actions
}

func (t *ClamAVTool) ConfigSummary() []core.ConfigEntry {
	// Try to parse clamd.conf.
	for _, path := range []string{"/etc/clamav/clamd.conf", "/etc/clamd.d/scan.conf", "/etc/clamd.conf"} {
		content, err := readFile(path)
		if err != nil {
			continue
		}
		want := map[string]bool{
			"scanpe": true, "scanelf": true, "scanole2": true,
			"maxfilesize": true, "maxscansize": true,
		}
		var entries []core.ConfigEntry
		for _, e := range parseKeyValue(content, " ") {
			k := strings.ToLower(e.Key)
			if want[k] {
				entries = append(entries, core.ConfigEntry{Key: e.Key, Value: e.Value})
			}
		}
		if len(entries) > 0 {
			return entries
		}
	}
	return []core.ConfigEntry{{Key: "config", Value: "not found"}}
}

func (t *ClamAVTool) RecentActivity(n int) []core.ActivityEntry {
	entries := journalLines("clamav-daemon", n)
	if len(entries) == 0 {
		entries = journalLines("clamd", n)
	}
	return entries
}

func (t *ClamAVTool) ExecuteAction(actionID string) core.ActionResult {
	switch actionID {
	case "clam_scan_home":
		out, err := runCmdSudo("clamscan", "-r", "--no-summary", "/home")
		if err != nil {
			// clamscan returns exit code 1 when infected files found.
			if strings.Contains(out, "FOUND") {
				return actionOK(fmt.Sprintf("Scan complete (infected files found):\n%s", out))
			}
			return actionErr("clamscan /home: %v\n%s", err, out)
		}
		if strings.TrimSpace(out) == "" {
			return actionOK("Scan complete. No threats found in /home.")
		}
		return actionOK(out)

	case "clam_scan_tmp":
		out, err := runCmdSudo("clamscan", "-r", "--no-summary", "/tmp")
		if err != nil {
			if strings.Contains(out, "FOUND") {
				return actionOK(fmt.Sprintf("Scan complete (infected files found):\n%s", out))
			}
			return actionErr("clamscan /tmp: %v\n%s", err, out)
		}
		if strings.TrimSpace(out) == "" {
			return actionOK("Scan complete. No threats found in /tmp.")
		}
		return actionOK(out)

	case "clam_update_db":
		out, err := runCmdSudo("freshclam")
		if err != nil {
			return actionErr("freshclam: %v\n%s", err, out)
		}
		return actionOK(fmt.Sprintf("Virus DB updated.\n%s", out))

	case "clam_start":
		out, err := runCmdSudo("systemctl", "start", "clamav-daemon")
		if err != nil {
			out2, err2 := runCmdSudo("systemctl", "start", "clamd")
			if err2 != nil {
				return actionErr("start failed: %v\n%s\n%s", err, out, out2)
			}
			return actionOK("clamd started successfully.")
		}
		return actionOK("clamav-daemon started successfully.")

	case "clam_stop":
		out, err := runCmdSudo("systemctl", "stop", "clamav-daemon")
		if err != nil {
			out2, err2 := runCmdSudo("systemctl", "stop", "clamd")
			if err2 != nil {
				return actionErr("stop failed: %v\n%s\n%s", err, out, out2)
			}
			return actionOK("clamd stopped. Real-time malware scanning is now disabled.")
		}
		return actionOK("clamav-daemon stopped. Real-time malware scanning is now disabled.")

	default:
		return actionErr("unknown action: %s", actionID)
	}
}

func (t *ClamAVTool) RunScan() []core.Finding {
	out, err := runCmdSudo("clamscan", "-r", "--no-summary", "/home")
	if err == nil && strings.TrimSpace(out) == "" {
		return nil
	}
	// Parse infected files.
	var findings []core.Finding
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "FOUND") {
			findings = append(findings, core.Finding{
				ID:       "clamav_infected",
				Module:   "clamav",
				Severity: core.SeverityCritical,
				TitleKey: "finding.clamav.infected.title",
				DetailKey: fmt.Sprintf("Infected file: %s",
					strings.SplitN(line, ":", 2)[0]),
			})
		}
	}
	return findings
}
