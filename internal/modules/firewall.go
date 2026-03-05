package modules

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/orlandobianco/SecTUI/internal/core"
)

const firewallModuleID = "firewall"

type firewallBackend struct {
	name    string
	detect  func() (active bool, err error)
	osScope core.OS
}

type FirewallModule struct {
	backends []firewallBackend
}

func NewFirewallModule() *FirewallModule {
	return &FirewallModule{
		backends: []firewallBackend{
			{name: "ufw", detect: detectUFW, osScope: core.OSLinux},
			{name: "iptables", detect: detectIptables, osScope: core.OSLinux},
			{name: "nftables", detect: detectNftables, osScope: core.OSLinux},
			{name: "pf", detect: detectPF, osScope: core.OSDarwin},
		},
	}
}

func (m *FirewallModule) ID() string            { return firewallModuleID }
func (m *FirewallModule) NameKey() string        { return "module.firewall.name" }
func (m *FirewallModule) DescriptionKey() string { return "module.firewall.description" }
func (m *FirewallModule) Priority() int          { return 20 }
func (m *FirewallModule) IsApplicable(_ *core.PlatformInfo) bool { return true }

func (m *FirewallModule) Scan(ctx *core.ScanContext) []core.Finding {
	var activeBackends []string

	for _, b := range m.backends {
		if b.osScope != ctx.Platform.OS {
			continue
		}
		active, err := b.detect()
		if err != nil {
			continue
		}
		if active {
			activeBackends = append(activeBackends, b.name)
		}
	}

	if len(activeBackends) == 0 {
		return []core.Finding{{
			ID:            "fw-001",
			Module:        firewallModuleID,
			Severity:      core.SeverityCritical,
			TitleKey:      "finding.fw_no_firewall.title",
			DetailKey:     "finding.fw_no_firewall.detail",
			FixID:         "fix-fw-001",
			CurrentValue:  "none",
			ExpectedValue: "active",
		}}
	}

	return nil
}

func (m *FirewallModule) AvailableFixes() []core.Fix {
	return []core.Fix{
		{
			ID:          "fix-fw-001",
			FindingID:   "fw-001",
			TitleKey:    "fix.fw_enable_ufw.title",
			Description: "Enable UFW with default deny incoming and allow SSH",
			Dangerous:   true,
		},
	}
}

func (m *FirewallModule) PreviewFix(fixID string, ctx *core.ScanContext) (string, error) {
	if fixID != "fix-fw-001" {
		return "", fmt.Errorf("unknown fix: %s", fixID)
	}

	if ctx.Platform.OS != core.OSLinux {
		return "", fmt.Errorf("UFW fix is only available on Linux")
	}

	var b strings.Builder
	b.WriteString("The following commands will be executed:\n")
	b.WriteString("  ufw default deny incoming\n")
	b.WriteString("  ufw default allow outgoing\n")
	b.WriteString("  ufw allow ssh\n")
	b.WriteString("  ufw --force enable\n")
	return b.String(), nil
}

func (m *FirewallModule) ApplyFix(fixID string, ctx *core.ApplyContext) (*core.ApplyResult, error) {
	if fixID != "fix-fw-001" {
		return nil, fmt.Errorf("unknown fix: %s", fixID)
	}

	if ctx.Platform.OS != core.OSLinux {
		return nil, fmt.Errorf("UFW fix is only available on Linux")
	}

	if ctx.DryRun {
		return &core.ApplyResult{
			Success: true,
			Message: "[dry-run] Would enable UFW with default deny incoming and allow SSH",
		}, nil
	}

	commands := [][]string{
		{"ufw", "default", "deny", "incoming"},
		{"ufw", "default", "allow", "outgoing"},
		{"ufw", "allow", "ssh"},
		{"ufw", "--force", "enable"},
	}

	for _, args := range commands {
		cmd := exec.Command(args[0], args[1:]...)
		if out, err := cmd.CombinedOutput(); err != nil {
			return &core.ApplyResult{
				Success: false,
				Message: fmt.Sprintf("Failed running %s: %s (%s)", strings.Join(args, " "), err, strings.TrimSpace(string(out))),
			}, nil
		}
	}

	return &core.ApplyResult{
		Success: true,
		Message: "UFW enabled with default deny incoming, allow outgoing, and SSH allowed",
	}, nil
}

// --- backend detection ---

func detectUFW() (bool, error) {
	if _, err := exec.LookPath("ufw"); err != nil {
		return false, nil
	}

	out, err := exec.Command("ufw", "status").CombinedOutput()
	if err != nil {
		return false, err
	}

	return strings.Contains(string(out), "Status: active"), nil
}

func detectIptables() (bool, error) {
	if _, err := exec.LookPath("iptables"); err != nil {
		return false, nil
	}

	out, err := exec.Command("iptables", "-L", "-n").CombinedOutput()
	if err != nil {
		return false, err
	}

	// A default-policy-only iptables has very few lines. Look for user-added rules
	// beyond the three default chains with their ACCEPT/DROP policies.
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	ruleLines := 0
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "Chain ") || strings.HasPrefix(line, "target") {
			continue
		}
		ruleLines++
	}

	return ruleLines > 0, nil
}

func detectNftables() (bool, error) {
	if _, err := exec.LookPath("nft"); err != nil {
		return false, nil
	}

	out, err := exec.Command("nft", "list", "ruleset").CombinedOutput()
	if err != nil {
		return false, err
	}

	content := strings.TrimSpace(string(out))
	return content != "" && strings.Contains(content, "table"), nil
}

func detectPF() (bool, error) {
	if _, err := exec.LookPath("pfctl"); err != nil {
		return false, nil
	}

	out, err := exec.Command("pfctl", "-s", "info").CombinedOutput()
	if err != nil {
		// pfctl often returns exit code 1 even when providing info; check output.
		if len(out) == 0 {
			return false, err
		}
	}

	return strings.Contains(string(out), "Status: Enabled"), nil
}
