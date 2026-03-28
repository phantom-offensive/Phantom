//go:build windows

package implant

func executeShellcodeCrossPlatform(shellcode []byte) error {
	return ExecuteShellcodeWindows(shellcode)
}

func injectShellcodeRemoteCrossPlatform(pid uint32, shellcode []byte) error {
	return InjectShellcodeRemote(pid, shellcode)
}
