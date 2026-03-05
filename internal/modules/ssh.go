package modules

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/orlandobianco/SecTUI/internal/core"
)

const (
	sshdConfigPath = "/etc/ssh/sshd_config"
	sshModuleID    = "ssh"
)

type sshCheck struct {
	id           string
	key          string
	expected     string
	defaultValue string
	severity     core.Severity
	titleKey     string
	detailKey    string
	fixID        string
	compareFunc  func(actual, expected string) bool
}

type SSHModule struct {
	checks []sshCheck
}

func NewSSHModule() *SSHModule {
	return &SSHModule{
		checks: []sshCheck{
			{
				id:           "ssh-001",
				key:          "PermitRootLogin",
				expected:     "no",
				defaultValue: "prohibit-password",
				severity:     core.SeverityCritical,
				titleKey:     "finding.ssh_root_login.title",
				detailKey:    "finding.ssh_root_login.detail",
				fixID:        "fix-ssh-001",
				compareFunc:  sshExpectExact,
			},
			{
				id:           "ssh-002",
				key:          "PasswordAuthentication",
				expected:     "no",
				defaultValue: "yes",
				severity:     core.SeverityCritical,
				titleKey:     "finding.ssh_password_auth.title",
				detailKey:    "finding.ssh_password_auth.detail",
				fixID:        "fix-ssh-002",
				compareFunc:  sshExpectExact,
			},
			{
				id:           "ssh-003",
				key:          "PermitEmptyPasswords",
				expected:     "no",
				defaultValue: "no",
				severity:     core.SeverityCritical,
				titleKey:     "finding.ssh_empty_passwords.title",
				detailKey:    "finding.ssh_empty_passwords.detail",
				fixID:        "fix-ssh-003",
				compareFunc:  sshExpectExact,
			},
			{
				id:           "ssh-004",
				key:          "PubkeyAuthentication",
				expected:     "yes",
				defaultValue: "yes",
				severity:     core.SeverityHigh,
				titleKey:     "finding.ssh_pubkey_auth.title",
				detailKey:    "finding.ssh_pubkey_auth.detail",
				fixID:        "fix-ssh-004",
				compareFunc:  sshExpectExact,
			},
			{
				id:           "ssh-005",
				key:          "MaxAuthTries",
				expected:     "3",
				defaultValue: "6",
				severity:     core.SeverityMedium,
				titleKey:     "finding.ssh_max_auth_tries.title",
				detailKey:    "finding.ssh_max_auth_tries.detail",
				fixID:        "fix-ssh-005",
				compareFunc:  sshExpectLessOrEqual,
			},
			{
				id:           "ssh-006",
				key:          "X11Forwarding",
				expected:     "no",
				defaultValue: "no",
				severity:     core.SeverityLow,
				titleKey:     "finding.ssh_x11_forwarding.title",
				detailKey:    "finding.ssh_x11_forwarding.detail",
				fixID:        "fix-ssh-006",
				compareFunc:  sshExpectExact,
			},
			{
				id:           "ssh-007",
				key:          "LoginGraceTime",
				expected:     "30",
				defaultValue: "120",
				severity:     core.SeverityLow,
				titleKey:     "finding.ssh_login_grace_time.title",
				detailKey:    "finding.ssh_login_grace_time.detail",
				fixID:        "fix-ssh-007",
				compareFunc:  sshExpectLessOrEqual,
			},
		},
	}
}

func (m *SSHModule) ID() string             { return sshModuleID }
func (m *SSHModule) NameKey() string        { return "module.ssh.name" }
func (m *SSHModule) DescriptionKey() string { return "module.ssh.description" }
func (m *SSHModule) Priority() int          { return 10 }

func (m *SSHModule) IsApplicable(_ *core.PlatformInfo) bool {
	_, err := os.Stat(sshdConfigPath)
	return err == nil
}

func (m *SSHModule) Scan(_ *core.ScanContext) []core.Finding {
	settings, err := parseSSHDConfig(sshdConfigPath)
	if err != nil {
		return []core.Finding{{
			ID:        "ssh-000",
			Module:    sshModuleID,
			Severity:  core.SeverityInfo,
			TitleKey:  "finding.ssh_config_unreadable.title",
			DetailKey: "finding.ssh_config_unreadable.detail",
		}}
	}

	var findings []core.Finding
	for _, c := range m.checks {
		actual, found := settings[strings.ToLower(c.key)]
		if !found {
			actual = c.defaultValue
		}

		if !c.compareFunc(actual, c.expected) {
			findings = append(findings, core.Finding{
				ID:            c.id,
				Module:        sshModuleID,
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

func (m *SSHModule) AvailableFixes() []core.Fix {
	fixes := make([]core.Fix, len(m.checks))
	for i, c := range m.checks {
		fixes[i] = core.Fix{
			ID:          c.fixID,
			FindingID:   c.id,
			TitleKey:    c.titleKey,
			Description: fmt.Sprintf("Set %s to %s in %s", c.key, c.expected, sshdConfigPath),
			Dangerous:   c.severity == core.SeverityCritical,
		}
	}
	return fixes
}

func (m *SSHModule) PreviewFix(fixID string, _ *core.ScanContext) (string, error) {
	check := m.findCheckByFixID(fixID)
	if check == nil {
		return "", fmt.Errorf("unknown fix: %s", fixID)
	}

	settings, err := parseSSHDConfig(sshdConfigPath)
	if err != nil {
		return "", fmt.Errorf("cannot read sshd_config: %w", err)
	}

	current, found := settings[strings.ToLower(check.key)]
	if !found {
		current = check.defaultValue
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("--- %s\n", sshdConfigPath))
	b.WriteString(fmt.Sprintf("+++ %s (modified)\n", sshdConfigPath))
	b.WriteString(fmt.Sprintf("- %s %s\n", check.key, current))
	b.WriteString(fmt.Sprintf("+ %s %s\n", check.key, check.expected))
	return b.String(), nil
}

func (m *SSHModule) ApplyFix(fixID string, ctx *core.ApplyContext) (*core.ApplyResult, error) {
	check := m.findCheckByFixID(fixID)
	if check == nil {
		return nil, fmt.Errorf("unknown fix: %s", fixID)
	}

	data, err := os.ReadFile(sshdConfigPath)
	if err != nil {
		return nil, fmt.Errorf("cannot read sshd_config: %w", err)
	}

	newContent := setSSHDValue(string(data), check.key, check.expected)

	if ctx.DryRun {
		return &core.ApplyResult{
			Success: true,
			Message: fmt.Sprintf("[dry-run] Would set %s to %s", check.key, check.expected),
		}, nil
	}

	if ctx.Backup {
		backupPath := sshdConfigPath + ".bak"
		if err := os.WriteFile(backupPath, data, 0o600); err != nil {
			return nil, fmt.Errorf("failed to create backup: %w", err)
		}
	}

	if err := os.WriteFile(sshdConfigPath, []byte(newContent), 0o600); err != nil {
		return nil, fmt.Errorf("failed to write sshd_config: %w", err)
	}

	backupInfo := ""
	if ctx.Backup {
		backupInfo = sshdConfigPath + ".bak"
	}

	return &core.ApplyResult{
		Success:    true,
		Message:    fmt.Sprintf("Set %s to %s", check.key, check.expected),
		BackupPath: backupInfo,
	}, nil
}

func (m *SSHModule) findCheckByFixID(fixID string) *sshCheck {
	for i := range m.checks {
		if m.checks[i].fixID == fixID {
			return &m.checks[i]
		}
	}
	return nil
}

// parseSSHDConfig reads an sshd_config file and returns a map of lowercase keys to values.
func parseSSHDConfig(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	settings := make(map[string]string)
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Handle both "Key Value" and "Key=Value" formats
		var key, value string
		if idx := strings.IndexByte(line, '='); idx != -1 {
			key = strings.TrimSpace(line[:idx])
			value = strings.TrimSpace(line[idx+1:])
		} else if fields := strings.Fields(line); len(fields) >= 2 {
			key = fields[0]
			value = fields[1]
		} else {
			continue
		}

		settings[strings.ToLower(key)] = value
	}

	return settings, nil
}

// setSSHDValue returns the config content with the given key set to the new value.
// If the key exists (commented or not), the line is replaced. Otherwise the value is appended.
func setSSHDValue(content, key, value string) string {
	lines := strings.Split(content, "\n")
	lowerKey := strings.ToLower(key)
	replaced := false

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Match active setting
		if fields := strings.Fields(trimmed); len(fields) >= 1 && strings.ToLower(fields[0]) == lowerKey {
			lines[i] = key + " " + value
			replaced = true
			break
		}

		// Match commented-out setting (e.g. "#PermitRootLogin yes")
		if strings.HasPrefix(trimmed, "#") {
			uncommented := strings.TrimSpace(strings.TrimPrefix(trimmed, "#"))
			if fields := strings.Fields(uncommented); len(fields) >= 1 && strings.ToLower(fields[0]) == lowerKey {
				lines[i] = key + " " + value
				replaced = true
				break
			}
		}
	}

	if !replaced {
		lines = append(lines, key+" "+value)
	}

	return strings.Join(lines, "\n")
}

func sshExpectExact(actual, expected string) bool {
	return strings.EqualFold(actual, expected)
}

func sshExpectLessOrEqual(actual, expected string) bool {
	a, err := strconv.Atoi(actual)
	if err != nil {
		return false
	}
	e, err := strconv.Atoi(expected)
	if err != nil {
		return false
	}
	return a <= e
}
