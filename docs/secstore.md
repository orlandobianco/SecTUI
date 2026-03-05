# SecStore

A modern app-store style interface for discovering and installing security tools that aren't yet on the system.

## Concept

- Only shows tools that are **not installed** and **applicable** to the current platform
- Once a tool is installed, it moves from SecStore to the sidebar TOOLS section
- Two-column card layout with category filtering
- One-key install with confirmation dialog

## Categories

| Category | Icon | Tools |
|----------|------|-------|
| Firewall | Shield | UFW, nftables, firewalld |
| Intrusion Prevention | Police light | fail2ban, CrowdSec |
| Malware | Bug | ClamAV, rkhunter, chkrootkit |
| VPN | Lock | WireGuard, Tailscale |
| File Integrity | Page | AIDE |
| Access Control | Lock+key | AppArmor, SELinux |

## UI Layout

```
┌─ SecStore ──────────────────────────────────────────────────┐
│  All | Firewall | IPS | Malware | VPN | FIM | Access        │
├─────────────────────────────┬───────────────────────────────┤
│ ┌─────────────────────────┐ │ ┌─────────────────────────┐   │
│ │  > CrowdSec             │ │ │  rkhunter               │   │
│ │  [Intrusion Prevention] │ │ │  [Malware]              │   │
│ │  Collaborative IPS with │ │ │  Rootkit detection.     │   │
│ │  crowd-sourced threat   │ │ │  Compares file hashes   │   │
│ │  intelligence.          │ │ │  against known good     │   │
│ │  Press [Enter] to inst. │ │ │  values.                │   │
│ └─────────────────────────┘ │ └─────────────────────────┘   │
│ ┌─────────────────────────┐ │ ┌─────────────────────────┐   │
│ │  AIDE                   │ │ │  Tailscale              │   │
│ │  [File Integrity]       │ │ │  [VPN]                  │   │
│ │  File integrity monit.  │ │ │  Mesh VPN on WireGuard. │   │
│ │  Creates baseline DB.   │ │ │  Zero-config, works     │   │
│ │                         │ │ │  everywhere.            │   │
│ └─────────────────────────┘ │ └─────────────────────────┘   │
├─────────────────────────────┴───────────────────────────────┤
│ [j/k] Browse  [Tab] Category  [Enter/i] Install  [h] Back  │
└─────────────────────────────────────────────────────────────┘
```

## Card Design

Each card shows:
1. **Tool name** (bold, accent color when selected)
2. **Category badge** `[Intrusion Prevention]` (dimmed)
3. **Description** (1-2 lines, muted color)
4. **Install hint** (only on selected card): "Press [Enter] to install" or "No automatic install available"

Selected card: highlighted border (accent color), shows install action.

## Navigation

| Key | Action |
|-----|--------|
| `j` / `Down` | Move to next card |
| `k` / `Up` | Move to previous card |
| `Tab` | Cycle category filter (All > Firewall > IPS > Malware > VPN > FIM > Access > All) |
| `Enter` / `i` | Install selected tool (opens confirm dialog) |
| `h` / `Esc` | Back to sidebar |
| `?` | Help |
| `q` | Quit |

## Card Grid Logic

- Cards arranged in two columns
- Each card is 6 lines tall (fixed height)
- `cards_per_column = ceil(total_cards / 2)`
- Left column: cards 0 to cards_per_column-1
- Right column: remaining cards
- Overflow: scroll within column

## Install Flow

1. User presses Enter on a card
2. **Confirm dialog** appears:
   ```
   ┌─ Install CrowdSec? ──────────────────┐
   │                                        │
   │  Command: curl -s https://install.     │
   │  crowdsec.net | bash                   │
   │                                        │
   │  This will install CrowdSec on your   │
   │  system.                               │
   │                                        │
   │     [y] Install  [n] Cancel            │
   └────────────────────────────────────────┘
   ```
3. If confirmed: execute install command (with sudo if needed)
4. Show result (success or error with details)
5. On success: refresh tool list, tool moves to sidebar TOOLS section
6. On error: show error dialog with the failure reason and how to fix it

## Empty State

When all applicable tools are installed:

```
│                                                             │
│  All available tools are already installed!                  │
│                                                             │
│  Check the TOOLS section in the sidebar to manage them.     │
│                                                             │
```

## Data Flow

```
App load -> detect_all() -> split by status:
    ToolStatus::Active/Installed  -> Sidebar TOOLS section
    ToolStatus::NotInstalled      -> SecStore cards
    ToolStatus::NotApplicable     -> Hidden (not shown anywhere)

After install:
    Re-run detect_all() -> tool moves from SecStore to Sidebar
```

## Tool Descriptions for SecStore

| Tool | Description |
|------|-------------|
| UFW | Simple firewall frontend. Blocks unwanted incoming connections with easy-to-understand rules. |
| fail2ban | Intrusion prevention framework. Monitors log files and bans IPs that show malicious signs like repeated failed login attempts. |
| CrowdSec | Collaborative IPS with crowd-sourced threat intelligence. Blocks IPs known to attack other servers in the network. |
| ClamAV | Open-source antivirus engine. Scans files for malware, trojans, viruses, and other threats. |
| rkhunter | Rootkit detection. Compares file hashes against known good values to detect unauthorized changes. |
| WireGuard | Modern VPN tunnel. Fast, secure, and simple. Uses state-of-the-art cryptography with minimal attack surface. |
| Tailscale | Mesh VPN built on WireGuard. Zero-config, works through CGNAT and firewalls. |
| AIDE | File integrity monitoring. Creates baseline database of file hashes, detects unauthorized changes to system files. |
| AppArmor | Mandatory access control system. Confines programs with per-program security profiles to limit resource access. |
| SELinux | Label-based mandatory access control. More powerful than AppArmor but more complex to manage. |
