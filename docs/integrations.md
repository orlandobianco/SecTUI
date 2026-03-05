# Integrations

SecTUI doesn't just manage security tools — it **uses** them. Scan results from third-party tools feed back into the dashboard, score, and notification system.

## Third-Party Tool Scan Integration

### How It Works

```
                SecTUI triggers scan
                        |
                        v
            ┌──────────────────────┐
            │  External Tool       │
            │  (ClamAV, rkhunter,  │
            │   AIDE, etc.)        │
            └──────────┬───────────┘
                       │
                  Raw output
                       │
                       v
            ┌──────────────────────┐
            │  SecTUI Parser       │
            │  Converts output to  │
            │  Finding objects     │
            └──────────┬───────────┘
                       │
              SecTUI Findings
                       │
           ┌───────────┼───────────┐
           v           v           v
      Dashboard    Score       Notifications
      (overview)   (0-100)    (if new findings)
```

### ClamAV Scan Integration

**Trigger**: Quick action in tool UI, scheduled scan, or `sectui scan full`

**Command**: `sudo clamscan -r --no-summary <paths>`

**Output parsing**:
```
/home/user/file.exe: Win.Trojan.Generic FOUND
/tmp/suspicious.sh: Unix.Malware.Agent FOUND
```

**Mapping to findings**:
- Each `FOUND` line -> Critical finding in Malware module
- Clean scan -> Info finding "ClamAV scan: no threats detected"

**Result in dashboard**:
- Score penalty: -15 per infected file (Critical)
- Active Protection panel: "ClamAV: ✓ Last scan clean" or "ClamAV: 3 threats found"
- Notification: sent if any threats detected

### rkhunter Scan Integration

**Trigger**: Scheduled scan or manual

**Command**: `sudo rkhunter --check --skip-keypress --report-warnings-only`

**Output parsing**:
```
Warning: The command '/usr/bin/lwp-request' has been replaced by a script
Warning: Hidden directory found: /dev/.udev
```

**Mapping to findings**:
- Each `Warning:` line -> High finding in Malware module
- Clean scan -> Info finding "rkhunter: no rootkits detected"

### AIDE Integrity Check Integration

**Trigger**: Scheduled check or manual

**Commands**:
- Initialize: `sudo aideinit` (first time)
- Check: `sudo aide --check`

**Output parsing**:
```
Changed files:
  /etc/passwd
  /etc/shadow
Added files:
  /tmp/suspicious_binary
Removed files:
  /usr/bin/legitimate_tool
```

**Mapping to findings**:
- Changed critical files (/etc/passwd, /etc/shadow) -> Critical
- New unexpected binaries -> High
- Changed config files -> Medium
- Removed files -> High

### Future: Lynis Integration

**Command**: `sudo lynis audit system --no-colors --quiet`

**Mapping**: Parse Lynis suggestions and warnings, map to SecTUI findings by module.

---

## Notifications

### Architecture

```
Notifier:
    client      HTTPClient      # HTTP client for API calls
    channels    []NotifChannel  # Configured channels

NotifChannel:
    Telegram { token, chat_id }
    Discord  { webhook_url }
```

No dedicated Telegram/Discord libraries needed. Just HTTP POST with JSON.

### Telegram Integration

#### Setup Flow (guided in wizard)

1. User opens Telegram, searches `@BotFather`, sends `/newbot`
2. Names the bot (e.g., "SecTUI Alerts")
3. Chooses username ending in `bot` (e.g., `my_sectui_bot`)
4. BotFather returns API token: `123456789:ABCdefGhIjKlMnOpQrStUvWxYz`
5. **Auto-detect chat ID**: User sends message to bot, SecTUI polls `getUpdates` API
6. Test message sent and confirmed

#### Sending Messages

```
POST https://api.telegram.org/bot<TOKEN>/sendMessage
{
    "chat_id": "<CHAT_ID>",
    "text": "<message>",
    "parse_mode": "Markdown"
}
```

#### Message Format
```
🚨 *SecTUI Alert*

*Type:* Brute Force Detected
*Server:* my-vps (192.168.1.100)
*Details:* 47 failed SSH login attempts from 45.33.xx.xx
*Time:* 2026-03-04 14:23:00 UTC

*Action taken:* IP banned via fail2ban for 1h
```

### Discord Integration

#### Setup Flow
1. Open Discord > Server Settings > Integrations > Webhooks
2. Click "New Webhook", name it (e.g., "SecTUI Alerts"), choose channel
3. Copy webhook URL
4. Paste in SecTUI wizard
5. Test embed sent

#### Sending Messages (Rich Embeds)

```
POST <webhook_url>
{
    "username": "SecTUI",
    "embeds": [{
        "title": "Brute Force Detected",
        "description": "47 failed SSH login attempts from 45.33.xx.xx",
        "color": 16711680,     // Red for Critical
        "fields": [
            { "name": "Server", "value": "my-vps", "inline": true },
            { "name": "Severity", "value": "Critical", "inline": true },
            { "name": "Module", "value": "SSH", "inline": true }
        ],
        "footer": { "text": "SecTUI Security Monitor" },
        "timestamp": "2026-03-04T14:23:00Z"
    }]
}
```

#### Severity Colors
| Severity | Color | Hex |
|----------|-------|-----|
| Critical | Red | 0xFF0000 |
| High | Orange | 0xFF8C00 |
| Medium | Yellow | 0xFFD700 |
| Low | Blue | 0x00BFFF |
| Info | Gray | 0x808080 |

### Alert Types

| Event | Severity | Default | Trigger |
|-------|----------|---------|---------|
| Brute force (>5/5min same IP) | Critical | On | Watch mode: auth log |
| Root login attempt | Critical | On | Watch mode: auth log |
| SSL cert <7 days | Critical | On | Watch mode: cert check |
| Service crashed | High | On | Watch mode: systemctl |
| New listening port | High | On | Watch mode: port baseline diff |
| Disk >90% | High | On | Watch mode: df |
| Unauthorized sudo attempt | Medium | On | Watch mode: auth log |
| Malware found (ClamAV) | Critical | On | Scheduled scan |
| Rootkit warning (rkhunter) | Critical | On | Scheduled scan |
| File integrity change (AIDE) | High | On | Scheduled check |
| Security updates available | Low | Off | Scheduled check |
| Scan completed | Info | Off | After any scan |

### Alert Deduplication

Same alert type + same source = suppress for configurable cooldown (default: 1 hour).

```toml
# config.toml
[notifications]
cooldown_minutes = 60        # Default cooldown
[notifications.cooldowns]
brute_force = 30             # Override per type
scan_complete = 1440         # Once per day
```

Alert history stored locally for `sectui alert history`.

---

## Scheduled Scans

### Configuration

```toml
[scan]
schedule = "0 3 * * *"          # Full SecTUI scan daily at 3 AM
schedule_enabled = false

[tools.clamav]
scan_schedule = "0 2 * * 0"     # Weekly Sunday 2 AM
scan_paths = ["/home", "/tmp", "/var/www"]

[tools.rkhunter]
scan_schedule = "0 3 * * 0"     # Weekly Sunday 3 AM

[tools.aide]
check_schedule = "0 4 * * *"    # Daily at 4 AM
```

### Implementation Options

1. **Built-in scheduler**: SecTUI's `watch` mode includes a cron-like scheduler
2. **System cron**: SecTUI generates crontab entries
3. **systemd timers**: SecTUI generates .timer units

Recommended: Built-in scheduler via `sectui watch --daemon` so all results are integrated.

### Scan Result Storage

Results stored at `~/.local/share/sectui/scans/`:
```
scans/
├── 2026-03-05T03:00:00.json    # Full scan result
├── 2026-03-02T02:00:00.json    # ClamAV scan
├── 2026-03-02T03:00:00.json    # rkhunter check
└── latest.json                  # Symlink to most recent
```

Used for:
- `sectui report trend` (score history)
- `sectui report compare` (diff two scans)
- Dashboard "Last scan" widget
- Score delta calculation
