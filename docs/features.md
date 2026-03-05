# Features

## 1. Security Scanning

SecTUI scans your system across multiple security domains and produces a hardening score from 0 to 100.

### Security Modules

| Module | What it checks |
|--------|---------------|
| **SSH** | Root login, password auth, key auth, max retries, grace time, X11 forwarding, empty passwords, AllowUsers/Groups, protocol version |
| **Firewall** | UFW, nftables, iptables (Linux), pf (macOS), firewalld (RHEL) — installed, active, rules configured |
| **Network** | Open ports via `ss -tlnp`, listening services, databases exposed on 0.0.0.0 |
| **Users** | Root password status, extra UID 0 accounts, sudo configuration, passwordless sudo |
| **Updates** | Automatic security updates (unattended-upgrades, dnf-automatic, softwareupdate) |
| **Kernel** | sysctl hardening (ASLR, SYN cookies, ICMP redirects, reverse path filtering, ptrace, etc.) |
| **Malware** | ClamAV scan results, rkhunter check results, chkrootkit (via tool integration) |
| **Docker** | Rootless mode, daemon config, socket permissions, privileged containers, image scanning |
| **SSL** | Certificate expiry, TLS version, cipher suites for detected web services |
| **Filesystem** | AIDE integrity check results, critical file permissions (/etc/passwd, /etc/shadow, etc.) |

### Scan Types
- **Quick scan** (~10s): SSH, firewall, users, updates, kernel
- **Full audit** (~2-5min): All modules including port scan, service audit, tool scans
- **Module scan**: Scan specific module only (`sectui scan ssh`)
- **Tool scan**: Trigger ClamAV/rkhunter/AIDE scan and integrate results

### Scoring
- Base score: 100
- Deductions: Critical -15, High -10, Medium -5, Low -2, Info -0
- Score never below 0
- Per-module scores + aggregate total
- Score delta shown: `+5 since last scan`

### Finding Structure
Every finding includes:
- **Title**: Short description of the issue
- **Detail**: Full explanation of WHY this matters (educate!)
- **Fix description**: What SecTUI will do to fix it
- **Current vs Expected values**: What we found vs what's safe
- **Severity level**: Info / Low / Medium / High / Critical

---

## 2. Interactive Hardening

### Fix Application
- Select individual findings or batch-select
- Dry-run preview shows exact changes before applying
- Confirmation dialog for every destructive action
- Automatic backup of all modified config files
- Rollback info printed after each change

### Hardening Profiles
| Profile | What it does | Target Score |
|---------|-------------|-------------|
| **Beginner** | UFW + fail2ban + SSH hardening + auto-updates + ClamAV + sysctl | 70-80 |
| **Intermediate** | Beginner + CrowdSec + AIDE + AppArmor/SELinux + Docker hardening | 80-90 |
| **Fortress** | Intermediate + WireGuard-only SSH + nftables + rootless containers + full sysctl + 2FA | 90-100 |
| **macOS** | pf + SSH hardening + ClamAV + rkhunter + softwareupdate | 70-80 |

### What SecTUI Can Harden
- **SSH**: Modify `/etc/ssh/sshd_config` (root login, password auth, key auth, max retries, grace time, X11, empty passwords)
- **Firewall**: Install + enable UFW, set default deny + allow SSH
- **Kernel**: Write sysctl tweaks to `/etc/sysctl.d/99-sectui.conf`
- **Updates**: Install + configure unattended-upgrades / dnf-automatic
- **Tool Installation**: Install security tools via package manager (fail2ban, ClamAV, etc.)

---

## 3. Tool Management

SecTUI detects, installs, and manages 10+ external security tools.

### Tool Lifecycle
1. **Detection**: Check if binary exists (`which`) + check if service active (`systemctl is-active`)
2. **Installation**: Via package manager (apt, dnf, pacman, brew) or custom installer
3. **Management**: Dedicated TUI per tool — status, config, actions, activity log
4. **Integration**: Use tools to run scans, feed results back into dashboard

### Per-Tool Management UI (for tools with ToolManager)
- 4-panel layout: Status, Config, Quick Actions, Activity
- Quick actions via number keys (1-9)
- Dangerous actions require confirmation
- Live service status + version info
- Config file parsing and display
- Recent activity from journal/logs

### Currently Managed Tools (full ToolManager)
- **ClamAV**: daemon status, DB updates, on-demand scans, config display
- **fail2ban**: jail status, banned IPs, restart, unban, jail.local config
- **CrowdSec**: decisions, alerts, hub updates, bouncer/parser/collection counts
- **AppArmor**: profile status, profile list, reload, restart

### Basic Detection (no full manager yet)
- UFW, rkhunter, WireGuard, Tailscale, AIDE, SELinux

---

## 4. SecStore (App Store)

Modern card-based interface for discovering and installing uninstalled security tools.

- Category filtering (Firewall, IPS, Malware, VPN, FIM, Access Control)
- Two-column card layout with tool description
- One-key install with confirmation dialog
- After install: tool moves from SecStore to sidebar TOOLS section

---

## 5. Setup Wizard

Interactive 5-step first-run experience:

1. **Welcome**: Language selection, introduction
2. **System Detection**: Auto-detect OS, arch, init system, existing tools
3. **Quick Scan**: Initial security assessment with live score
4. **Recommendations**: Checkboxes for hardening actions, tool choices with explanations
5. **Notifications**: Optional Telegram/Discord setup
6. **Done**: Score before/after comparison, useful commands

The wizard is idempotent: running again detects existing config and offers to modify.

---

## 6. Notifications

Real-time alerts via Telegram and Discord when something needs attention.

### Alert Types

| Event | Severity | Default |
|-------|----------|---------|
| Brute force (>5 attempts/5min same IP) | Critical | On |
| Root login attempt | Critical | On |
| SSL cert <7 days from expiry | Critical | On |
| Service crashed | High | On |
| New listening port (not in baseline) | High | On |
| Disk >90% | High | On |
| Unauthorized sudo attempt | Medium | On |
| Security updates available | Low | Off |
| Scan completed | Info | Off |

### Features
- Guided setup wizard for both platforms (step-by-step bot creation / webhook setup)
- Auto-detect Telegram chat ID (poll getUpdates after user sends message)
- Rich Discord embeds with severity colors
- Alert deduplication (same alert + source = suppress for 1h)
- Test command: `sectui alert test`
- Alert history: `sectui alert history`

---

## 7. Scheduled Scans

SecTUI can schedule recurring scans using third-party tools:

- **ClamAV**: Weekly malware scan of configurable paths (`/home`, `/tmp`, `/var/www`)
- **rkhunter**: Weekly rootkit check
- **AIDE**: Daily file integrity check
- **Full SecTUI scan**: Daily/weekly comprehensive security audit

Results are stored and feed into the dashboard score. Notifications sent on new findings.

Configuration via `config.toml`:
```toml
[scan]
schedule = "0 3 * * *"    # cron expression
schedule_enabled = true

[tools.clamav]
scan_schedule = "0 2 * * 0"
scan_paths = ["/home", "/tmp", "/var/www"]
```

---

## 8. Watch Mode (Background Monitor)

Continuous monitoring with real-time alerting:

```bash
sectui watch                    # Foreground
sectui watch --daemon           # Background (systemd recommended)
sectui watch --interval 60      # Check every 60s
```

### What It Monitors
- Auth log: brute force detection, root login attempts
- Port changes: new listening ports vs baseline
- Service status: detect crashes
- SSL certificates: expiry countdown
- Disk space: threshold alerts
- Sudo attempts: unauthorized usage

### Implementation
- Runs as a loop with configurable interval
- Can install as systemd service for auto-start
- Sends notifications through configured channels
- Stores events for `sectui alert history`

---

## 9. Reports

Generate security reports for documentation/compliance:

```bash
sectui report                       # Markdown (default)
sectui report --format json         # JSON
sectui report --format html         # HTML
sectui report compare <scan1> <scan2>  # Compare two scans
sectui report trend                 # Score history over time
```

### Report Contents
- System information
- Overall score + per-module scores
- All findings with severity, description, fix status
- Active security tools
- Recommendations
- Score trend (if history available)

---

## 10. Platform Detection

Automatic detection of:
- **OS**: Linux, macOS
- **Distribution**: Ubuntu, Debian, Fedora, RHEL, Rocky, Arch, Alpine + derivatives (Pop!_OS, Mint, Manjaro, etc.)
- **Architecture**: x86_64, aarch64
- **Init system**: systemd, launchd, OpenRC
- **Package manager**: apt, dnf, pacman, brew, apk
- **Containers**: Docker, LXC, Kubernetes (detect if running inside)
- **WSL**: Windows Subsystem for Linux
- **Docker installed**: version, rootless mode

Derivative distro normalization:
- Pop!_OS, Neon, Elementary, Zorin, Mint -> Ubuntu
- Manjaro, EndeavourOS, Garuda -> Arch
- Rocky, AlmaLinux, CentOS, Oracle Linux -> RHEL

---

## 11. Configuration

TOML config at `~/.config/sectui/config.toml` (XDG-compliant).

Managed via:
```bash
sectui config show          # Display current config
sectui config set key val   # Set a value
sectui config path          # Show file location
sectui config reset         # Reset to defaults
sectui config init          # Re-run setup wizard
```

---

## 12. i18n (Internationalization)

All user-facing strings use i18n keys. YAML locale files embedded in binary.

- Locale detection: config > `SECTUI_LOCALE` env > `LANG` env > fallback `en`
- Finding explanations are the most important content to translate
- Each finding has 3 keys: title, detail (WHY), fix (WHAT)
- Easy contribution: copy en.yml, translate values, submit PR

Planned languages: English, Italian, Spanish, Portuguese, German, French, Japanese, Chinese.

---

## 13. Self-Update

```bash
sectui update               # Update binary
sectui update check         # Check for updates
sectui update db            # Update CVE/vulnerability database
```

Downloads latest release from GitHub, verifies checksum, replaces binary.
