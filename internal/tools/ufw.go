package tools

import (
	"fmt"
	"strings"

	"github.com/orlandobianco/SecTUI/internal/core"
)

type UFWTool struct{}

func (t *UFWTool) ID() string                  { return "ufw" }
func (t *UFWTool) Name() string                { return "UFW Firewall" }
func (t *UFWTool) Description() string         { return core.T("tool.ufw.description") }
func (t *UFWTool) Category() core.ToolCategory { return core.ToolCatFirewall }

func (t *UFWTool) Detect(_ *core.PlatformInfo) core.ToolStatus {
	return detectStatus("ufw", "ufw")
}

func (t *UFWTool) InstallCommand(p *core.PlatformInfo) string {
	return installCmd("ufw", p.PackageManager)
}

func (t *UFWTool) IsApplicable(p *core.PlatformInfo) bool {
	return isLinux(p) && isDebianBased(p)
}

// --- ToolManager implementation ---

func (t *UFWTool) ToolID() string { return "ufw" }

func (t *UFWTool) GetServiceStatus() core.ServiceStatus {
	ss := serviceStatusInfo("ufw")

	// UFW has its own status command that's more useful than systemd's.
	out, err := runCmdSudo("ufw", "status", "verbose")
	if err == nil {
		for _, line := range strings.Split(out, "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "Status:") {
				val := strings.TrimSpace(strings.TrimPrefix(line, "Status:"))
				ss.Extra["status"] = val
				ss.Running = val == "active"
			}
			if strings.HasPrefix(line, "Default:") {
				ss.Extra["defaults"] = strings.TrimSpace(strings.TrimPrefix(line, "Default:"))
			}
			if strings.HasPrefix(line, "Logging:") {
				ss.Extra["logging"] = strings.TrimSpace(strings.TrimPrefix(line, "Logging:"))
			}
		}

		// Count rules.
		rules := parseUFWRules(out)
		ss.Extra["rules"] = fmt.Sprintf("%d", len(rules))
	}

	// Get version.
	if ver, err := runCmd("ufw", "version"); err == nil {
		// "ufw 0.36.2" → extract version.
		parts := strings.Fields(ver)
		for _, p := range parts {
			if len(p) > 0 && p[0] >= '0' && p[0] <= '9' {
				ss.Extra["version"] = p
				break
			}
		}
	}

	return ss
}

func (t *UFWTool) QuickActions() []core.QuickAction {
	actions := []core.QuickAction{
		{ID: "ufw_rules", Label: "Show rules", Key: '1', Description: "Display all active firewall rules"},
	}

	if isUFWActive() {
		actions = append(actions, core.QuickAction{
			ID: "ufw_disable", Label: "Disable", Key: '2', Dangerous: true,
			Description: "Disable firewall — all traffic will be allowed",
		})
	} else {
		actions = append(actions, core.QuickAction{
			ID: "ufw_enable", Label: "Enable", Key: '2', Dangerous: true,
			Description: "Enable firewall with current rules",
		})
	}

	actions = append(actions,
		core.QuickAction{
			ID: "ufw_allow_ssh", Label: "Allow SSH", Key: '3',
			Description: "Allow incoming SSH (port 22/tcp)",
		},
		core.QuickAction{
			ID: "ufw_reload", Label: "Reload", Key: '4', Dangerous: true,
			Description: "Reload rules and restart UFW",
		},
	)

	return actions
}

func (t *UFWTool) ConfigSummary() []core.ConfigEntry {
	out, err := runCmdSudo("ufw", "status", "verbose")
	if err != nil {
		return []core.ConfigEntry{{Key: "status", Value: "unavailable"}}
	}

	var entries []core.ConfigEntry

	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "Default:"):
			val := strings.TrimSpace(strings.TrimPrefix(line, "Default:"))
			// "deny (incoming), allow (outgoing), deny (routed)"
			for _, part := range strings.Split(val, ",") {
				part = strings.TrimSpace(part)
				if strings.Contains(part, "incoming") {
					entries = append(entries, core.ConfigEntry{Key: "incoming", Value: cleanPolicyValue(part)})
				} else if strings.Contains(part, "outgoing") {
					entries = append(entries, core.ConfigEntry{Key: "outgoing", Value: cleanPolicyValue(part)})
				} else if strings.Contains(part, "routed") {
					entries = append(entries, core.ConfigEntry{Key: "routed", Value: cleanPolicyValue(part)})
				}
			}
		case strings.HasPrefix(line, "Logging:"):
			val := strings.TrimSpace(strings.TrimPrefix(line, "Logging:"))
			entries = append(entries, core.ConfigEntry{Key: "logging", Value: val})
		}
	}

	if len(entries) == 0 {
		return []core.ConfigEntry{{Key: "config", Value: "not found"}}
	}
	return entries
}

func (t *UFWTool) RecentActivity(n int) []core.ActivityEntry {
	// UFW logs to kernel via iptables LOG target, grep for [UFW in journal.
	entries := journalLinesGrep("UFW", n, "-k")
	if len(entries) > 0 {
		return entries
	}
	// Fallback: check ufw service journal.
	return journalLines("ufw", n)
}

func (t *UFWTool) ExecuteAction(actionID string) core.ActionResult {
	switch actionID {
	case "ufw_rules":
		out, err := runCmdSudo("ufw", "status", "numbered")
		if err != nil {
			return actionErr("ufw status numbered: %v\n%s", err, out)
		}
		return actionOK(formatUFWRules(out))

	case "ufw_enable":
		out, err := runCmdSudo("ufw", "--force", "enable")
		if err != nil {
			return actionErr("ufw enable: %v\n%s", err, out)
		}
		return actionOK("Firewall enabled.\n\n" + formatPostEnable())

	case "ufw_disable":
		out, err := runCmdSudo("ufw", "disable")
		if err != nil {
			return actionErr("ufw disable: %v\n%s", err, out)
		}
		return actionOK("Firewall disabled.\n\nAll incoming traffic is now allowed. Enable the firewall as soon as possible.")

	case "ufw_allow_ssh":
		out, err := runCmdSudo("ufw", "allow", "ssh")
		if err != nil {
			return actionErr("ufw allow ssh: %v\n%s", err, out)
		}
		return actionOK(formatRuleAdded("SSH (22/tcp)", out))

	case "ufw_reload":
		out, err := runCmdSudo("ufw", "reload")
		if err != nil {
			return actionErr("ufw reload: %v\n%s", err, out)
		}
		return actionOK("Firewall reloaded.\n\n" + strings.TrimSpace(out))

	default:
		return actionErr("unknown action: %s", actionID)
	}
}

func (t *UFWTool) RunScan() []core.Finding {
	if !isUFWActive() {
		return []core.Finding{{
			ID:       "ufw_inactive",
			Module:   "ufw",
			Severity: core.SeverityCritical,
			TitleKey: "finding.ufw.inactive.title",
			DetailKey: "UFW firewall is not active. Your system is exposed to " +
				"all incoming network traffic without filtering.",
		}}
	}

	// Check if SSH is allowed.
	out, _ := runCmdSudo("ufw", "status")
	if !strings.Contains(out, "22") && !strings.Contains(strings.ToLower(out), "ssh") {
		return []core.Finding{{
			ID:        "ufw_no_ssh_rule",
			Module:    "ufw",
			Severity:  core.SeverityMedium,
			TitleKey:  "finding.ufw.no_ssh.title",
			DetailKey: "No SSH rule found in UFW. If you connect via SSH, you may get locked out.",
		}}
	}

	return nil
}

// --- Helpers ---

func isUFWActive() bool {
	out, err := runCmdSudo("ufw", "status")
	if err != nil {
		return false
	}
	return strings.Contains(out, "Status: active")
}

// ufwRule represents a single parsed UFW rule.
type ufwRule struct {
	num    string
	to     string
	action string
	from   string
	note   string // (v6), comment, etc.
}

// parseUFWRules parses "ufw status numbered" or "ufw status verbose" output.
func parseUFWRules(raw string) []ufwRule {
	var rules []ufwRule
	inRules := false

	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)

		// The rules start after "---" line.
		if strings.HasPrefix(line, "---") {
			inRules = true
			continue
		}
		if !inRules || line == "" {
			continue
		}

		// Numbered format: "[ 1] 22/tcp    ALLOW IN    Anywhere"
		// Verbose format:  "22/tcp    ALLOW IN    Anywhere"
		rule := parseUFWRuleLine(line)
		if rule.to != "" {
			rules = append(rules, rule)
		}
	}
	return rules
}

func parseUFWRuleLine(line string) ufwRule {
	r := ufwRule{}

	// Strip rule number: "[1]", "[ 1]", "[12]"
	if strings.HasPrefix(line, "[") {
		if idx := strings.Index(line, "]"); idx >= 0 {
			r.num = strings.TrimSpace(line[1:idx])
			line = strings.TrimSpace(line[idx+1:])
		}
	}

	// Split fields. UFW output is column-aligned.
	// Typical: "22/tcp                     ALLOW IN    Anywhere"
	// Or:      "Anywhere                   DENY IN     10.0.0.0/8"

	// Find ALLOW/DENY/REJECT/LIMIT.
	actions := []string{"ALLOW IN", "DENY IN", "REJECT IN", "LIMIT IN",
		"ALLOW OUT", "DENY OUT", "REJECT OUT", "LIMIT OUT",
		"ALLOW FWD", "DENY FWD", "ALLOW", "DENY", "REJECT", "LIMIT"}

	for _, a := range actions {
		idx := strings.Index(strings.ToUpper(line), a)
		if idx < 0 {
			continue
		}
		r.to = strings.TrimSpace(line[:idx])
		r.action = a
		rest := strings.TrimSpace(line[idx+len(a):])
		// Rest is "from" potentially with comments.
		if commentIdx := strings.Index(rest, "#"); commentIdx >= 0 {
			r.note = strings.TrimSpace(rest[commentIdx+1:])
			rest = strings.TrimSpace(rest[:commentIdx])
		}
		// Check for (v6) marker.
		if strings.HasSuffix(rest, "(v6)") {
			r.note = "(v6)"
			rest = strings.TrimSpace(strings.TrimSuffix(rest, "(v6)"))
		}
		r.from = rest
		break
	}

	return r
}

func formatUFWRules(raw string) string {
	rules := parseUFWRules(raw)

	var b strings.Builder
	b.WriteString("Firewall Rules:\n\n")

	if len(rules) == 0 {
		out, _ := runCmdSudo("ufw", "status")
		if strings.Contains(out, "inactive") {
			b.WriteString("  Firewall is inactive. No rules are being enforced.\n")
			b.WriteString("  Use [2] Enable to activate the firewall.\n")
		} else {
			b.WriteString("  No rules configured.\n")
			b.WriteString("  Default policy applies to all traffic.\n")
		}
		return b.String()
	}

	// Table header.
	b.WriteString(fmt.Sprintf("  %-4s %-22s %-12s %-20s %s\n",
		"#", "To", "Action", "From", "Note"))
	b.WriteString(fmt.Sprintf("  %s\n",
		strings.Repeat("─", 70)))

	for _, r := range rules {
		num := r.num
		if num == "" {
			num = "-"
		}
		to := r.to
		if len(to) > 22 {
			to = to[:21] + "…"
		}
		from := r.from
		if len(from) > 20 {
			from = from[:19] + "…"
		}
		note := r.note
		if len(note) > 15 {
			note = note[:14] + "…"
		}

		b.WriteString(fmt.Sprintf("  %-4s %-22s %-12s %-20s %s\n",
			num, to, r.action, from, note))
	}

	b.WriteString(fmt.Sprintf("\n  Total: %d rules\n", len(rules)))

	return b.String()
}

func formatPostEnable() string {
	// Show current rules after enabling.
	out, err := runCmdSudo("ufw", "status", "verbose")
	if err != nil {
		return "Firewall is now active."
	}

	var b strings.Builder
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		b.WriteString("  " + line + "\n")
	}
	return b.String()
}

func formatRuleAdded(ruleName, raw string) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Rule added: %s\n\n", ruleName))

	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.Contains(line, "existing") || strings.Contains(line, "skipping") {
			b.WriteString("  ⓘ " + line + "\n")
		} else if strings.Contains(line, "added") || strings.Contains(line, "updated") {
			b.WriteString("  ✓ " + line + "\n")
		} else {
			b.WriteString("  " + line + "\n")
		}
	}

	return b.String()
}

// cleanPolicyValue extracts the policy from "deny (incoming)" → "deny".
func cleanPolicyValue(s string) string {
	s = strings.TrimSpace(s)
	if idx := strings.Index(s, "("); idx > 0 {
		return strings.TrimSpace(s[:idx])
	}
	return s
}
