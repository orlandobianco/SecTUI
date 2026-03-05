# CLI Commands

## Command Structure

```
sectui [COMMAND] [SUBCOMMAND] [FLAGS]
```

When no command is given, `sectui` opens the TUI dashboard.

## Global Flags

```
--config <path>      Custom config file path
--no-color           Disable colored output (also: NO_COLOR env var)
--quiet / -q         Minimal output
--verbose / -v       Detailed output (stackable: -vvv)
--format <format>    Output format: table (default), json, yaml
--help / -h          Show help
--version / -V       Show version
```

## Commands

### `sectui` (no args)

Opens the TUI dashboard. Same as `sectui dashboard`.

---

### `sectui setup`

First-run interactive wizard. Detects system, scans, recommends, and applies hardening.

```
sectui setup                    # Interactive 5-step wizard
sectui setup --non-interactive  # Apply defaults without prompts (for automation)
```

Idempotent: running again detects existing config, offers to modify.

---

### `sectui scan`

Security scanning.

```
sectui scan                     # Quick scan (default, ~10s)
sectui scan quick               # Explicit quick scan
sectui scan full                # Comprehensive audit (~2-5 min, includes tool scans)
sectui scan ssh                 # Scan specific module only
sectui scan ports               # Port scan only
sectui scan services            # Service audit only
sectui scan ssl <domain>        # Check SSL cert for domain
sectui scan packages            # Check installed packages vs CVE DB
sectui scan docker              # Docker security audit
```

**Flags**:
```
--format <table|json|yaml>      Output format
--output <file>                 Write results to file
--severity <low|med|high|crit>  Filter minimum severity
--module <name>                 Scan specific module only
```

**Full scan includes tool scans**: If ClamAV is installed, runs malware scan. If rkhunter is installed, runs rootkit check. If AIDE is initialized, runs integrity check. All results feed into the overall score.

---

### `sectui dashboard`

Full TUI dashboard with real-time updates. Default command.

```
sectui dashboard                # Launch TUI
sectui stat                     # Alias
```

**Flags**:
```
--refresh <seconds>             Update interval (default: 5)
--layout <compact|full>         Layout density
```

---

### `sectui status`

One-shot status report. Prints to stdout and exits (no TUI).

```
sectui status                   # Full summary
sectui status ports             # Open ports
sectui status ssh               # Active SSH sessions
sectui status firewall          # Firewall rules
sectui status services          # Running services
sectui status disk              # Disk usage
sectui status docker            # Container health
sectui status score             # Just the hardening score
```

---

### `sectui harden`

Apply security hardening.

```
sectui harden                   # Interactive hardening (TUI mode)
sectui harden check             # Audit current level (score 0-100)
sectui harden apply <profile>   # Apply profile: beginner|intermediate|fortress
sectui harden ssh               # Harden SSH specifically
sectui harden firewall          # Setup/harden firewall
sectui harden kernel            # Apply sysctl tweaks
sectui harden updates           # Setup auto-updates
```

**Flags**:
```
--dry-run                       Show what would change (default: true)
--yes / -y                      Skip confirmations
--backup / --no-backup          Backup configs before changing (default: true)
```

---

### `sectui alert`

Notification management.

```
sectui alert config             # Interactive notification setup wizard
sectui alert test               # Test all configured channels
sectui alert test telegram      # Test Telegram only
sectui alert test discord       # Test Discord only
sectui alert history            # Show recent alerts
sectui alert history --clear    # Clear history
```

---

### `sectui watch`

Continuous background monitoring with alerting.

```
sectui watch                    # Run in foreground
sectui watch --daemon           # Daemonize (systemd service recommended)
sectui watch --interval <secs>  # Check frequency (default: 60)
```

**Alert triggers**:
- >5 failed logins from same IP in 5 min
- New listening port not in baseline
- SSL cert <7 days from expiry
- Service enters failed state
- Disk >90%
- Root login attempt
- Unauthorized sudo attempt

---

### `sectui report`

Generate security reports.

```
sectui report                   # Generate full report (default: markdown)
sectui report --format <fmt>    # Format: markdown, json, html
sectui report --output <file>   # Save to file
sectui report compare <a> <b>   # Compare two scan results
sectui report trend             # Show score trend over time
```

---

### `sectui config`

Configuration management.

```
sectui config init              # Re-run setup wizard
sectui config show              # Display current config
sectui config set <key> <val>   # Set a value (e.g., sectui config set locale it)
sectui config path              # Show config file location
sectui config reset             # Reset to defaults (with confirmation)
```

---

### `sectui update`

Self-update and database updates.

```
sectui update                   # Update SecTUI binary
sectui update check             # Check for updates without applying
sectui update db                # Update CVE/vulnerability database
```

---

### `sectui completions`

Generate shell completions.

```
sectui completions bash > /etc/bash_completion.d/sectui
sectui completions zsh > ~/.zfunc/_sectui
sectui completions fish > ~/.config/fish/completions/sectui.fish
```

---

## Non-Interactive Mode

When stdout is not a TTY (piped or redirected):
- Disable colors automatically
- Disable progress animations
- Use machine-parseable output
- Skip interactive prompts (use defaults or fail)

Detection: `os.Stdout.Fd()` isatty check.

---

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | General error |
| 2 | Invalid arguments |
| 3 | Permission denied (needs root/sudo) |
| 4 | Scan found critical issues (useful for CI/CD) |
| 5 | Configuration error |

Exit code 4 is useful for CI pipelines:
```bash
sectui scan --format json || echo "Security issues found!"
```

---

## Installation

```bash
# One-liner (recommended)
curl -fsSL https://get.sectui.dev | bash

# Specific version
curl -fsSL https://get.sectui.dev | bash -s -- --version v1.0.0

# From source
git clone https://github.com/orlandobianco/sectui.git
cd sectui && go build -o sectui ./cmd/sectui

# Uninstall
rm $(which sectui)
rm -rf ~/.config/sectui/
rm -rf ~/.local/share/sectui/
```
