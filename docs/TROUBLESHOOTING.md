# Phantom C2 — Troubleshooting Guide

## Quick Diagnostics

Run the built-in diagnostic tool:

```bash
# Before starting the server
./build/phantom-server --doctor

# Or from the CLI while running
phantom > doctor
```

This checks: config files, RSA keys, TLS certs, database, ports, network, build tools, and directories.

---

## Common Issues

### 1. "load private key: no such file"

**Cause:** RSA keys haven't been generated yet.

**Fix:**
```bash
go run ./cmd/keygen -out configs/
# or
make keygen
```

### 2. "bind: permission denied" on port 443 or 53

**Cause:** Ports below 1024 require root/admin privileges.

**Fix (Option A):** Run with sudo
```bash
sudo ./build/phantom-server --config configs/server.yaml
```

**Fix (Option B):** Use non-privileged ports — edit `configs/server.yaml`:
```yaml
listeners:
  - name: "https-listener"
    type: "https"
    bind: "0.0.0.0:8443"    # Instead of 443
```

### 3. "address already in use" on port 8080

**Cause:** Another process is using the port.

**Fix:** Find and kill it:
```bash
# Linux/macOS
ss -tlnp | grep 8080
kill -9 <PID>

# Windows
netstat -aon | findstr 8080
taskkill /PID <PID> /F
```

Or change the port in `configs/server.yaml`.

### 4. Agent not connecting / no callback

**Check these in order:**

1. **Is the listener running?**
   ```
   phantom > listeners
   ```
   If stopped: `phantom > listeners start <name>`

2. **Was the agent built with the correct URL?**
   ```bash
   # The LISTENER_URL must match your server's IP and port
   make agent-linux LISTENER_URL=http://YOUR-IP:8080
   ```

3. **Firewall blocking the port?**
   ```bash
   # Linux
   sudo ufw allow 8080
   # or
   sudo iptables -A INPUT -p tcp --dport 8080 -j ACCEPT
   ```

4. **Agent and server on different networks?**
   - Use your external/VPN IP, not 127.0.0.1
   - Check: `ip addr show` or `ifconfig`

### 5. Mobile app not calling back

1. **Check the endpoint:** Mobile apps use `/api/v1/mobile/checkin` (not `/api/v1/status`)
2. **Check listener is running:** The HTTP listener must be accessible from the phone's network
3. **HTTPS required?** If the phone blocks HTTP, use HTTPS listener with valid/self-signed cert
4. **Evasion blocking:** If the phone has security apps, the evasion module may delay callback by 5+ minutes
5. **Emulator?** The evasion module blocks callbacks from emulators entirely

### 6. "go: command not found"

**Linux:**
```bash
sudo apt install golang-go
# or download from https://go.dev/dl/
export PATH=$PATH:/usr/local/go/bin
```

**Windows:**
```powershell
winget install GoLang.Go
# Restart your terminal
```

### 7. Web UI not loading

1. **Check if it's started:** `phantom > webui` or use `--mode web`
2. **Check the port:** Default is http://localhost:3000
3. **WSL users:** Access via http://localhost:3000 in Windows browser (WSL ports forward automatically)
4. **Port conflict:** Try a different port: `phantom > webui 0.0.0.0:4000`

### 8. "authentication failed" on login

**Reset credentials:**
```bash
rm configs/.phantom_creds
./build/phantom-server --config configs/server.yaml
# It will prompt for new credentials
```

### 9. Agent builds failing

1. **Go compiler required:** `go version` must show 1.22+
2. **Cross-compilation:** Set environment variables:
   ```bash
   GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build ...
   ```
3. **Garble not found:** Install it: `go install mvdan.cc/garble@latest`

### 10. Database errors

**Reset the database:**
```bash
rm data/phantom.db
# The database is recreated automatically on next startup
```

---

## Getting Help

1. **Run diagnostics:** `./build/phantom-server --doctor`
2. **Check logs:** `logs/session_*.log`
3. **GitHub Issues:** https://github.com/Phantom-C2-77/Phantom/issues
4. **Version info:** `./build/phantom-server --version`

---

## Debug Mode

For verbose output, check the session log:
```bash
ls -la logs/
cat logs/session_*.log
```

The session log captures every command and output for debugging.
