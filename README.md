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

## Install

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

## What It Scans

SecTUI ships with **6 security modules** covering **35+ checks**:

<img width="1532" height="958" alt="iTerm2 2026-03-05 15 08 31" src="https://github.com/user-attachments/assets/8bd66e84-2c2f-45ee-8abd-d86df5dc528e" />


<img width="1550" height="966" alt="iTerm2 2026-03-05 14 00 33" src="https://github.com/user-attachments/assets/8ca84084-17e7-42c0-9bd9-db31f00fff77" />


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

## TUI Dashboard

The dashboard has a sidebar + content layout:

- **Overview** - Score gauge, platform info, findings summary
- **Modules** - Per-module findings with details, current/expected values, and fix selection
- **Tools** / **SecStore** - Coming soon

Key bindings:
| Key | Action |
|-----|--------|
| `s` | Start scan |
| `Tab` | Toggle sidebar/content focus |
| `j`/`k` | Navigate |
| `Space` | Toggle fix selection |
| `a` | Select all fixes |
| `Enter` | Apply selected fixes |
| `h`/`Esc` | Back |
| `q` | Quit |

## Platforms

| | Linux | macOS |
|--|-------|-------|
| SSH | Yes | Yes |
| Firewall | ufw, iptables, nftables | pf |
| Network | `ss` | `lsof` |
| Users | /etc/passwd, /etc/shadow | dscl |
| Updates | apt, dnf | softwareupdate |
| Kernel | sysctl | Skipped |

## Building from Source

```sh
go build -o sectui ./cmd/sectui
```

Cross-compile:
```sh
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags "-s -w" -o sectui ./cmd/sectui
```

## License

[GPL-3.0](LICENSE)
