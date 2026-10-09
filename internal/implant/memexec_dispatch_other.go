//go:build !windows && !linux

package implant

import "fmt"

func executeShellcodeCrossPlatform(shellcode []byte) error {
	return fmt.Errorf("shellcode execution not supported on this platform")
}

func injectShellcodeRemoteCrossPlatform(pid uint32, shellcode []byte) error {
	return fmt.Errorf("process injection not supported on this platform")
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
