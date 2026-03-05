# Architecture

## Package Structure

```
sectui/
├── cmd/                          # CLI entry point
│   └── sectui/
│       └── main.go               # clap-style subcommand routing
│
├── internal/
│   ├── core/                     # Pure logic, no TUI deps
│   │   ├── platform.go           # OS/distro/arch detection
│   │   ├── config.go             # TOML config management (XDG)
│   │   ├── report.go             # Scan results, scoring (0-100)
│   │   ├── notifier.go           # Telegram/Discord dispatch
│   │   └── scheduler.go          # Cron-like scheduled scans
│   │
│   ├── modules/                  # Security check modules
│   │   ├── registry.go           # SecurityModule trait + registry
│   │   ├── ssh.go                # SSH config analysis + hardening
│   │   ├── firewall.go           # UFW / pf / nftables / firewalld
│   │   ├── network.go            # Open ports, listening services
│   │   ├── users.go              # User/permission/sudo audit
│   │   ├── kernel.go             # sysctl, AppArmor, SELinux
│   │   ├── updates.go            # Unattended upgrades, package freshness
│   │   ├── malware.go            # ClamAV, rkhunter, chkrootkit
│   │   ├── filesystem.go         # AIDE, file permissions, FIM
│   │   ├── docker.go             # Container security, rootless check
│   │   └── ssl.go                # Certificate expiry, TLS config
│   │
│   ├── tools/                    # External tool management
│   │   ├── registry.go           # SecurityTool trait + registry
│   │   ├── manager.go            # ToolManager trait (full management)
│   │   ├── ufw.go
│   │   ├── fail2ban.go
│   │   ├── crowdsec.go
│   │   ├── clamav.go
│   │   ├── rkhunter.go
│   │   ├── wireguard.go
│   │   ├── tailscale.go
│   │   ├── aide.go
│   │   ├── apparmor.go
│   │   └── selinux.go
│   │
│   └── tui/                      # Terminal UI (Bubble Tea)
│       ├── app.go                # Main model, event loop, routing
│       ├── theme.go              # Colors, styles, branding
│       ├── sidebar.go            # Left sidebar navigation
│       ├── overview.go           # Dashboard overview (animated)
│       ├── module_content.go     # Module findings + fix UI
│       ├── tool_content.go       # Per-tool management UI
│       ├── secstore.go           # App store for tools
│       ├── scanner.go            # Scan progress view
│       ├── wizard.go             # First-run setup wizard
│       ├── dialog.go             # Confirm/error dialogs
│       └── help.go               # Keybinding overlay
│
├── locales/                      # i18n YAML files
│   ├── en.yml
│   ├── it.yml
│   └── ...
│
├── scripts/
│   └── install.sh                # curl|bash installer
│
├── docs/                         # This documentation
├── go.mod
├── go.sum
└── README.md
```

## Core Interfaces

### SecurityModule (scan + harden)

```
SecurityModule:
    id()             -> string          # "ssh", "firewall"
    name_key()       -> string          # i18n key for display name
    description_key() -> string         # i18n key for description
    scan(ctx)        -> []Finding       # Run all security checks
    available_fixes() -> []Fix          # List fixable findings
    apply_fix(id, ctx) -> ApplyResult   # Apply a specific fix
    preview_fix(id, ctx) -> string      # Dry-run preview
    is_applicable(platform) -> bool     # Relevant on this OS?
    priority()       -> int             # Scan ordering (lower = first)
```

### SecurityTool (detect + install)

```
SecurityTool:
    id()             -> string          # "ufw", "fail2ban"
    name()           -> string          # Display name
    description()    -> string          # What it does
    category()       -> ToolCategory    # Firewall, IPS, Malware, etc.
    detect(platform) -> ToolStatus      # NotInstalled/Installed/Active
    install_command(platform) -> string # apt install -y fail2ban
    is_applicable(platform) -> bool     # Relevant on this OS?
```

### ToolManager (full management UI)

```
ToolManager:
    tool_id()        -> string
    service_status() -> ServiceStatus   # Running, PID, version, extras
    quick_actions()  -> []QuickAction   # Keybind-triggered actions
    config_summary() -> []ConfigEntry   # Key config values
    recent_activity(n) -> []Activity    # Journal/log entries
    execute_action(id) -> ActionResult  # Run a quick action
    run_scan()       -> []Finding       # Use tool to scan, return findings
```

**Key difference from SecurityModule**: ToolManager wraps an external tool (ClamAV, rkhunter, etc.) and uses it to perform scans. Results are converted to SecTUI Findings and feed back into the dashboard score.

## Key Data Types

```
Severity: Info | Low | Medium | High | Critical

Finding:
    id              string          # "ssh-001"
    module          string          # "ssh"
    severity        Severity
    title_key       string          # i18n key
    detail_key      string          # i18n key (explains WHY)
    fix_id          string?         # nil if no auto-fix
    current_value   string?
    expected_value  string?

Report:
    timestamp       time.Time
    platform        PlatformInfo
    score           int             # 0-100 hardening index
    findings        []Finding
    modules_scanned []string
    duration        time.Duration

PlatformInfo:
    os              OS              # Linux, MacOS
    distro          string?         # Ubuntu, Debian, Fedora, Arch...
    version         string?
    arch            string          # x86_64, aarch64
    init_system     InitSystem      # Systemd, Launchd, OpenRC
    package_manager PackageManager? # Apt, Dnf, Pacman, Brew, Apk
    is_container    bool
    is_wsl          bool

ToolCategory: Firewall | IntrusionPrevention | Malware | Vpn | FileIntegrity | AccessControl

ToolStatus: NotInstalled | Installed | Active | NotApplicable

ServiceStatus:
    running         bool
    enabled         bool
    pid             int?
    uptime          string?
    extra           map[string]string   # tool-specific data (version, jails, etc.)

QuickAction:
    id              string          # "status_sshd"
    label           string          # "SSH jail status"
    key             char            # '1' (keyboard shortcut)
    dangerous       bool            # requires confirmation
    description     string

ConfigEntry:
    key             string          # "bantime"
    value           string          # "10m"

ActivityEntry:
    timestamp       string          # "Mar 05 14:23"
    message         string

ActionResult:
    success         bool
    message         string
```

## Scoring System

Base score: 100. Subtract per finding:

| Severity | Penalty |
|----------|---------|
| Critical | -15 |
| High | -10 |
| Medium | -5 |
| Low | -2 |
| Info | 0 |

Score never goes below 0. Per-module scores + aggregate total.

**Tool integration bonus**: Active security tools add a bonus:
- Active firewall: +0 (expected)
- Active IPS (fail2ban/CrowdSec): +0 (expected)
- Missing firewall: penalty via firewall module finding
- Having ClamAV scan results with no malware: reduces malware module severity

## Configuration

Location: `~/.config/sectui/config.toml` (XDG-compliant)

```toml
[general]
locale = "en"
color = true

[scan]
default_type = "quick"          # quick | full
excluded_modules = []
schedule = "0 3 * * *"          # cron expression (3 AM daily)
schedule_enabled = false

[notifications]
enabled = false

[notifications.telegram]
enabled = false
token = ""
chat_id = ""

[notifications.discord]
enabled = false
webhook_url = ""

[dashboard]
refresh_interval = 5            # seconds

[harden]
auto_backup = true
dry_run_default = true

[tools]
# Per-tool scan schedules
[tools.clamav]
scan_schedule = "0 2 * * 0"     # weekly Sunday 2 AM
scan_paths = ["/home", "/tmp", "/var/www"]

[tools.rkhunter]
scan_schedule = "0 3 * * 0"     # weekly Sunday 3 AM

[tools.aide]
check_schedule = "0 4 * * *"    # daily 4 AM
```

## Design Principles

1. **Single binary** — no runtime deps, no Python, no Node
2. **Offline-first** — works without internet
3. **Idempotent** — running harden twice produces same result
4. **Respect existing config** — detect what's already set up, don't overwrite
5. **Educate** — every finding has WHY explanation, not just WHAT
6. **Never break a system** — dry-run by default, backup before changes
7. **Tool integration** — don't just manage tools, USE them for scanning

## i18n

- YAML locale files compiled/embedded in binary
- `t("key")` helper function everywhere
- Locale priority: config file > `SECTUI_LOCALE` env > `LANG` env > fallback `en`
- Every user-facing string uses i18n key, never hardcoded
- Findings have 3 keys each: `title`, `detail` (WHY), `fix` (WHAT we'll do)
