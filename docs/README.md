# SecTUI - Product Specification

**Security hardening for your server — scan, harden, monitor.**

A single binary that helps anyone secure a Linux or macOS VPS. Combines security scanning, interactive hardening, tool management, and a real-time TUI dashboard.

## Target Audience

Developers who can build apps (vibe coding with Cursor, Bolt, v0) but don't know how to secure a VPS for self-hosting. SecTUI makes security approachable.

## Philosophy

1. **Educate while hardening** — every finding explains WHY it matters, not just what's wrong
2. **Sensible defaults, expert options** — beginners get safe presets, power users get full control
3. **Beautiful terminal UI** — security should be approachable, not intimidating
4. **Single binary, zero deps** — no runtime dependencies, no Python, no Node
5. **Offline-first** — works without internet (CVE checks optional)
6. **Idempotent** — running harden twice produces the same result
7. **Respect existing config** — detects what's already set up, never overwrites blindly
8. **Where tools overlap** (fail2ban vs CrowdSec) — explain differences and let user choose

## Documentation Index

| Document | Description |
|----------|-------------|
| [Architecture](architecture.md) | Package structure, core traits, data types, config system, design principles |
| [UI Design](ui-design.md) | Full TUI design with ASCII mockups — dashboard, sidebar, tool UIs, wizard, scanner, dialogs |
| [Features](features.md) | Complete feature list with detailed descriptions |
| [Tools](tools.md) | All supported security tools by category — detection, management UI, integration |
| [Integrations](integrations.md) | Third-party tool scans, notifications (Telegram/Discord), scheduled scans |
| [SecStore](secstore.md) | App store for security tools — categories, card UI, install flow |
| [CLI Commands](cli.md) | Full command reference — flags, subcommands, exit codes, completions |

## Quick Reference

```
sectui                    # Open TUI dashboard (default)
sectui scan               # Run security scan
sectui setup              # First-run interactive wizard
sectui harden             # Apply security hardening
sectui alert config       # Setup notifications
sectui watch              # Continuous background monitoring
```

## Installation

```bash
curl -fsSL https://get.sectui.dev | bash
```

## Platform Support

- **Linux**: Ubuntu, Debian, Fedora, RHEL, Rocky, Arch, Alpine, and derivatives
- **macOS**: Full support including pf firewall and Homebrew
- **Containers**: Detects Docker, LXC, Kubernetes environments
- **WSL**: Windows Subsystem for Linux detection
- **Init systems**: systemd, launchd, OpenRC
