# Security Tools

SecTUI manages 10+ external security tools. Each tool has detection logic, optional install, and (for key tools) a full management interface.

## Tools by Category

### Firewall

| Tool | Platform | Difficulty | Binary | Service |
|------|----------|------------|--------|---------|
| **UFW** | Linux (Debian/Ubuntu) | Beginner | `ufw` | `ufw` |
| **nftables** | Linux | Advanced | `nft` | `nftables` |
| **iptables** | Linux (legacy) | Advanced | `iptables` | - |
| **pf** | macOS | Advanced | `pfctl` | - |
| **firewalld** | Linux (RHEL/Fedora) | Intermediate | `firewall-cmd` | `firewalld` |

**SecTUI approach**: Detect distro -> suggest UFW (Debian/Ubuntu), firewalld (RHEL), pf (macOS). Offer nftables as advanced option.

**Currently implemented**: UFW detection + basic management.

---

### Intrusion Prevention

| Tool | Platform | Difficulty | Binary | Service |
|------|----------|------------|--------|---------|
| **fail2ban** | Linux | Beginner | `fail2ban-client` | `fail2ban` |
| **CrowdSec** | Linux | Intermediate | `cscli` | `crowdsec` |

**SecTUI approach**: Install fail2ban as default (beginner). Offer CrowdSec upgrade path (intermediate). Explain the difference:
- **fail2ban** = reactive (bans after attack on YOUR server)
- **CrowdSec** = proactive (blocks IPs known to attack OTHER servers via community blocklists)

#### fail2ban — Full Management UI

**Detection**: `which fail2ban-client` + `systemctl is-active fail2ban`
**Install**: `apt install -y fail2ban` / `dnf install -y fail2ban` / `pacman -S --noconfirm fail2ban`

**Management panels**:

| Panel | Content |
|-------|---------|
| Status | Service running, version, number of jails, jail list |
| Config | bantime, findtime, maxretry, ignoreip (parsed from jail.local or jail.conf) |
| Quick Actions | `[1]` SSH jail status, `[2]` List banned IPs, `[3]` Restart (dangerous), `[4]` Unban all (dangerous) |
| Activity | `journalctl -u fail2ban` recent entries |

**Commands used**:
- `fail2ban-client --version`
- `sudo fail2ban-client status` (jail count + list)
- `sudo fail2ban-client status sshd` (SSH jail detail)
- `sudo fail2ban-client banned` (all banned IPs)
- `sudo systemctl restart fail2ban`
- `sudo fail2ban-client unban --all`

**Scan integration**: fail2ban findings count as Intrusion Prevention module data. Number of bans and attacks feed into dashboard.

#### CrowdSec — Full Management UI

**Detection**: `which cscli` + `systemctl is-active crowdsec`
**Install**: `curl -s https://install.crowdsec.net | bash`

**Management panels**:

| Panel | Content |
|-------|---------|
| Status | Service running, version, metrics summary |
| Config | Collections count, bouncers registered, parsers installed |
| Quick Actions | `[1]` Active decisions, `[2]` Recent alerts, `[3]` Update hub, `[4]` Restart (dangerous) |
| Activity | `journalctl -u crowdsec` recent entries |

**Commands used**:
- `cscli version`
- `sudo cscli metrics show --no-color`
- `sudo cscli decisions list --no-color`
- `sudo cscli alerts list --no-color -l 10`
- `sudo cscli hub update`
- `sudo cscli collections list --no-color -o raw`
- `sudo cscli bouncers list --no-color -o raw`
- `sudo cscli parsers list --no-color -o raw`
- `sudo systemctl restart crowdsec`

---

### Malware Detection

| Tool | Platform | Difficulty | Binary | Service |
|------|----------|------------|--------|---------|
| **ClamAV** | Linux + macOS | Beginner | `clamscan`, `freshclam` | `clamav-daemon` |
| **rkhunter** | Linux + macOS | Beginner | `rkhunter` | - |
| **chkrootkit** | Linux + macOS | Beginner | `chkrootkit` | - |

#### ClamAV — Full Management UI

**Detection**: `which clamscan` + `systemctl is-active clamav-daemon`
**Install**: `apt install -y clamav clamav-daemon` / `dnf install -y clamav clamav-update clamd` / `pacman -S --noconfirm clamav`

**Management panels**:

| Panel | Content |
|-------|---------|
| Status | Daemon running, freshclam running, DB version + signature count, last update timestamp |
| Config | ScanPE, ScanELF, ScanOLE2, MaxFileSize, MaxScanSize (parsed from clamd.conf) |
| Quick Actions | `[1]` Scan /home, `[2]` Scan /tmp, `[3]` Update virus DB, `[4]` Start daemon (dangerous) |
| Activity | `journalctl -u clamav-daemon` recent entries |

**Commands used**:
- `clamscan --version`
- `freshclam --version`
- `sigtool --info /var/lib/clamav/daily.cvd` (DB info)
- `sudo clamscan -r --no-summary /home` (home scan)
- `sudo clamscan -r --no-summary /tmp` (tmp scan)
- `sudo freshclam` (update DB)
- `sudo systemctl start clamav-daemon`

**Scan integration**: ClamAV scan results become Malware module findings. Infected files = Critical findings. Dashboard shows "Last ClamAV scan: Clean" or "3 infected files found".

#### rkhunter — Basic Detection + Scan Integration

**Detection**: `which rkhunter`
**Install**: `apt install -y rkhunter` / `dnf install -y rkhunter` / `pacman -S --noconfirm rkhunter`

**Scan integration**:
- `sudo rkhunter --check --skip-keypress --report-warnings-only`
- Warnings become Critical/High findings in Malware module
- Update: `sudo rkhunter --update`

---

### VPN / Tunneling

| Tool | Platform | Difficulty | Binary | Service |
|------|----------|------------|--------|---------|
| **WireGuard** | Linux (kernel) + macOS | Intermediate | `wg` | `wg-quick@*` |
| **Tailscale** | Linux + macOS | Beginner | `tailscale` | `tailscaled` |

**SecTUI approach**: Explain the difference:
- **WireGuard** = full control, manual config, kernel-level performance
- **Tailscale** = instant setup, mesh VPN built on WireGuard, works through CGNAT
- **Headscale** = Tailscale but fully self-hosted control server

**Detection**:
- WireGuard: `which wg` + `systemctl is-active wg-quick@*`
- Tailscale: `which tailscale` + `systemctl is-active tailscaled`

**Install**:
- WireGuard: `apt install -y wireguard` / `dnf install -y wireguard-tools`
- Tailscale: `curl -fsSL https://tailscale.com/install.sh | sh`

---

### File Integrity Monitoring

| Tool | Platform | Difficulty | Binary | Service |
|------|----------|------------|--------|---------|
| **AIDE** | Linux | Intermediate | `aide` | - |

**Detection**: `which aide`
**Install**: `apt install -y aide` / `dnf install -y aide`

**Scan integration**:
- Initialize baseline: `sudo aideinit` or `sudo aide --init`
- Check: `sudo aide --check`
- Changed files become High findings in FileIntegrity module
- New files = Medium, removed files = High, changed permissions = Medium

---

### Access Control (MAC)

| Tool | Platform | Difficulty | Binary | Service |
|------|----------|------------|--------|---------|
| **AppArmor** | Linux (Debian/Ubuntu) | Intermediate | `aa-status` / `apparmor_status` | `apparmor` |
| **SELinux** | Linux (RHEL/Fedora) | Advanced | `getenforce`, `sestatus` | - |

#### AppArmor — Full Management UI

**Detection**: `which apparmor_status` or `which aa-status` + `systemctl is-active apparmor`
**Install**: `apt install -y apparmor apparmor-utils`

**Management panels**:

| Panel | Content |
|-------|---------|
| Status | Service running, profiles loaded/enforce/complain/unconfined counts |
| Config | Profile file count in /etc/apparmor.d, config dir path |
| Quick Actions | `[1]` Full status, `[2]` List profiles, `[3]` Reload profiles, `[4]` Restart (dangerous) |
| Activity | `journalctl -k --grep=apparmor` recent kernel log entries |

**Commands used**:
- `sudo aa-status` / `sudo apparmor_status`
- `sudo aa-status --profiled` (profile list)
- `sudo systemctl reload apparmor`
- `sudo systemctl restart apparmor`

#### SELinux — Basic Detection

**Detection**: `which getenforce` -> run `getenforce` (Enforcing/Permissive/Disabled)
**Install**: Typically pre-installed on RHEL. `dnf install -y selinux-policy-targeted`

**Future management**: `sestatus`, `semanage`, `setsebool` commands.

---

## Tool Detection Logic

For each tool:

```
1. Check binary exists:     which <binary> (exit code 0 = exists)
2. Check service active:    systemctl is-active <service> (returns "active")
3. Map to status:
   - Binary missing              -> NotInstalled
   - Binary exists, service off  -> Installed
   - Binary exists, service on   -> Active
   - Not applicable on this OS   -> NotApplicable
```

Platform applicability:
- Linux-only tools: UFW, fail2ban, CrowdSec, AppArmor, SELinux, AIDE, firewalld, nftables
- macOS-only tools: pf
- Cross-platform: ClamAV, rkhunter, WireGuard, Tailscale

## Adding a New Tool

1. Create `internal/tools/newtool.go`
2. Implement `SecurityTool` interface (id, name, description, category, detect, install_command, is_applicable)
3. Register in `internal/tools/registry.go`
4. Optionally implement `ToolManager` for full management UI
5. Add i18n keys in `locales/en.yml`
6. Add to SecStore category mapping
