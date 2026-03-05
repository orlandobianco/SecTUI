<p align="center">
  <strong>SecTUI</strong><br>
  Interactive security hardening for Linux &amp; macOS servers
</p>

<p align="center">
  <a href="https://github.com/orlandobianco/SecTUI/actions/workflows/ci.yml"><img src="https://github.com/orlandobianco/SecTUI/actions/workflows/ci.yml/badge.svg" alt="CI"></a>
  <a href="https://github.com/orlandobianco/SecTUI/blob/main/LICENSE"><img src="https://img.shields.io/badge/license-GPL--3.0-blue" alt="License"></a>
  <a href="https://github.com/orlandobianco/SecTUI/releases"><img src="https://img.shields.io/github/v/release/orlandobianco/SecTUI?label=release" alt="Release"></a>
</p>

---

SecTUI is a terminal tool that scans your server for security issues and helps you fix them interactively. It combines automated scanning with an interactive hardening wizard and a real-time TUI dashboard.

Built for developers who deploy apps but aren't security experts. Every finding explains **why** it matters and what the fix does before you apply it.

<img width="2324" height="1342" alt="CleanShot 2026-03-05 at 23 32 59@2x" src="https://github.com/user-attachments/assets/3c67d424-e6c2-45ab-8e44-cf04bbea4e40" />


```sh
curl -fsSL https://orlandobianco.github.io/SecTUI/install.sh | sh
```

Or download a binary from [Releases](https://github.com/orlandobianco/SecTUI/releases).

## Quick Start

```sh
# Launch the TUI dashboard
sectui

# Run a quick security scan
sectui scan

# Interactive hardening (dry-run by default)
sectui harden

# Check your score
sectui status score
```

## Features

### Security Scanning

SecTUI ships with **6 security modules** covering **35+ checks**:

| Module | What It Checks |
|--------|---------------|
| **SSH** | Root login, password auth, key auth, max retries, grace time, X11 forwarding, empty passwords |
| **Firewall** | UFW, iptables, nftables (Linux), pf (macOS), firewalld (RHEL) |
| **Network** | Open ports, listening services, databases exposed on 0.0.0.0 |
| **Users** | Root password, extra UID 0 accounts, passwordless sudo, empty passwords |
| **Updates** | Automatic security updates, pending patches, stale package cache |
| **Kernel** | ASLR, SYN cookies, ICMP redirects, reverse path filtering, ptrace, symlink/hardlink protection |

Every finding includes a severity level, an explanation of **why** it matters, and a concrete fix.

Scoring: base 100, deductions per severity (Critical −15, High −10, Medium −5, Low −2). Score never goes below 0.



### Interactive Hardening

Select findings to fix, preview what will change, confirm, and apply — all from the TUI.

- **Dry-run by default**: see exact changes before applying
- **Automatic backups**: every modified config file is backed up first
- **Root detection**: prompts to re-run with sudo when needed
- **Batch or individual**: select specific fixes or apply all at once

<img width="2328" height="1350" alt="CleanShot 2026-03-05 at 23 36 10@2x" src="https://github.com/user-attachments/assets/80e435a0-c874-4c30-aee1-67e3f4c3bf75" />




### Tool Management

SecTUI detects, installs, and manages **10 external security tools** across 6 categories.

**Full management UI** (4-panel layout with status, actions, config, and activity) for:

| Tool | Category | Quick Actions |
|------|----------|---------------|
| **fail2ban** | Intrusion Prevention | SSH jail status, banned IPs, restart, unban all |
| **CrowdSec** | Intrusion Prevention | Active decisions, recent alerts, hub update, restart |
| **ClamAV** | Malware Detection | Scan /home, scan /tmp, update virus DB, start daemon |
| **AppArmor** | Access Control | Full status, list profiles, reload, restart |

Dangerous actions (restart, unban) require explicit confirmation. Non-dangerous actions execute immediately.

<img width="2326" height="1346" alt="CleanShot 2026-03-05 at 23 35 30@2x" src="https://github.com/user-attachments/assets/feaa5ec6-ae91-4916-be06-840a0d00ab1b" />


**Basic detection** (status badge in sidebar) for: UFW, firewalld, rkhunter, WireGuard, Tailscale, AIDE.

### SecStore

App-store style interface for discovering and installing security tools that aren't on your system yet.

- Browse by category: Firewall, IPS, Malware, VPN, FIM, Access Control
- Filter with `1-7` number keys
- One-key install with confirmation dialog
- After install, tool moves from SecStore to the sidebar TOOLS section

<img width="2330" height="1350" alt="CleanShot 2026-03-05 at 23 34 05@2x" src="https://github.com/user-attachments/assets/ed01cbeb-35c1-4149-b4f1-d4a01f34fecd" />


## TUI Dashboard

The dashboard uses a sidebar + content layout with context-sensitive navigation.

- **Overview** — Score gauge, platform info, findings summary
- **Modules** — Per-module findings with details, current/expected values, and fix selection
- **Tools** — 4-panel management UI for installed tools (status, actions, config, activity)
- **SecStore** — Browse and install uninstalled security tools

Visual focus indicators show whether you're navigating the sidebar or the content area.

<!-- SCREENSHOT: overview — overview dashboard showing score, system info, findings summary by severity -->

### Key Bindings

| Key | Action |
|-----|--------|
| `Tab` | Toggle focus between sidebar and content |
| `j` / `k` | Navigate up/down |
| `Enter` / `l` | Select / enter content |
| `h` / `Esc` | Back to sidebar |
| `s` | Start security scan |
| `Space` | Toggle fix selection (module view) |
| `a` | Select/deselect all fixes (module view) |
| `1-4` | Execute quick action (tool view) |
| `r` | Refresh tool data (tool view) |
| `1-7` | Filter by category (SecStore) |
| `?` | Toggle help overlay |
| `q` | Quit |

The footer dynamically shows contextual hints for the active view.

## CLI Commands

```
sectui                          Launch TUI dashboard
sectui scan [quick|full|MODULE] Run a security scan
sectui status [score]           Show security status or score only
sectui harden [check|ssh|firewall|kernel|all]
                                Interactive hardening (dry-run by default)
sectui version                  Print version
```

### Harden Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--dry-run` | `true` | Show what would change without applying |
| `--no-backup` | `false` | Skip config file backups |

Every fix requires explicit confirmation (`y/N`) before applying. There is no way to skip confirmation.

## Platforms

| | Linux | macOS |
|--|-------|-------|
| SSH | Yes | Yes |
| Firewall | ufw, iptables, nftables, firewalld | pf |
| Network | `ss` | `lsof` |
| Users | /etc/passwd, /etc/shadow | dscl |
| Updates | apt, dnf | softwareupdate |
| Kernel | sysctl | Skipped |

### Tool Availability

| Tool | Debian/Ubuntu | RHEL/Fedora | macOS |
|------|--------------|-------------|-------|
| UFW | Yes | — | — |
| firewalld | — | Yes | — |
| fail2ban | Yes | Yes | — |
| CrowdSec | Yes | Yes | — |
| ClamAV | Yes | Yes | Yes |
| rkhunter | Yes | Yes | Yes |
| WireGuard | Yes | Yes | Yes |
| Tailscale | Yes | Yes | Yes |
| AIDE | Yes | Yes | — |
| AppArmor | Yes | — | — |

## Building from Source

```sh
go build -o sectui ./cmd/sectui
```

Cross-compile:
```sh
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags "-s -w" -o sectui ./cmd/sectui
```

Run tests:
```sh
go test ./...
```

## Architecture

```
cmd/sectui/main.go          CLI entry point (Cobra)
internal/core/               Types, interfaces, platform detection, scoring
internal/modules/            Security scan modules (SSH, Firewall, Network, ...)
internal/tools/              Tool management (detect, install, ToolManager)
internal/tui/                Bubble Tea TUI (app, sidebar, views, theme)
locales/                     i18n YAML locale files
docs/                        Design documentation
```

Key interfaces:
- **SecurityModule** — `Scan()` → findings, `ApplyFix()` → harden
- **SecurityTool** — `Detect()` → status, `InstallCommand()` → install string
- **ToolManager** — extends SecurityTool with `GetServiceStatus()`, `QuickActions()`, `ConfigSummary()`, `RecentActivity()`, `ExecuteAction()`

## License

[GPL-3.0](LICENSE)
