package modules

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/orlandobianco/SecTUI/internal/core"
)

const (
	usersModuleID = "users"

	passwdPath  = "/etc/passwd"
	shadowPath  = "/etc/shadow"
	sudoersPath = "/etc/sudoers"
	sudoersDDir = "/etc/sudoers.d"

	// Minimum UID for non-system (human) accounts on most Linux distributions.
	minHumanUID = 1000
)

// noLoginShells lists shells that indicate the account cannot log in interactively.
var noLoginShells = []string{
	"/sbin/nologin",
	"/bin/false",
	"/usr/sbin/nologin",
	"/usr/bin/false",
}

// passwdEntry represents a single parsed line from /etc/passwd.
type passwdEntry struct {
	Username string
	UID      int
	GID      int
	Comment  string
	Home     string
	Shell    string
}

// shadowEntry represents the user and password hash from /etc/shadow.
type shadowEntry struct {
	Username string
	Hash     string // "!" or "*" = locked, "" = empty/no password, otherwise a hash
}

// UsersModule scans user accounts for security issues.
// It ONLY reads system files and never modifies anything.
type UsersModule struct{}

func NewUsersModule() *UsersModule {
	return &UsersModule{}
}

func (m *UsersModule) ID() string             { return usersModuleID }
func (m *UsersModule) NameKey() string        { return "module.users.name" }
func (m *UsersModule) DescriptionKey() string { return "module.users.description" }
func (m *UsersModule) Priority() int          { return 40 }

func (m *UsersModule) IsApplicable(_ *core.PlatformInfo) bool {
	return true
}

func (m *UsersModule) Scan(ctx *core.ScanContext) []core.Finding {
	var findings []core.Finding

	isDarwin := ctx.Platform.OS == core.OSDarwin

	// --- Parse /etc/passwd (available on both Linux and macOS) ---
	passwdEntries, passwdErr := parsePasswdFile(passwdPath)
	if passwdErr != nil {
		// On macOS, /etc/passwd may be minimal; fall back to dscl.
		if isDarwin {
			passwdEntries = listDarwinUsers()
		}
	}

	// --- Parse /etc/shadow (Linux only) ---
	var shadowEntries []shadowEntry
	var shadowErr error
	if !isDarwin {
		shadowEntries, shadowErr = parseShadowFile(shadowPath)
	}

	// --- Check usr-001: Root account has password set ---
	if !isDarwin {
		findings = append(findings, checkRootPassword(shadowEntries, shadowErr)...)
	}

	// --- Check usr-002: Extra UID 0 accounts ---
	if passwdErr == nil || isDarwin {
		findings = append(findings, checkExtraUID0(passwdEntries)...)
	}

	// --- Check usr-003: Passwordless sudo ---
	findings = append(findings, checkPasswordlessSudo()...)

	// --- Check usr-004: Users with empty passwords ---
	if !isDarwin {
		findings = append(findings, checkEmptyPasswords(shadowEntries, shadowErr, passwdEntries)...)
	}

	// --- Check usr-005: Non-system users with shell access ---
	if passwdErr == nil || isDarwin {
		findings = append(findings, checkShellUsers(passwdEntries, isDarwin)...)
	}

	return findings
}

func (m *UsersModule) AvailableFixes() []core.Fix {
	return []core.Fix{
		{
			ID:          "fix-usr-lock-empty",
			FindingID:   "usr-004",
			TitleKey:    "finding.usr_empty_password.title",
			Description: "Lock all accounts that have empty passwords (passwd -l)",
			Dangerous:   true,
		},
	}
}

func (m *UsersModule) PreviewFix(fixID string, _ *core.ScanContext) (string, error) {
	if fixID != "fix-usr-lock-empty" {
		return "", fmt.Errorf("unknown fix: %s", fixID)
	}

	entries, err := parseShadowFile(shadowPath)
	if err != nil {
		return "", fmt.Errorf("cannot read shadow file: %w", err)
	}

	var b strings.Builder
	b.WriteString("Lock accounts with empty passwords:\n")
	found := false
	for _, e := range entries {
		if e.Hash == "" {
			b.WriteString(fmt.Sprintf("  passwd -l %s\n", e.Username))
			found = true
		}
	}
	if !found {
		b.WriteString("  (no accounts with empty passwords found)\n")
	}
	return b.String(), nil
}

func (m *UsersModule) ApplyFix(fixID string, ctx *core.ApplyContext) (*core.ApplyResult, error) {
	if fixID != "fix-usr-lock-empty" {
		return nil, fmt.Errorf("unknown fix: %s", fixID)
	}

	entries, err := parseShadowFile(shadowPath)
	if err != nil {
		return nil, fmt.Errorf("cannot read shadow file: %w", err)
	}

	var toLock []string
	for _, e := range entries {
		if e.Hash == "" {
			toLock = append(toLock, e.Username)
		}
	}

	if len(toLock) == 0 {
		return &core.ApplyResult{
			Success: true,
			Message: "No accounts with empty passwords found",
		}, nil
	}

	if ctx.DryRun {
		return &core.ApplyResult{
			Success: true,
			Message: fmt.Sprintf("[dry-run] Would lock %d account(s): %s", len(toLock), strings.Join(toLock, ", ")),
		}, nil
	}

	var locked []string
	var failed []string
	for _, user := range toLock {
		cmd := exec.Command("passwd", "-l", user)
		if out, err := cmd.CombinedOutput(); err != nil {
			failed = append(failed, fmt.Sprintf("%s (%s)", user, strings.TrimSpace(string(out))))
		} else {
			locked = append(locked, user)
		}
	}

	if len(failed) > 0 && len(locked) == 0 {
		return nil, fmt.Errorf("failed to lock all accounts: %s", strings.Join(failed, "; "))
	}

	msg := fmt.Sprintf("Locked %d account(s): %s", len(locked), strings.Join(locked, ", "))
	if len(failed) > 0 {
		msg += fmt.Sprintf(" (failed: %s)", strings.Join(failed, "; "))
	}

	return &core.ApplyResult{
		Success: len(locked) > 0,
		Message: msg,
	}, nil
}

// ---------------------------------------------------------------------------
// Check implementations (read-only)
// ---------------------------------------------------------------------------

// checkRootPassword reports when root has a password hash set (Info level).
// A locked root (hash starts with "!" or "*") is the safer default; having a
// password set is not necessarily wrong but worth noting.
func checkRootPassword(entries []shadowEntry, readErr error) []core.Finding {
	if readErr != nil {
		return []core.Finding{{
			ID:           "usr-001",
			Module:       usersModuleID,
			Severity:     core.SeverityInfo,
			TitleKey:     "finding.usr_shadow_unreadable.title",
			DetailKey:    "finding.usr_shadow_unreadable.detail",
			CurrentValue: readErr.Error(),
		}}
	}

	for _, e := range entries {
		if e.Username != "root" {
			continue
		}

		if isPasswordLocked(e.Hash) {
			// Root is locked -- nothing to report.
			return nil
		}

		// Root has a real password hash set.
		return []core.Finding{{
			ID:            "usr-001",
			Module:        usersModuleID,
			Severity:      core.SeverityInfo,
			TitleKey:      "finding.usr_root_password.title",
			DetailKey:     "finding.usr_root_password.detail",
			CurrentValue:  "password set",
			ExpectedValue: "locked (! or *)",
		}}
	}

	return nil
}

// checkExtraUID0 looks for accounts with UID 0 that are NOT "root".
func checkExtraUID0(entries []passwdEntry) []core.Finding {
	var extras []string
	for _, e := range entries {
		if e.UID == 0 && e.Username != "root" {
			extras = append(extras, e.Username)
		}
	}

	if len(extras) == 0 {
		return nil
	}

	return []core.Finding{{
		ID:            "usr-002",
		Module:        usersModuleID,
		Severity:      core.SeverityCritical,
		TitleKey:      "finding.usr_extra_uid0.title",
		DetailKey:     "finding.usr_extra_uid0.detail",
		CurrentValue:  strings.Join(extras, ", "),
		ExpectedValue: "only root should have UID 0",
	}}
}

// checkPasswordlessSudo scans /etc/sudoers and /etc/sudoers.d/* for NOPASSWD directives.
func checkPasswordlessSudo() []core.Finding {
	var nopasswdLines []string

	// Scan main sudoers file.
	if lines, err := scanFileForNopasswd(sudoersPath); err == nil {
		nopasswdLines = append(nopasswdLines, lines...)
	}

	// Scan drop-in directory.
	entries, err := os.ReadDir(sudoersDDir)
	if err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			path := filepath.Join(sudoersDDir, entry.Name())
			if lines, err := scanFileForNopasswd(path); err == nil {
				nopasswdLines = append(nopasswdLines, lines...)
			}
		}
	}

	if len(nopasswdLines) == 0 {
		return nil
	}

	return []core.Finding{{
		ID:            "usr-003",
		Module:        usersModuleID,
		Severity:      core.SeverityHigh,
		TitleKey:      "finding.usr_nopasswd_sudo.title",
		DetailKey:     "finding.usr_nopasswd_sudo.detail",
		CurrentValue:  fmt.Sprintf("%d NOPASSWD rule(s) found", len(nopasswdLines)),
		ExpectedValue: "no NOPASSWD rules",
	}}
}

// checkEmptyPasswords finds accounts whose shadow entry has an empty password field.
// It generates one finding per empty-password user so each can be fixed individually.
func checkEmptyPasswords(entries []shadowEntry, readErr error, passwdEntries []passwdEntry) []core.Finding {
	if readErr != nil {
		return nil
	}

	// Build a set of human users (UID >= 1000) or root for targeted findings.
	humanOrRoot := map[string]bool{"root": true}
	for _, pe := range passwdEntries {
		if pe.UID >= minHumanUID {
			humanOrRoot[pe.Username] = true
		}
	}

	var empty []string
	for _, e := range entries {
		if e.Hash == "" {
			empty = append(empty, e.Username)
		}
	}

	if len(empty) == 0 {
		return nil
	}

	// Single consolidated finding with a fix ID to lock all empty-password accounts.
	return []core.Finding{{
		ID:            "usr-004",
		Module:        usersModuleID,
		Severity:      core.SeverityCritical,
		TitleKey:      "finding.usr_empty_password.title",
		DetailKey:     "finding.usr_empty_password.detail",
		FixID:         "fix-usr-lock-empty",
		CurrentValue:  strings.Join(empty, ", "),
		ExpectedValue: "all accounts should have a password or be locked",
	}}
}

// checkShellUsers counts non-system accounts (UID >= 1000) that have a real
// interactive shell. This is informational, not a problem by itself.
func checkShellUsers(entries []passwdEntry, isDarwin bool) []core.Finding {
	var users []string
	for _, e := range entries {
		if e.UID < minHumanUID {
			continue
		}
		if isNoLoginShell(e.Shell) {
			continue
		}
		users = append(users, e.Username)
	}

	if len(users) == 0 {
		return nil
	}

	return []core.Finding{{
		ID:            "usr-005",
		Module:        usersModuleID,
		Severity:      core.SeverityMedium,
		TitleKey:      "finding.usr_shell_users.title",
		DetailKey:     "finding.usr_shell_users.detail",
		CurrentValue:  fmt.Sprintf("%d user(s): %s", len(users), strings.Join(users, ", ")),
		ExpectedValue: "review that all listed users require shell access",
	}}
}

// ---------------------------------------------------------------------------
// File parsing helpers (read-only)
// ---------------------------------------------------------------------------

// parsePasswdFile reads /etc/passwd and returns parsed entries.
// Format: user:x:uid:gid:comment:home:shell
func parsePasswdFile(path string) ([]passwdEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var entries []passwdEntry
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		fields := strings.Split(line, ":")
		if len(fields) < 7 {
			continue
		}

		uid, err := strconv.Atoi(fields[2])
		if err != nil {
			continue
		}
		gid, _ := strconv.Atoi(fields[3])

		entries = append(entries, passwdEntry{
			Username: fields[0],
			UID:      uid,
			GID:      gid,
			Comment:  fields[4],
			Home:     fields[5],
			Shell:    fields[6],
		})
	}

	if err := scanner.Err(); err != nil {
		return entries, err
	}

	return entries, nil
}

// parseShadowFile reads /etc/shadow and returns username + hash pairs.
// Format: user:hash:lastchanged:min:max:warn:inactive:expire:reserved
func parseShadowFile(path string) ([]shadowEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var entries []shadowEntry
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		fields := strings.Split(line, ":")
		if len(fields) < 2 {
			continue
		}

		entries = append(entries, shadowEntry{
			Username: fields[0],
			Hash:     fields[1],
		})
	}

	if err := scanner.Err(); err != nil {
		return entries, err
	}

	return entries, nil
}

// scanFileForNopasswd opens a file and returns trimmed lines that contain
// NOPASSWD (case-insensitive), skipping comment lines.
func scanFileForNopasswd(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var matches []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.Contains(strings.ToUpper(line), "NOPASSWD") {
			matches = append(matches, line)
		}
	}

	return matches, scanner.Err()
}

// listDarwinUsers uses dscl to enumerate local users on macOS and returns them
// as passwdEntry values. This is a fallback when /etc/passwd is incomplete.
func listDarwinUsers() []passwdEntry {
	out, err := exec.Command("dscl", ".", "-list", "/Users", "UniqueID").CombinedOutput()
	if err != nil {
		return nil
	}

	var entries []passwdEntry
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		uid, err := strconv.Atoi(fields[len(fields)-1])
		if err != nil {
			continue
		}

		shell := getDarwinUserShell(fields[0])

		entries = append(entries, passwdEntry{
			Username: fields[0],
			UID:      uid,
			Shell:    shell,
		})
	}

	return entries
}

// getDarwinUserShell reads the UserShell attribute for a macOS local user via dscl.
func getDarwinUserShell(username string) string {
	out, err := exec.Command("dscl", ".", "-read", "/Users/"+username, "UserShell").CombinedOutput()
	if err != nil {
		return "/usr/bin/false"
	}

	// Output format: "UserShell: /bin/zsh"
	line := strings.TrimSpace(string(out))
	if idx := strings.Index(line, ":"); idx != -1 {
		return strings.TrimSpace(line[idx+1:])
	}

	return "/usr/bin/false"
}

// ---------------------------------------------------------------------------
// Small utility helpers
// ---------------------------------------------------------------------------

// isPasswordLocked returns true when the shadow hash field indicates the
// account cannot authenticate with a password.
func isPasswordLocked(hash string) bool {
	return hash == "!" || hash == "*" || hash == "!!" ||
		strings.HasPrefix(hash, "!") || strings.HasPrefix(hash, "*")
}

// isNoLoginShell returns true if the given shell path is a well-known
// no-login shell.
func isNoLoginShell(shell string) bool {
	for _, s := range noLoginShells {
		if shell == s {
			return true
		}
	}
	return false
}
