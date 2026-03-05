# UI Design

## Framework

- **Bubble Tea** (Elm Architecture) — Model, Update, View pattern
- Inspired by: btop (dashboard density), lazygit (panel navigation), k9s (command mode)

## Color Palette (Semantic)

| Meaning | Color | Terminal |
|---------|-------|---------|
| Healthy / OK | Green | ANSI Green |
| Warning | Yellow | ANSI Yellow |
| Critical / Alert | Red | ANSI Red |
| Info / Neutral | Cyan | ANSI Cyan |
| Dimmed / Secondary | Dark Gray | ANSI Bright Black |
| Interactive / Selected | Bold White | Bold |
| Accent / Branding | Magenta | ANSI Magenta |
| Dangerous action | Orange | ANSI 208 |
| Key hint | Cyan Bold | Cyan + Bold |

Rules:
- Use base 16 terminal colors (respect user's theme)
- Support `NO_COLOR` env var and `--no-color` flag
- Test on both light and dark terminal backgrounds

---

## Global Layout: Sidebar + Content

All TUI views (except wizard) use a fixed sidebar + dynamic content area:

```
┌──────────────────────────────────────────────────────────────┐
│ SecTUI                              Score: 85/100    [?]help │ <- Header (1 line)
├──────────────┬───────────────────────────────────────────────┤
│              │                                               │
│  OVERVIEW    │     (Content changes based on                 │
│              │      sidebar selection)                       │
│  MODULES     │                                               │
│  > SSH       │     Could be:                                 │
│    Firewall  │     - Overview dashboard                      │
│    Network   │     - Module findings + fixes                 │
│    Users     │     - Tool management UI                      │
│    Updates   │     - SecStore                                │
│    Kernel    │                                               │
│              │                                               │
│  TOOLS       │                                               │
│  > UFW       │                                               │
│    fail2ban  │                                               │
│    ClamAV    │                                               │
│              │                                               │
│  SECSTORE    │                                               │
│              │                                               │
├──────────────┴───────────────────────────────────────────────┤
│ [Tab] Focus  [s] Scan  [?] Help  [q] Quit                   │ <- Footer (1 line)
└──────────────────────────────────────────────────────────────┘
```

- Sidebar width: 22 characters fixed
- Content area: fills remaining space
- Header: 1 line with app name + score + help hint
- Footer: 1 line with context-sensitive keybindings

### Navigation

| Key | Where | Action |
|-----|-------|--------|
| `Tab` | Anywhere | Toggle focus between Sidebar and Content |
| `j` / `Down` | Sidebar | Move selection down (loads content instantly) |
| `k` / `Up` | Sidebar | Move selection up (loads content instantly) |
| `Enter` / `l` / `Right` | Sidebar | Focus content area |
| `h` / `Left` / `Esc` | Content | Return focus to sidebar |
| `s` | Anywhere | Start security scan |
| `?` | Anywhere | Toggle help overlay |
| `q` | Anywhere | Quit |

**Instant navigation**: Content loads on j/k movement — no need to press Enter to see content. Enter just switches focus to the content for interaction.

### Sidebar Sections

```
  OVERVIEW            <- Always first, shows dashboard
  ────────
  MODULES             <- Section header (dimmed, uppercase)
  > SSH               <- Active selection: accent + bold + arrow
    Firewall          <- Normal item
    Network
    Users & Perms
    Updates
    Kernel
  ────────
  TOOLS               <- Only shows Installed/Active tools
  > UFW         [ON]  <- Status badge: green ON / yellow OFF
    fail2ban    [ON]
    ClamAV      [OFF]
  ────────
  SecStore            <- Always at bottom
```

- Installed/Active tools appear in TOOLS section
- NotInstalled tools appear only in SecStore
- Tool status badges: `[ON]` green = Active, `[OFF]` yellow = Installed but not active

---

## Overview (Dashboard) — Animated & Interactive

The default view. Shows real-time system security status with live-updating widgets.

```
┌─────────────────────────────────────────────────────────────┐
│                                                             │
│  Score  [██████████████████░░░░░░░░] 78/100                 │
│         ▲ +5 since last scan                                │
│                                                             │
├─────────────────────────────┬───────────────────────────────┤
│  Findings by Severity       │  Active Protection            │
│                             │                               │
│  CRIT ██████  3             │  Firewall    ✓ UFW (12 rules) │
│  HIGH ████████  5           │  IPS         ✓ fail2ban (3j)  │
│  MED  ████  2               │  MAC         ✓ AppArmor       │
│  LOW  ██  1                 │  Malware     ✗ not active     │
│  INFO █  1                  │  FIM         ✗ not installed  │
│                             │  VPN         ✓ WireGuard      │
├─────────────────────────────┼───────────────────────────────┤
│  Open Ports (7)             │  Failed Logins (24h)          │
│  ┌─────┬──────────┬───────┐│  Total: 143   Unique IPs: 28  │
│  │  22 │ sshd     │ 0.0.0 ││                               │
│  │  80 │ nginx    │ 0.0.0 ││  ▁▂▃▅▇█▅▃▂▁ (sparkline)      │
│  │ 443 │ nginx    │ 0.0.0 ││                               │
│  │3000 │ node     │ 127.0 ││  Top attacker: 45.33.xx.xx    │
│  │5432 │ postgres │ 127.0 ││  Last attempt: 2m ago         │
│  └─────┴──────────┴───────┘│                               │
├─────────────────────────────┼───────────────────────────────┤
│  System Info                │  Last Scan                    │
│  OS: Ubuntu 22.04 (x86_64) │  2 min ago - Score: 78/100    │
│  Uptime: 42d 3h 15m        │  12 findings (3 CRIT)         │
│  Docker: ✓ (5 containers)  │  Next scheduled: 3:00 AM      │
│  SSH: 2 active sessions    │  [s] Scan now                 │
└─────────────────────────────┴───────────────────────────────┘
```

### Animated Elements

1. **Score gauge**: Animates from 0 to current score on load (fill animation over ~1s)
2. **Sparkline**: Login attempts over last 24h, updates in real-time
3. **Status indicators**: Pulse/blink when state changes (e.g., new SSH session)
4. **Score delta**: `+5 since last scan` or `-3 since last scan` with color
5. **Refresh timer**: Dashboard auto-refreshes every N seconds (configurable)

### Active Protection Panel

Shows all managed tool categories with live status:
- Green check + tool name = active and protecting
- Red X = not installed or not active
- Clicking/selecting shows count of related findings

---

## Module Content View

When selecting a module (e.g., SSH) from the sidebar:

```
┌─ SSH Configuration ─────────────────────────────────────────┐
│  7 findings   3 CRIT  2 HIGH  1 MED  1 LOW     Score: 42   │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  Findings                                                   │
│  ───────                                                    │
│  [x] CRIT  Root login enabled via SSH                       │
│  [x] CRIT  Password authentication enabled                  │
│  [x] CRIT  Empty passwords allowed                          │
│  [ ] HIGH  Public key auth not explicitly enabled            │
│  [ ] MED   Too many authentication attempts (6)             │
│  > [ ] LOW   X11 forwarding enabled                         │ <- Selected
│  [ ] LOW   Login grace time too long (120s)                 │
│                                                             │
├─────────────────────────────────────────────────────────────┤
│  X11 Forwarding Enabled                          Severity:  │
│                                                  LOW        │
│  WHY: X11 forwarding allows graphical apps to be            │
│  forwarded over SSH. On a server, this is rarely needed     │
│  and increases attack surface.                              │
│                                                             │
│  Current: X11Forwarding yes                                 │
│  Expected: X11Forwarding no                                 │
│                                                             │
│  FIX: Set 'X11Forwarding no' in /etc/ssh/sshd_config       │
│  and restart sshd.                                          │
│                                                             │
├─────────────────────────────────────────────────────────────┤
│ [Space] Toggle fix  [a] Select all  [Enter] Apply  [h] Back│
└─────────────────────────────────────────────────────────────┘
```

### Layout
- Top: Module header with severity breakdown + module score
- Middle-top: Findings list (scrollable, j/k navigation)
- Middle-bottom: Detail panel for selected finding (WHY + current vs expected + FIX)
- Bottom: Context keybindings

### Interaction
- `j/k` navigate findings
- `Space` toggle checkbox (mark for fixing)
- `a` select all fixable findings
- `Enter` apply selected fixes (opens confirm dialog first)
- `h/Esc` back to sidebar

---

## Tool Content View (Managed Tool)

When selecting an installed tool (e.g., fail2ban) that has a ToolManager:

```
┌─────────────────────────────────────────────────────────────┐
│  fail2ban  [Intrusion Prevention]  ✓ Running                │
├─────────────────────────────┬───────────────────────────────┤
│  Status                     │  Configuration                │
│  ──────                     │  ─────────────                │
│  Service   ✓ Running        │  bantime       10m            │
│  Version   1.0.2            │  findtime      10m            │
│  Number of jail: 3          │  maxretry      5              │
│  Jail list: sshd, nginx,..  │  ignoreip      127.0.0.1     │
│                             │  Config file   /etc/fail2ban/ │
│  Findings  4                │     jail.local                │
│                             │                               │
│  ✓ Last scan: Clean         │                               │
├─────────────────────────────┼───────────────────────────────┤
│  Quick Actions              │  Recent Activity              │
│  ─────────────              │  ───────────────              │
│  [1] SSH jail status        │  Mar 05 14:23 Ban 45.33.x.x  │
│  [2] List banned IPs        │  Mar 05 14:20 Found 45.33... │
│  [3] Restart service   (!)  │  Mar 05 13:15 Ban 92.118...  │
│  [4] Unban all IPs     (!)  │  Mar 05 13:12 Found 92.1...  │
│                             │  Mar 05 12:00 Jail started    │
│  [r] Refresh  [h] Back      │                               │
├─────────────────────────────┴───────────────────────────────┤
│  Result: SSH jail: 2 IPs banned, 47 failed attempts         │
└─────────────────────────────────────────────────────────────┘
```

### 4-Panel Layout

1. **Status** (top-left): Service running status, version, extra info (jails, profiles, etc.), related findings count, last scan result
2. **Configuration** (top-right): Key config values parsed from tool's config files
3. **Quick Actions** (bottom-left): Numbered shortcuts (1-9) for common operations, dangerous actions marked with (!) in orange/red
4. **Activity** (bottom-right): Recent log entries from journalctl/tool logs

### Tool-Specific Scan Results

Tools that can scan (ClamAV, rkhunter, AIDE) show their scan results as Findings that feed back into the dashboard:
- ClamAV: malware scan results -> Malware module findings
- rkhunter: rootkit check results -> Malware module findings
- AIDE: file integrity changes -> FileIntegrity module findings

Results appear in both the tool's management UI and the Overview dashboard.

---

## Tool Content View (Simple — No Manager)

For tools without a full ToolManager (basic detection only):

```
┌─ WireGuard ─────────────────────────────────────────────────┐
│                                                             │
│  WireGuard  [VPN]                                           │
│                                                             │
│  Modern VPN tunnel. Fast, secure, simple.                   │
│  Uses state-of-the-art cryptography.                        │
│                                                             │
│  Status: ✓ Active                                           │
│                                                             │
│  Running and protecting your system.                        │
│                                                             │
│  Related findings: 0                                        │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

For not-installed tools (if viewed from SecStore):

```
│  Status: ✗ Not Installed                                    │
│                                                             │
│  Install: apt install -y wireguard                          │
│                                                             │
│  Press [i] to install                                       │
```

---

## SecStore View

App-store style interface for discovering and installing security tools:

```
┌─────────────────────────────────────────────────────────────┐
│  SecStore                                                   │
│  All | Firewall | IPS | Malware | VPN | FIM | Access        │ <- Category tabs
├─────────────────────────────┬───────────────────────────────┤
│ ┌─────────────────────────┐ │ ┌─────────────────────────┐   │
│ │  CrowdSec               │ │ │  rkhunter               │   │
│ │  [Intrusion Prevention] │ │ │  [Malware]              │   │
│ │  Collaborative IPS with │ │ │  Rootkit detection.     │   │
│ │  crowd-sourced threat   │ │ │  Compares file hashes   │   │
│ │  intelligence.          │ │ │  against known good.    │   │
│ │  Press [Enter] to inst. │ │ │                         │   │
│ └─────────────────────────┘ │ └─────────────────────────┘   │
│ ┌─────────────────────────┐ │ ┌─────────────────────────┐   │
│ │  AIDE                   │ │ │  SELinux                │   │
│ │  [File Integrity]       │ │ │  [Access Control]       │   │
│ │  File integrity monit.  │ │ │  Label-based mandatory  │   │
│ │  Creates baseline DB.   │ │ │  access control.        │   │
│ │                         │ │ │                         │   │
│ └─────────────────────────┘ │ └─────────────────────────┘   │
│                             │                               │
│ ┌─────────────────────────┐ │                               │
│ │  Tailscale              │ │                               │
│ │  [VPN]                  │ │                               │
│ │  Mesh VPN on WireGuard. │ │                               │
│ │  Zero-config.           │ │                               │
│ └─────────────────────────┘ │                               │
├─────────────────────────────┴───────────────────────────────┤
│ [j/k] Browse  [Tab] Category  [Enter/i] Install  [h] Back  │
└─────────────────────────────────────────────────────────────┘
```

### Layout
- Category tab bar at top (Tab to cycle: All > Firewall > IPS > Malware > VPN > FIM > Access)
- Two-column card grid
- Each card: tool name, category badge, description
- Selected card: highlighted border, shows install hint

### Category Icons (Unicode)
- Firewall: shield
- Intrusion Prevention: police light
- Malware: bug
- VPN: lock
- File Integrity: page
- Access Control: lock with key

---

## Scanner View

Full-screen scan progress (replaces content area during scan):

```
┌─ Security Scan ─────────────────────────────────────────────┐
│                                                             │
│  Progress: [████████████████░░░░░░░░░░░] 62%                │
│  Current:  Scanning network ports...                        │
│                                                             │
├──────┬─────────────────────────────────┬────────┬───────────┤
│ Sev. │ Finding                         │ Module │ Fix?      │
├──────┼─────────────────────────────────┼────────┼───────────┤
│ CRIT │ Root login enabled via SSH      │ ssh    │ ✓         │
│ CRIT │ Password auth enabled           │ ssh    │ ✓         │
│ CRIT │ Empty passwords allowed         │ ssh    │ ✓         │
│ HIGH │ PubkeyAuth not explicitly set   │ ssh    │ ✓         │
│ HIGH │ Firewall inactive               │ fw     │ ✓         │
│ MED  │ Auto-updates not configured     │ update │ ✓         │
│ LOW  │ X11 forwarding enabled          │ ssh    │ ✓         │
│ ...  │ (findings appear as scan runs)  │        │           │
├──────┴─────────────────────────────────┴────────┴───────────┤
│  Score: 45/100    Findings: 3 CRIT, 2 HIGH, 1 MED, 1 LOW   │
│  Modules: ssh ✓  firewall ✓  network ▶  users ...  kernel  │
│                                                             │
│  [Esc] Cancel                                               │
└─────────────────────────────────────────────────────────────┘
```

### Behavior
- Findings appear live as each module completes
- Progress bar fills with each module
- Module status bar at bottom: ✓ done, ▶ in progress, ... pending
- Score updates live as findings come in
- After scan: automatically switches to Overview with updated data

---

## Setup Wizard

Full-screen, step-by-step first-run experience:

### Step 1: Welcome
```
┌──────────────────────────────────────────────────────┐
│                                                      │
│           Welcome to SecTUI!                         │
│                                                      │
│  SecTUI helps you secure your server with            │
│  easy-to-understand security tools.                  │
│                                                      │
│  Let's start by scanning your system.                │
│                                                      │
│  Language: [English v]                               │
│                                                      │
│                  [Start Setup ->]                     │
└──────────────────────────────────────────────────────┘
```

### Step 2: System Detection (automatic)
```
│  [1/5] System Detection                              │
│                                                      │
│  OS:         Ubuntu 22.04 LTS                        │
│  Arch:       x86_64                                  │
│  Init:       systemd                                 │
│  Package:    apt                                     │
│  Docker:     ✓ Installed (v24.0.7)                   │
│  Fresh:      ✗ Existing system (uptime: 42 days)     │
│                                                      │
│  Existing security tools detected:                   │
│  * UFW: installed but inactive                       │
│  * fail2ban: not installed                           │
│  * AppArmor: installed, enforcing                    │
│                                                      │
│                  [Continue ->]                        │
```

### Step 3: Quick Scan
```
│  [2/5] Initial Security Scan                         │
│                                                      │
│  Running quick scan...                               │
│  [████████████████░░░░] 80%                          │
│                                                      │
│  Current score: 38/100                               │
│  Found: 3 critical, 2 high, 4 medium                │
│                                                      │
│                  [See Results ->]                     │
```

### Step 4: Hardening Recommendations
```
│  [3/5] Recommended Actions                           │
│                                                      │
│  [x] Enable UFW firewall                             │
│      Blocks all incoming except SSH (22)             │
│                                                      │
│  Intrusion Prevention (choose one):                  │
│  (*) fail2ban  - Simple, proven, lightweight         │
│  ( ) CrowdSec  - Modern, crowd-sourced intelligence  │
│  WHY: fail2ban reacts to attacks on YOUR server.     │
│  CrowdSec also blocks IPs known to attack OTHER     │
│  servers. CrowdSec is newer but more powerful.       │
│                                                      │
│  [x] Harden SSH configuration                        │
│  [x] Enable automatic security updates               │
│  [x] Apply kernel hardening (sysctl)                 │
│                                                      │
│          [Apply Selected ->]  [Skip ->]              │
```

### Step 5: Notifications
```
│  [4/5] Notifications (Optional)                      │
│                                                      │
│  Get alerts when something needs attention:          │
│  * Brute force attacks detected                      │
│  * Services crash                                    │
│  * SSL certificates expiring                         │
│                                                      │
│  [t] Setup Telegram notifications                    │
│  [d] Setup Discord notifications                     │
│  [s] Skip for now                                    │
│                                                      │
│  You can always configure later: sectui alert config │
```

### Step 6: Done
```
│  [5/5] Setup Complete!                               │
│                                                      │
│  Score: 38 -> 82 (+44)                               │
│  ████████████████░░░░ 82/100                         │
│                                                      │
│  Installed: UFW, fail2ban, unattended-upgrades       │
│  Hardened:  SSH config, sysctl parameters            │
│                                                      │
│  Config saved: ~/.config/sectui/config.toml          │
│                                                      │
│  Commands:                                           │
│    sectui dashboard    - Open status dashboard       │
│    sectui scan full    - Run comprehensive scan      │
│                                                      │
│         [Open Dashboard ->]  [Exit]                  │
```

---

## Confirmation Dialog

Modal overlay for destructive/important actions:

```
           ┌─ Confirm ────────────────────┐
           │                              │
           │  Apply SSH hardening?        │
           │                              │
           │  This will:                  │
           │  - Disable root login        │
           │  - Disable password auth     │
           │  - Set MaxAuthTries to 3     │
           │                              │
           │  A backup will be created.   │
           │                              │
           │     [y] Confirm  [n] Cancel  │
           └──────────────────────────────┘
```

### Error Dialog

```
           ┌─ Error ──────────────────────┐
           │                              │
           │  Permission denied           │
           │                              │
           │  Cannot read /etc/ssh/sshd.. │
           │  Run sectui with sudo or     │
           │  check file permissions.     │
           │                              │
           │     [any key] Dismiss        │
           └──────────────────────────────┘
```

---

## Help Overlay

Full-screen keybinding reference (toggle with `?`):

```
┌─ Help ──────────────────────────────────────────────────────┐
│                                                             │
│  Navigation                                                 │
│  Tab .......... Toggle sidebar/content focus                │
│  j/k .......... Move up/down                                │
│  Enter/l ...... Select / focus content                      │
│  h/Esc ........ Back to sidebar                             │
│                                                             │
│  Global                                                     │
│  s ............ Start security scan                         │
│  ? ............ Toggle this help                            │
│  q ............ Quit                                        │
│                                                             │
│  Module Content                                             │
│  Space ........ Toggle fix checkbox                         │
│  a ............ Select all fixes                            │
│  Enter ........ Apply selected fixes                        │
│                                                             │
│  Tool Management                                            │
│  1-9 .......... Quick actions                               │
│  r ............ Refresh tool data                           │
│  i ............ Install (if not installed)                  │
│                                                             │
│  SecStore                                                   │
│  Tab .......... Cycle categories                            │
│  Enter/i ...... Install selected tool                       │
│                                                             │
│                    [?] Close                                 │
└─────────────────────────────────────────────────────────────┘
```

---

## UX Patterns

### Progress Indicators
- Quick operations (<2s): braille spinner `...`
- Longer operations: `[████████░░░░░░░░] 50% - Scanning ports...`
- Background tasks: footer text `Scanning... 1024/65535 ports`

### Search/Filter (future)
- `/` to enter filter mode (like vim/k9s)
- Filter findings by severity, module, or text
- `:` for command mode (`:ports`, `:ssh`, `:firewall`)

### Responsive Layout
- Detect terminal size on startup and resize events
- Compact mode for small terminals (<80 cols): stack panels vertically
- Full mode for large terminals (>120 cols): all panels visible

### Confirmation for Destructive Actions
- Always show what will change BEFORE applying
- Offer dry-run preview
- Dangerous quick actions (restart, unban) require confirmation
- Non-dangerous actions (status, list) execute immediately
